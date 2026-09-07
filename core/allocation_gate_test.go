package core

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestAllocationGatePacesConcurrentAttempts(t *testing.T) {
	interval := 25 * time.Millisecond
	g := newAllocationGate(interval)
	start := make(chan struct{})
	attempts := make(chan time.Time, 3)
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := g.Wait(context.Background()); err != nil {
				t.Error(err)
				return
			}
			attempts <- time.Now()
		}()
	}
	close(start)
	wg.Wait()
	close(attempts)

	got := make([]time.Time, 0, 3)
	for at := range attempts {
		got = append(got, at)
	}
	if len(got) != 3 {
		t.Fatalf("completed attempts = %d, want 3", len(got))
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Before(got[j]) })
	for i := 1; i < len(got); i++ {
		if elapsed := got[i].Sub(got[i-1]); elapsed < interval/2 {
			t.Fatalf("attempts %d and %d were not paced: %v", i-1, i, elapsed)
		}
	}
}

func TestAllocationGateWaitCancels(t *testing.T) {
	g := newAllocationGate(time.Second)
	if err := g.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := g.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait() error = %v, want context.Canceled", err)
	}
}
