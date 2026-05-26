package service

import (
	"context"
	"net/http"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/query"
	logmodel "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/model"
)

// BrowserStreamService는 브라우저 클라이언트의 스트리밍 프록시 요청을 처리한다.
// BrowserService와 동일한 Origin 검증을 사용하며, 응답을 버퍼링하지 않는다.
type BrowserStreamService struct {
	domainQuery    *query.DomainQueryService
	streamPipeline *StreamPipeline
}

func NewBrowserStreamService(
	domainQuery *query.DomainQueryService,
	streamPipeline *StreamPipeline,
) *BrowserStreamService {
	return &BrowserStreamService{domainQuery: domainQuery, streamPipeline: streamPipeline}
}

func (s *BrowserStreamService) Handle(
	ctx context.Context,
	token string,
	r *http.Request,
	body []byte,
) (*StreamSession, error) {
	origin := r.Header.Get("Origin")
	info := model.NewRequestInfo(r, body)

	return s.streamPipeline.Execute(
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
