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

	// GetPeakConcurrent는 현재 분 버킷의 최대 동시 요청 수를 반환한다.
	GetPeakConcurrent(ctx context.Context, apiTokenID int64) (int64, error)

	// IsWarningActive는 WARNING 상태 TTL 키가 아직 유효한지 확인한다.
	IsWarningActive(ctx context.Context, apiTokenID int64) (bool, error)

	// CheckClientIP는 클라이언트 IP별 분당 요청 수를 증가시키고 초과 시 에러를 반환한다.
	CheckClientIP(ctx context.Context, apiTokenID int64, clientIP string) error

	// CheckAndUpdateState는 요청 수가 임계치를 초과하면 상태 전환을 요청한다.
	CheckAndUpdateState(ctx context.Context, apiTokenID int64, stateSvc *TokenStateService) error
}
