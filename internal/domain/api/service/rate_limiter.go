package service

import "context"

// RateLimiter는 API 토큰별 요청 제한 추적 계약이다.
// Java의 ApiRateLimiterService 인터페이스에 대응한다.
type RateLimiter interface {
	// IncrementAndGet은 분당 요청 수를 증가시키고 현재 값을 반환한다.
	IncrementAndGet(ctx context.Context, apiTokenID int64) (int64, error)

	// IncrementConcurrent는 동시 요청 수를 증가시킨다.
	IncrementConcurrent(ctx context.Context, apiTokenID int64) (int64, error)

	// DecrementConcurrent는 동시 요청 수를 감소시킨다.
	DecrementConcurrent(ctx context.Context, apiTokenID int64) (int64, error)

	// GetConcurrent는 현재 동시 요청 수를 반환한다.
	GetConcurrent(ctx context.Context, apiTokenID int64) (int64, error)

	// CheckAndUpdateState는 요청 수가 임계치를 초과하면 상태 전환을 요청한다.
	CheckAndUpdateState(ctx context.Context, apiTokenID int64, stateSvc *TokenStateService) error
}
