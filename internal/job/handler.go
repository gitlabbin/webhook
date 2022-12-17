package job

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/adnanh/webhook/internal/hook"
)

// EventProcessor describes an interface to send hook event somewhere.
type EventProcessor interface {
	apply(event HookEvent)
}

// HookEventHandler include 2 channels for handler
type HookEventHandler struct {
	sendUpdates   chan struct{}
	updateChannel chan HookEvent
}

// NewHookEventHandler initial handler with job queue and channel
func NewHookEventHandler(channelSize int) EventProcessor {
	return &HookEventHandler{
		sendUpdates:   make(chan struct{}),
		updateChannel: GetJobQueue(channelSize), //reference of dispatcher jobQueue
	}
}

func (hookEvtHandler *HookEventHandler) apply(event HookEvent) {
	_, err := HandleHook(&event.Hook, &event.Request)
	if err != nil {
		return
	}
}

// HandleHook process the hook with coming request
func HandleHook(h *hook.Hook, r *hook.Request) (string, error) {
	var errors []error

	// check the command exists
	var lookpath string
	if filepath.IsAbs(h.ExecuteCommand) || h.CommandWorkingDirectory == "" {
		lookpath = h.ExecuteCommand
	} else {
		lookpath = filepath.Join(h.CommandWorkingDirectory, h.ExecuteCommand)
	}

	cmdPath, err := exec.LookPath(lookpath)
	if err != nil {
		log.Printf("[%s] error in %s", r.ID, err)

		// check if parameters specified in execute-command by mistake
		if strings.IndexByte(h.ExecuteCommand, ' ') != -1 {
			s := strings.Fields(h.ExecuteCommand)[0]
			log.Printf("[%s] use 'pass-arguments-to-command' to specify args for '%s'", r.ID, s)
		}

		return "", err
	}

	cmd := exec.Command(cmdPath)
	cmd.Dir = h.CommandWorkingDirectory

	cmd.Args, errors = h.ExtractCommandArguments(r)
	for _, err := range errors {
		log.Printf("[%s] error extracting command arguments: %s\n", r.ID, err)
	}

	var envs []string
	envs, errors = h.ExtractCommandArgumentsForEnv(r)

	for _, err := range errors {
		log.Printf("[%s] error extracting command arguments for environment: %s\n", r.ID, err)
	}

	files, errors := h.ExtractCommandArgumentsForFile(r)

	for _, err := range errors {
		log.Printf("[%s] error extracting command arguments for file: %s\n", r.ID, err)
	}

	for i := range files {
		tmpfile, err := ioutil.TempFile(h.CommandWorkingDirectory, files[i].EnvName)
		if err != nil {
			log.Printf("[%s] error creating temp file [%s]", r.ID, err)
			continue
		}
		log.Printf("[%s] writing env %s file %s", r.ID, files[i].EnvName, tmpfile.Name())
		if _, err := tmpfile.Write(files[i].Data); err != nil {
			log.Printf("[%s] error writing file %s [%s]", r.ID, tmpfile.Name(), err)
			continue
		}
		if err := tmpfile.Close(); err != nil {
			log.Printf("[%s] error closing file %s [%s]", r.ID, tmpfile.Name(), err)
			continue
		}

		files[i].File = tmpfile
		envs = append(envs, files[i].EnvName+"="+tmpfile.Name())
	}

	cmd.Env = append(os.Environ(), envs...)

	log.Printf("[%s] executing %s (%s) with arguments %q and environment %s using %s as cwd\n", r.ID, h.ExecuteCommand, cmd.Path, cmd.Args, envs, cmd.Dir)

	out, err := cmd.CombinedOutput()

	log.Printf("[%s] command output: %s\n", r.ID, out)

	if err != nil {
		log.Printf("[%s] error occurred: %+v\n", r.ID, err)
	}

	for i := range files {
		if files[i].File != nil {
			log.Printf("[%s] removing file %s\n", r.ID, files[i].File.Name())
			err := os.Remove(files[i].File.Name())
			if err != nil {
				log.Printf("[%s] error removing file %s [%s]", r.ID, files[i].File.Name(), err)
			}
		}
	}

	log.Printf("[%s] finished handling %s\n", r.ID, h.ID)

	return string(out), err
}

// Writer retrieves the interface that should be used to write to the StatusUpdateHandler.
func (hookEvtHandler *HookEventHandler) Writer() EventUpdater {
	return &RequestEventWriter{
		enabled:       hookEvtHandler.sendUpdates,
		updateChannel: hookEvtHandler.updateChannel,
	}
}

// EventUpdater describes an interface to send hook event somewhere.
type EventUpdater interface {
	Send(su HookEvent)
}

// RequestEventWriter takes status updates and sends these to the StatusUpdateHandler via a channel.
type RequestEventWriter struct {
	enabled       <-chan struct{}
	updateChannel chan<- HookEvent
}

// Send sends the given StatusUpdate off to the update channel for writing by the StatusUpdateHandler.
func (suw *RequestEventWriter) Send(event HookEvent) {
	// Non-blocking receive to see if we should pass along update.
	select {
	case <-suw.enabled:
		suw.updateChannel <- event
		//Push(event)
	default:
	}
}
