package service

import "context"

// NoopRateLimiter는 rate limit이 비활성화된 환경에서 사용하는 no-op 구현체다.
// dev 환경에서 rate_limit.enabled: false 설정 시 주입된다.
type NoopRateLimiter struct{}

func NewNoopRateLimiter() RateLimiter {
	return &NoopRateLimiter{}
}

func (n *NoopRateLimiter) IncrementAndGet(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

func (n *NoopRateLimiter) IncrementConcurrent(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

func (n *NoopRateLimiter) DecrementConcurrent(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

func (n *NoopRateLimiter) GetConcurrent(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

func (n *NoopRateLimiter) GetPeakConcurrent(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}

func (n *NoopRateLimiter) IsWarningActive(_ context.Context, _ int64) (bool, error) {
	return false, nil
}

func (n *NoopRateLimiter) CheckClientIP(_ context.Context, _ int64, _ string) error {
	return nil
}

func (n *NoopRateLimiter) CheckAndUpdateState(_ context.Context, _ int64, _ *TokenStateService) error {
	return nil
}
