package daemon

import (
	"context"
	"time"
)

const (
	restoreBatchSize  = 5
	restoreBatchDelay = 200 * time.Millisecond
)

// throttledBatch executes fn for each item in batches with delay between batches.
// It returns the count of succeeded and failed invocations. If the context is
// cancelled, remaining items are counted as failed.
func throttledBatch[T any](ctx context.Context, items []T, batchSize int, delay time.Duration, fn func(T) error) (succeeded int, failed int) {
	for i, item := range items {
		if ctx.Err() != nil {
			failed += len(items) - i
			return
		}
		if err := fn(item); err != nil {
			failed++
		} else {
			succeeded++
		}
		// Delay after each full batch (but not after the last item)
		if (i+1)%batchSize == 0 && i+1 < len(items) {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				failed += len(items) - i - 1
				return
			}
		}
	}
	return
}
