package query

import (
	"context"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/cache"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/repository"
)

// DomainQueryService는 캐시를 통한 TokenDomain 조회를 담당한다.
// Java의 TokenDomainReactiveQueryService에 대응한다.
type DomainQueryService struct {
	repo  repository.DomainRepository
	cache cache.Service
}

func NewDomainQueryService(repo repository.DomainRepository, cache cache.Service) *DomainQueryService {
	return &DomainQueryService{repo: repo, cache: cache}
}

func (s *DomainQueryService) FindAllByTokenID(ctx context.Context, tokenID int64) ([]model.TokenDomain, error) {
	key := cache.TokenDomainKey(tokenID)
	var domains []model.TokenDomain

	err := s.cache.GetOrFetchSlice(ctx, key, func() (any, error) {
		return s.repo.FindAllByTokenID(ctx, tokenID)
	}, &domains)

	return domains, err
}
