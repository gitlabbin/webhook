package job

import (
	"context"
	"hash/crc32"
	"log"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/adnanh/webhook/internal/hook"
)

// HookEvent used to combine the hook and repository push event
type HookEvent struct {
	Hook    hook.Hook
	Request hook.Request
}

var (
	initOnce sync.Once
	jobQueue chan HookEvent
)

// GetJobQueue a buffered channel that we can send work requests on.
func GetJobQueue() chan HookEvent {
	initOnce.Do(func() {
		jobQueue = make(chan HookEvent, 100)
	})
	return jobQueue
}

// Queueable interface of Queueable Job
type Queueable interface {
	Handle() error
}

// Dispatcher worker dispatcher
type Dispatcher struct {
	maxWorkers uint32
	WorkerPool chan LabelChannel
	Workers    []Worker
	Suh        HookEventHandler
}

// StartQueueDispatcher to initial loading the queue dispatcher
func StartQueueDispatcher(ctx context.Context, suh HookEventHandler) {
	logrus.Infoln("Queue Dispatcher starting......")
	queueDispatcher := NewDispatcher(4, suh)
	queueDispatcher.Run(ctx)

}

// NewDispatcher creates new queue dispatcher
func NewDispatcher(maxWorkers uint32, suh HookEventHandler) *Dispatcher {
	// make job
	_ = GetJobQueue()

	pool := make(chan LabelChannel, maxWorkers)
	return &Dispatcher{WorkerPool: pool, maxWorkers: maxWorkers, Suh: suh}
}

// Run starts work of dispatcher and creates the workers
func (d *Dispatcher) Run(ctx context.Context) {
	// starting n number of workers
	for i := uint32(0); i < d.maxWorkers; i++ {
		worker := NewWorker(d.WorkerPool, d.Suh)
		worker.Start(ctx)
		d.Workers = append(d.Workers, worker)
	}

	go d.dispatchPartition(ctx)
}

// Dispatch get job from queue and put into labelled job channel
func (d *Dispatcher) Dispatch() {
	for {
		job := <-jobQueue
		// a job request has been received
		log.Printf("[%s] %s NewJob ticket\n", job.Request.ID, job.Hook.ID)
		go func(job HookEvent) {
			// try to obtain a worker job channel that is available.
			// this will block until a worker is idle
			jobLabelChannel := <-d.WorkerPool

			// dispatch the job to the worker job channel
			jobLabelChannel.JobChannel <- job
		}(job)

	}
}

func (d *Dispatcher) dispatchPartition(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// we have received a signal to stop
			logrus.Infof("job reader going stop for terminate or killed.........")
			return
		case job := <-jobQueue:
			go func(job HookEvent, ctx context.Context) {
				part := job.Partition(d.maxWorkers)
				// a job request has been received
				log.Printf("[%s] %s NewJob ticket, partition: %d \n", job.Request.ID, job.Hook.ID, part)
				// try to obtain a worker job channel that is available.
				// this will block until a worker is idle
				for {
					select {
					case <-ctx.Done():
						// we have received a signal to stop
						logrus.Infof("worker dispatcher going stop for terminate or killed.........")
						return
					case jobLabelChannel := <-d.WorkerPool:
						if jobLabelChannel.Label == part {
							// dispatch the job to the worker job channel
							jobLabelChannel.JobChannel <- job
							return
						}

						// put labelChannel back to pool
						d.WorkerPool <- jobLabelChannel
					}

				}
			}(job, ctx)
		}
	}
}

// Partition calculates a numbered partition this StatusUpdate belongs to based on a max of partitions
func (u *HookEvent) Partition(partitions uint32) uint32 {
	return crc32.ChecksumIEEE([]byte(u.Hook.ID)) % partitions
}

// Push allows external push HookEvent to jobQueue
func Push(job HookEvent) {
	jobQueue <- job
}
