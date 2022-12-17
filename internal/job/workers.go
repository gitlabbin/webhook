package job

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/sirupsen/logrus"
)

const workerPrefix string = "Worker"

type single struct {
	mu     sync.RWMutex
	values map[string]uint32
}

func (s *single) Get(key string) uint32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values[key]
}

func (s *single) Incr(key string) uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key]++
	return s.values[key]
}

// counter increases every time we create a worker
var counters = single{
	values: make(map[string]uint32),
}

// Worker simple worker that handles queueable tasks
type Worker struct {
	Name            string
	WorkerPool      chan LabelChannel
	JobLabelChannel LabelChannel
	quit            chan bool
	Processor       EventProcessor
}

// LabelChannel make channel support partition
type LabelChannel struct {
	Label      uint32
	JobChannel chan HookEvent
}

// NewWorker creates a new worker
func NewWorker(workerPool chan LabelChannel, processor EventProcessor) Worker {
	defer counters.Incr(workerPrefix)
	return Worker{
		Name:       fmt.Sprintf("%s-%d", workerPrefix, counters.Get(workerPrefix)),
		WorkerPool: workerPool,
		JobLabelChannel: LabelChannel{counters.Get(workerPrefix),
			make(chan HookEvent),
		},
		quit:      make(chan bool),
		Processor: processor,
	}
}

// Start initiate worker to start lisntening for upcomings queueable jobs
func (w Worker) Start(ctx context.Context) {
	go func() {
		for {
			// register the current worker into the worker queue.
			w.WorkerPool <- w.JobLabelChannel

			select {
			case job := <-w.JobLabelChannel.JobChannel:
				// we have received a work request.
				// track the total number of jobs processed by the worker
				log.Printf("[%s] %s doing job: \n", job.Request.ID, w.Name)
				w.Processor.apply(job)

			case <-ctx.Done():
				// we have received a signal to stop
				logrus.Infof("%v going stop for terminate or killed.........", w.Name)
				return
			case <-w.quit:
				// we have received a signal to stop
				logrus.Infof("%v going quit.........", w.Name)
				return
			}
		}
	}()
}

// Stop signals the worker to stop listening for work requests.
func (w Worker) Stop() {
	go func() {
		w.quit <- true
	}()
}
