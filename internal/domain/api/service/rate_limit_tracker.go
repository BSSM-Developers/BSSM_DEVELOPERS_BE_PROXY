package service

import (
	"context"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
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

// tryRecoverWarning은 WARNING TTL이 만료된 토큰을 NORMAL로 자동 복구한다.
func (t *rateLimitTracker) tryRecoverWarning(ctx context.Context, token *model.ApiToken) {
	if token.State != model.StateWarning {
		return
	}
	active, err := t.rateLimiter.IsWarningActive(ctx, token.ApiTokenID)
	if err != nil {
		t.logger.Warn("WARNING 상태 TTL 확인 실패", zap.Int64("tokenId", token.ApiTokenID), zap.Error(err))
		return
	}
	if active {
		return
	}
	if err := t.stateSvc.RecoverToNormal(ctx, token); err != nil {
		t.logger.Warn("WARNING 자동복구 실패", zap.Int64("tokenId", token.ApiTokenID), zap.Error(err))
		return
	}
	t.logger.Info("WARNING 상태 자동복구 완료", zap.Int64("tokenId", token.ApiTokenID))
}

// checkClientIP는 클라이언트 IP별 분당 요청 수를 확인한다.
func (t *rateLimitTracker) checkClientIP(ctx context.Context, apiTokenID int64, clientIP string) error {
	return t.rateLimiter.CheckClientIP(ctx, apiTokenID, clientIP)
}
