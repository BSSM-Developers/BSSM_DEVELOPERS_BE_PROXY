package query

import (
	"context"
	"errors"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/cache"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/repository"
	"gorm.io/gorm"
)

// TokenQueryService는 캐시를 통한 ApiToken 조회를 담당한다.
// Java의 ApiTokenReactiveQueryService에 대응한다.
type TokenQueryService struct {
	repo  repository.TokenRepository
	cache cache.Service
}

func NewTokenQueryService(repo repository.TokenRepository, cache cache.Service) *TokenQueryService {
	return &TokenQueryService{repo: repo, cache: cache}
}

func (s *TokenQueryService) FindByClientID(ctx context.Context, clientID string) (*model.ApiToken, error) {
	key := cache.ApiTokenKey(clientID)
	var token model.ApiToken

	err := s.cache.GetOrFetch(ctx, key, func() (any, error) {
		return s.repo.FindByClientID(ctx, clientID)
	}, &token)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrApiTokenNotFound
		}
		return nil, err
	}
	return &token, nil
}
