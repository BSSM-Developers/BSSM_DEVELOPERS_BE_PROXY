package queue

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/config"
)

// SemaphoreQueue는 FIFO 방식의 단순 세마포어 기반 큐다.
// priority는 무시되며 선착순으로 처리된다.
// Java의 SemaphoreRequestQueue에 대응한다.
type SemaphoreQueue struct {
	mu      sync.Mutex
	waiters []*simpleWaiter
	sem     chan struct{}
	timeout time.Duration
}

func NewSemaphoreQueue(cfg config.QueueConfig) RequestQueue {
	sem := make(chan struct{}, cfg.MaxInflight)
	for i := 0; i < cfg.MaxInflight; i++ {
		sem <- struct{}{}
	}
	return &SemaphoreQueue{
		waiters: make([]*simpleWaiter, 0),
		sem:     sem,
		timeout: cfg.AcquireTimeout,
	}
}

func (q *SemaphoreQueue) Acquire(ctx context.Context, _ string, _ float64) (bool, error) {
	select {
	case <-q.sem:
		return true, nil
	default:
	}

	w := &simpleWaiter{ch: make(chan bool, 1)}

	q.mu.Lock()
	q.waiters = append(q.waiters, w)
	q.mu.Unlock()

	go func() {
		timer := time.NewTimer(q.timeout)
		defer timer.Stop()
		select {
		case <-timer.C:
			if w.complete(false) {
				q.mu.Lock()
				q.removeWaiter(w)
				q.mu.Unlock()
			}
		case <-ctx.Done():
			if w.complete(false) {
				q.mu.Lock()
				q.removeWaiter(w)
				q.mu.Unlock()
			}
		}
	}()

	return <-w.ch, nil
}

func (q *SemaphoreQueue) Release() {
	for {
		q.mu.Lock()
		if len(q.waiters) == 0 {
			q.mu.Unlock()
			q.sem <- struct{}{}
			return
		}
		w := q.waiters[0]
		q.waiters = q.waiters[1:]
		q.mu.Unlock()

		if w.complete(true) {
			return
		}
	}
}

func (q *SemaphoreQueue) removeWaiter(target *simpleWaiter) {
	for i, w := range q.waiters {
		if w == target {
			q.waiters = append(q.waiters[:i], q.waiters[i+1:]...)
			return
		}
	}
}

type simpleWaiter struct {
	ch   chan bool
	done atomic.Bool
}

func (w *simpleWaiter) complete(result bool) bool {
	if !w.done.CompareAndSwap(false, true) {
		return false
	}
	w.ch <- result
	return true
}
