package service

import (
	"context"

	"go.uber.org/zap"
)

// rateLimitTracker는 동시 요청 수와 분당 요청 수를 비동기로 추적하는 공통 컴포넌트다.
// Pipeline과 StreamPipeline이 공유한다.
type rateLimitTracker struct {
	rateLimiter RateLimiter
	stateSvc    *TokenStateService
	logger      *zap.Logger
}

func (t *rateLimitTracker) trackAsync(apiTokenID int64) {
	go func() {
		ctx := context.Background()
		if _, err := t.rateLimiter.IncrementConcurrent(ctx, apiTokenID); err != nil {
			t.logger.Warn("동시 요청 카운터 증가 실패", zap.Int64("tokenId", apiTokenID), zap.Error(err))
		}
		if _, err := t.rateLimiter.IncrementAndGet(ctx, apiTokenID); err != nil {
			t.logger.Warn("분당 요청 카운터 증가 실패", zap.Int64("tokenId", apiTokenID), zap.Error(err))
			return
		}
		if err := t.rateLimiter.CheckAndUpdateState(ctx, apiTokenID, t.stateSvc); err != nil {
			t.logger.Warn("상태 검사 실패", zap.Int64("tokenId", apiTokenID), zap.Error(err))
		}
	}()
}

func (t *rateLimitTracker) releaseAsync(apiTokenID int64) {
	go func() {
		if _, err := t.rateLimiter.DecrementConcurrent(context.Background(), apiTokenID); err != nil {
			t.logger.Warn("동시 요청 카운터 감소 실패", zap.Int64("tokenId", apiTokenID), zap.Error(err))
		}
	}()
}
