package daemon

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestThrottledBatch_AllSucceed(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7}
	succeeded, failed := throttledBatch(context.Background(), items, 3, time.Millisecond, func(i int) error {
		return nil
	})
	if succeeded != 7 {
		t.Errorf("succeeded = %d, want 7", succeeded)
	}
	if failed != 0 {
		t.Errorf("failed = %d, want 0", failed)
	}
}

func TestThrottledBatch_SomeFail(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	succeeded, failed := throttledBatch(context.Background(), items, 5, time.Millisecond, func(i int) error {
		if i%2 == 0 {
			return fmt.Errorf("fail")
		}
		return nil
	})
	if succeeded != 3 {
		t.Errorf("succeeded = %d, want 3", succeeded)
	}
	if failed != 2 {
		t.Errorf("failed = %d, want 2", failed)
	}
}

func TestThrottledBatch_Empty(t *testing.T) {
	succeeded, failed := throttledBatch(context.Background(), []int{}, 5, time.Millisecond, func(i int) error {
		t.Fatal("should not be called")
		return nil
	})
	if succeeded != 0 || failed != 0 {
		t.Errorf("succeeded=%d failed=%d, want 0 0", succeeded, failed)
	}
}

func TestThrottledBatch_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	callCount := 0
	succeeded, failed := throttledBatch(ctx, items, 3, 50*time.Millisecond, func(i int) error {
		callCount++
		// Cancel after 3rd item (end of first batch), during the delay
		if callCount == 3 {
			cancel()
		}
		return nil
	})
	// First batch of 3 should succeed, then context cancelled during delay
	// remaining 7 items counted as failed
	if succeeded != 3 {
		t.Errorf("succeeded = %d, want 3", succeeded)
	}
	if failed != 7 {
		t.Errorf("failed = %d, want 7", failed)
	}
}

func TestThrottledBatch_ContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	items := []int{1, 2, 3}
	succeeded, failed := throttledBatch(ctx, items, 2, time.Millisecond, func(i int) error {
		t.Fatal("should not be called")
		return nil
	})
	if succeeded != 0 {
		t.Errorf("succeeded = %d, want 0", succeeded)
	}
	if failed != 3 {
		t.Errorf("failed = %d, want 3", failed)
	}
}

func TestThrottledBatch_BatchSizeOne(t *testing.T) {
	items := []int{1, 2, 3}
	succeeded, failed := throttledBatch(context.Background(), items, 1, time.Millisecond, func(i int) error {
		return nil
	})
	if succeeded != 3 {
		t.Errorf("succeeded = %d, want 3", succeeded)
	}
	if failed != 0 {
		t.Errorf("failed = %d, want 0", failed)
	}
}

func TestThrottledBatch_DelayBetweenBatches(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6}
	delay := 50 * time.Millisecond
	start := time.Now()
	throttledBatch(context.Background(), items, 3, delay, func(i int) error {
		return nil
	})
	elapsed := time.Since(start)
	// With 6 items and batch size 3: 2 batches, 1 delay between them
	if elapsed < delay {
		t.Errorf("elapsed %v, expected at least %v (one delay between batches)", elapsed, delay)
	}
}

func TestThrottledBatch_ExactBatchSize(t *testing.T) {
	// When items count equals batch size, no delay should be inserted
	items := []int{1, 2, 3, 4, 5}
	start := time.Now()
	succeeded, failed := throttledBatch(context.Background(), items, 5, 100*time.Millisecond, func(i int) error {
		return nil
	})
	elapsed := time.Since(start)
	if succeeded != 5 || failed != 0 {
		t.Errorf("succeeded=%d failed=%d, want 5 0", succeeded, failed)
	}
	// Should complete quickly since no inter-batch delay is needed
	if elapsed > 50*time.Millisecond {
		t.Errorf("elapsed %v, expected under 50ms (no inter-batch delay needed)", elapsed)
	}
}
