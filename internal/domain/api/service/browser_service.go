package service

import (
	"context"
	"net/http"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/query"
	logmodel "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/requester"
)

// BrowserService는 브라우저 클라이언트 프록시 요청을 처리한다.
// Origin 헤더를 등록된 허용 도메인 목록과 비교하여 접근을 검증한다.
// Java의 BrowserUseApiService에 대응한다.
type BrowserService struct {
	domainQuery *query.DomainQueryService
	pipeline    *Pipeline
}

func NewBrowserService(domainQuery *query.DomainQueryService, pipeline *Pipeline) *BrowserService {
	return &BrowserService{domainQuery: domainQuery, pipeline: pipeline}
}

func (s *BrowserService) Handle(
	ctx context.Context,
	token string,
	r *http.Request,
	body []byte,
) (*requester.ProxyResponse, error) {
	origin := r.Header.Get("Origin")
	info := model.NewRequestInfo(r, body)

	return s.pipeline.Execute(
		ctx, token, info, r,
		logmodel.DirectionBrowserToServer,
		func(ctx context.Context, apiToken *model.ApiToken) error {
			domains, err := s.domainQuery.FindAllByTokenID(ctx, apiToken.ApiTokenID)
			if err != nil {
				return err
			}
			return apiToken.ValidateBrowserAccess(origin, domains)
		},
	)
}
