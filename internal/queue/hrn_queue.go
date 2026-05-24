package queue

import (
	"container/heap"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/config"
)

// HRNQueue는 HRN(Highest Response Ratio Next) 기반 우선순위 요청 큐다.
// 실패 횟수에 따른 우선순위와 대기 시간(aging)을 고려하여 스케줄링한다.
// Java의 HrnRequestQueue에 대응한다.
type HRNQueue struct {
	mu      sync.Mutex
	heap    waitHeap
	sem     chan struct{}
	timeout time.Duration
}

func NewHRNQueue(cfg config.QueueConfig) RequestQueue {
	sem := make(chan struct{}, cfg.MaxInflight)
	for i := 0; i < cfg.MaxInflight; i++ {
		sem <- struct{}{}
	}
	q := &HRNQueue{
		heap:    make(waitHeap, 0),
		sem:     sem,
		timeout: cfg.AcquireTimeout,
	}
	heap.Init(&q.heap)
	return q
}

func (q *HRNQueue) Acquire(ctx context.Context, clientID string, priority float64) (bool, error) {
	// 즉시 획득 시도
	select {
	case <-q.sem:
		return true, nil
	default:
	}

	// 슬롯이 없으면 큐에서 대기
	w := &waiter{
		clientID:   clientID,
		priority:   priority,
		enqueuedAt: time.Now(),
		ch:         make(chan bool, 1),
		index:      -1,
	}

	q.mu.Lock()
	heap.Push(&q.heap, w)
	q.mu.Unlock()

	// 타임아웃 고루틴: timeout 경과 시 waiter를 실패 처리하고 큐에서 제거
	go func() {
		timer := time.NewTimer(q.timeout)
		defer timer.Stop()
		select {
		case <-timer.C:
			if w.complete(false) {
				q.mu.Lock()
				if w.index >= 0 {
					heap.Remove(&q.heap, w.index)
				}
				q.mu.Unlock()
			}
		case <-ctx.Done():
			if w.complete(false) {
				q.mu.Lock()
				if w.index >= 0 {
					heap.Remove(&q.heap, w.index)
				}
				q.mu.Unlock()
			}
		}
	}()

	result := <-w.ch
	return result, nil
}

func (q *HRNQueue) Release() {
	// 대기 중인 waiter 중 우선순위가 가장 높은 것에 슬롯 이전
	// complete가 실패하면 이미 타임아웃된 waiter이므로 다음을 시도한다
	for {
		q.mu.Lock()
		if q.heap.Len() == 0 {
			q.mu.Unlock()
			q.sem <- struct{}{}
			return
		}
		w := heap.Pop(&q.heap).(*waiter)
		q.mu.Unlock()

		if w.complete(true) {
			return
		}
	}
}

// --- waiter ---

type waiter struct {
	clientID   string
	priority   float64
	enqueuedAt time.Time
	ch         chan bool
	index      int
	done       atomic.Bool
}

// complete는 원자적으로 결과를 전송한다. 최초 호출자만 true를 반환한다.
func (w *waiter) complete(result bool) bool {
	if !w.done.CompareAndSwap(false, true) {
		return false
	}
	w.ch <- result
	return true
}

// effectivePriority는 HRN 알고리즘에 따라 대기 시간이 길수록 우선순위를 높인다.
func (w *waiter) effectivePriority() float64 {
	waitSecs := time.Since(w.enqueuedAt).Seconds()
	return w.priority + (waitSecs * 0.1)
}

// --- heap.Interface ---

type waitHeap []*waiter

func (h waitHeap) Len() int { return len(h) }

// Less: effectivePriority가 높은 waiter가 먼저 처리된다 (max-heap)
func (h waitHeap) Less(i, j int) bool {
	return h[i].effectivePriority() > h[j].effectivePriority()
}

func (h waitHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *waitHeap) Push(x any) {
	w := x.(*waiter)
	w.index = len(*h)
	*h = append(*h, w)
}

func (h *waitHeap) Pop() any {
	old := *h
	n := len(old)
	w := old[n-1]
	old[n-1] = nil
	w.index = -1
	*h = old[:n-1]
	return w
}
