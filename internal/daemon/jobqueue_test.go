package daemon

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestJobQueue_SequentialExecution(t *testing.T) {
	jq := NewJobQueue()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go jq.Run(ctx)

	var mu sync.Mutex
	var order []int

	done := make(chan struct{})
	for i := 0; i < 5; i++ {
		i := i
		jq.Submit(func() {
			mu.Lock()
			order = append(order, i)
			mu.Unlock()
			time.Sleep(5 * time.Millisecond) // simulate work
			if i == 4 {
				close(done)
			}
		})
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for jobs")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 5 {
		t.Fatalf("len(order) = %d, want 5", len(order))
	}
	for i, v := range order {
		if v != i {
			t.Errorf("order[%d] = %d, want %d", i, v, i)
		}
	}
}

func TestJobQueue_IsBusy(t *testing.T) {
	jq := NewJobQueue()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go jq.Run(ctx)

	if jq.IsBusy() {
		t.Error("expected not busy initially")
	}

	started := make(chan struct{})
	finish := make(chan struct{})
	jq.Submit(func() {
		close(started)
		<-finish
	})

	<-started
	if !jq.IsBusy() {
		t.Error("expected busy during job execution")
	}

	close(finish)
	// Wait for busy flag to clear
	time.Sleep(20 * time.Millisecond)
	if jq.IsBusy() {
		t.Error("expected not busy after job completion")
	}
}

func TestJobQueue_ContextCancellation(t *testing.T) {
	jq := NewJobQueue()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		jq.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

func TestJobQueue_SubmitAndProcess(t *testing.T) {
	jq := NewJobQueue()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go jq.Run(ctx)

	result := make(chan int, 1)
	jq.Submit(func() {
		result <- 42
	})

	select {
	case v := <-result:
		if v != 42 {
			t.Errorf("got %d, want 42", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for job result")
	}
}
