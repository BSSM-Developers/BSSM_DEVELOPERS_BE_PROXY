package service

import (
	"context"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/cache"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TokenStateService는 API 토큰 상태를 NORMAL→WARNING→BLOCKED 순으로 전환한다.
// Java의 ApiTokenStateUpdateService에 대응한다.
type TokenStateService struct {
	repo   repository.TokenRepository
	cache  cache.Service
	logger *zap.Logger
}

func NewTokenStateService(repo repository.TokenRepository, cache cache.Service, logger *zap.Logger) *TokenStateService {
	return &TokenStateService{repo: repo, cache: cache, logger: logger}
}

// TransitionState는 현재 상태에서 다음 단계로 전환한다.
// 상태가 변경된 경우 새 상태를 반환하고, 이미 BLOCKED이면 빈 문자열을 반환한다.
func (s *TokenStateService) TransitionState(ctx context.Context, apiTokenID int64) (model.ApiTokenState, error) {
	token, err := s.repo.FindByID(ctx, apiTokenID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}

	prev := token.State
	next := token.TransitionToNextState()
	if next == "" {
		return "", nil
	}

	if err := s.repo.Save(ctx, token); err != nil {
		return "", err
	}

	// BLOCKED 전환 시 캐시 즉시 무효화 — TTL 내 차단 우회 방지
	if next == model.StateBlocked {
		if err := s.cache.Evict(ctx, cache.ApiTokenKey(token.ApiTokenUUID)); err != nil {
			s.logger.Warn("BLOCKED 토큰 캐시 evict 실패",
				zap.Int64("apiTokenId", apiTokenID),
				zap.Error(err),
			)
		}
	}

	s.logger.Warn("API 토큰 상태 변경",
		zap.Int64("apiTokenId", apiTokenID),
		zap.String("from", string(prev)),
		zap.String("to", string(next)),
	)
	return next, nil
}
