package service

import (
	"context"
	"net/http"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	logmodel "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/model"
)

// ServerStreamService는 서버-투-서버 스트리밍 프록시 요청을 처리한다.
// ServerService의 bcrypt 검증 로직을 재사용한다 (같은 캐시 공유).
type ServerStreamService struct {
	serverSvc      *ServerService
	streamPipeline *StreamPipeline
}

func NewServerStreamService(
	serverSvc *ServerService,
	streamPipeline *StreamPipeline,
) *ServerStreamService {
	return &ServerStreamService{serverSvc: serverSvc, streamPipeline: streamPipeline}
}

func (s *ServerStreamService) Handle(
	ctx context.Context,
	secretKey string,
	token string,
	r *http.Request,
	body []byte,
) (*StreamSession, error) {
	info := model.NewRequestInfo(r, body)

	return s.streamPipeline.Execute(
		ctx, token, info, r,
		logmodel.DirectionServerToServer,
		func(_ context.Context, apiToken *model.ApiToken) error {
			return s.serverSvc.validateSecretKey(apiToken, secretKey)
		},
	)
}
