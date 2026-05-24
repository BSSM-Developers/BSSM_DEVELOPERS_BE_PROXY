package query

import (
	"context"
	"regexp"
	"strings"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/cache"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/repository"
)

// UsageQueryService는 캐시를 통한 ApiUsage 조회를 담당한다.
// Java의 ApiUsageReactiveQueryService에 대응한다.
type UsageQueryService struct {
	repo  repository.UsageRepository
	cache cache.Service
}

func NewUsageQueryService(repo repository.UsageRepository, cache cache.Service) *UsageQueryService {
	return &UsageQueryService{repo: repo, cache: cache}
}

// FindByTokenAndEndpoint는 토큰 ID와 실제 요청 경로를 비교하여 매칭 ApiUsage를 반환한다.
// 엔드포인트 템플릿의 {param} 패턴을 정규식으로 변환하여 매칭한다.
func (s *UsageQueryService) FindByTokenAndEndpoint(ctx context.Context, tokenID int64, endpoint string) (*model.ApiUsage, error) {
	// 쿼리 파라미터 제거 후 경로만 추출
	actualPath := strings.SplitN(endpoint, "?", 2)[0]

	key := cache.ApiUsageListKey(tokenID)
	var usages []model.ApiUsage

	err := s.cache.GetOrFetchSlice(ctx, key, func() (any, error) {
		return s.repo.FindAllByTokenID(ctx, tokenID)
	}, &usages)

	if err != nil {
		return nil, err
	}

	for _, u := range usages {
		if matchesTemplate(u.Endpoint, actualPath) {
			return &u, nil
		}
	}
	return nil, apperrors.ErrApiUsageNotFound
}

// matchesTemplate는 {param} 형태의 경로 템플릿을 정규식으로 변환하여 일치 여부를 확인한다.
func matchesTemplate(template, actualPath string) bool {
	// {paramName} → [^/?]+ 로 변환
	pattern := regexp.MustCompile(`\{[^}]+}`).ReplaceAllString(template, "[^/?]+")
	matched, _ := regexp.MatchString("^"+pattern+"$", actualPath)
	return matched
}
