package job

import (
	"hash/crc32"
	"sync"

	"github.com/adnanh/webhook/internal/hook"
)

// HookEvent used to combine the hook and repository push event
type HookEvent struct {
	Hook    hook.Hook
	Request hook.Request
}

// Partition calculates a numbered partition this StatusUpdate belongs to based on a max of partitions
func (u *HookEvent) Partition(partitions uint32) uint32 {
	return crc32.ChecksumIEEE([]byte(u.Hook.ID)) % partitions
}

var (
	initOnce sync.Once
	jobQueue chan HookEvent
)

// GetJobQueue a buffered channel that we can send work requests on.
func GetJobQueue(queueSize int) chan HookEvent {
	initOnce.Do(func() {
		jobQueue = make(chan HookEvent, queueSize)
	})
	return jobQueue
}

// Push allows external push HookEvent to jobQueue
func Push(job HookEvent) {
	jobQueue <- job
}
