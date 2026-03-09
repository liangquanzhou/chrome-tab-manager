package daemon

import (
	"context"
	"sync"
)

// JobQueue serializes expensive operations to prevent resource contention.
// Heavy operations like session restore, workspace switch, and bookmark mirror
// are routed through this queue so they run one at a time.
type JobQueue struct {
	mu   sync.Mutex
	busy bool
	ch   chan func()
}

// NewJobQueue creates a job queue with a buffered channel.
func NewJobQueue() *JobQueue {
	return &JobQueue{
		ch: make(chan func(), 16),
	}
}

// Run processes jobs sequentially until the context is cancelled.
func (jq *JobQueue) Run(ctx context.Context) {
	for {
		select {
		case fn := <-jq.ch:
			jq.mu.Lock()
			jq.busy = true
			jq.mu.Unlock()

			fn()

			jq.mu.Lock()
			jq.busy = false
			jq.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

// Submit enqueues a function to be executed by the job queue.
func (jq *JobQueue) Submit(fn func()) {
	jq.ch <- fn
}

// IsBusy returns whether the queue is currently executing a job.
func (jq *JobQueue) IsBusy() bool {
	jq.mu.Lock()
	defer jq.mu.Unlock()
	return jq.busy
}
