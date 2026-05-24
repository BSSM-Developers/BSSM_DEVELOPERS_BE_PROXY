package queue

import "context"

// RequestQueue는 요청 동시성 제어 큐의 계약이다.
// Java의 RequestQueue 인터페이스에 대응한다.
type RequestQueue interface {
	// Acquire는 큐 슬롯을 획득한다.
	// 슬롯이 없으면 timeout까지 대기한다. 대기 중 ctx가 취소되면 즉시 반환한다.
	// 반환값: (획득 성공 여부, 에러)
	Acquire(ctx context.Context, clientID string, priority float64) (bool, error)

	// Release는 보유한 슬롯을 반환한다.
	// 대기 중인 waiter가 있으면 우선순위가 높은 waiter에게 슬롯을 이전한다.
	Release()
}
