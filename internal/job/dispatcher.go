package job

import (
	"context"
	"log"
	"sync"

	"github.com/sirupsen/logrus"
)

// Queueable interface of Queueable Job
type Queueable interface {
	Handle() error
}

// Dispatcher worker dispatcher
type Dispatcher struct {
	maxWorkers uint32
	WorkerPool chan LabelChannel
	Workers    []Worker
	Processor  EventProcessor
}

// StartQueueDispatcher to initial loading the queue dispatcher
func StartQueueDispatcher(ctx context.Context, wg *sync.WaitGroup, eventProcessor EventProcessor, queueSize int, maxWorkers uint32) {
	logrus.Infoln("Queue Dispatcher starting......")
	queueDispatcher := NewDispatcher(queueSize, maxWorkers, eventProcessor)
	queueDispatcher.Run(ctx, wg)

}

// NewDispatcher creates new queue dispatcher
func NewDispatcher(queueSize int, maxWorkers uint32, eventProcessor EventProcessor) *Dispatcher {
	// make job
	_ = GetJobQueue(queueSize)

	pool := make(chan LabelChannel, maxWorkers)
	return &Dispatcher{WorkerPool: pool, maxWorkers: maxWorkers, Processor: eventProcessor}
}

// Run starts work of dispatcher and creates the workers
func (d *Dispatcher) Run(ctx context.Context, wg *sync.WaitGroup) {
	// starting n number of workers
	for i := uint32(0); i < d.maxWorkers; i++ {
		worker := NewWorker(d.WorkerPool, d.Processor)
		worker.Start(ctx, wg)
		d.Workers = append(d.Workers, worker)
	}

	go d.dispatchPartition(ctx, wg)
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

func (d *Dispatcher) dispatchPartition(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done() // decrements the WaitGroup counter by one when the function returns

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
