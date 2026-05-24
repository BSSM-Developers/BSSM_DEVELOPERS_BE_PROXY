package service

import (
	"context"
	"net/http"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	logmodel "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/requester"
)

// ServerService는 서버-투-서버 프록시 요청을 처리한다.
// bssm-dev-secret 헤더의 bcrypt 시크릿 키를 검증하여 접근을 허가한다.
// Java의 ServerUseApiService에 대응한다.
type ServerService struct {
	pipeline *Pipeline
}

func NewServerService(pipeline *Pipeline) *ServerService {
	return &ServerService{pipeline: pipeline}
}

func (s *ServerService) Handle(
	ctx context.Context,
	secretKey string,
	token string,
	r *http.Request,
	body []byte,
) (*requester.ProxyResponse, error) {
	info := model.NewRequestInfo(r, body)

	return s.pipeline.Execute(
		ctx, token, info, r,
		logmodel.DirectionServerToServer,
		func(_ context.Context, apiToken *model.ApiToken) error {
			return apiToken.ValidateServerAccess(secretKey)
		},
	)
}
