package core

import (
	"context"
	"sync"
	"time"
)

const allocateGateInterval = 100 * time.Millisecond

// allocationGate spaces TURN Allocate attempts. A Core owns one gate, so its
// workers share a quota without unrelated Core instances blocking each other.
type allocationGate struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}

func newAllocationGate(interval time.Duration) *allocationGate {
	return &allocationGate{interval: interval}
}

func (g *allocationGate) Wait(ctx context.Context) error {
	g.mu.Lock()
	due := time.Now()
	if g.next.After(due) {
		due = g.next
	}
	g.next = due.Add(g.interval)
	g.mu.Unlock()

	wait := time.Until(due)
	if wait <= 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
