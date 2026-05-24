package service

import (
	"context"
	"net/http"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/requester"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/validator"
)

// HealthCheckResponse는 API 상태 확인 결과다.
type HealthCheckResponse struct {
	Healthy bool   `json:"healthy"`
	Body    string `json:"body,omitempty"`
}

// HealthService는 등록된 외부 API의 가용성을 확인한다.
// Java의 HealthCheckApiService에 대응한다.
type HealthService struct {
	httpReq   requester.Requester
	validator *validator.DomainValidator
}

func NewHealthService(httpReq requester.Requester, v *validator.DomainValidator) *HealthService {
	return &HealthService{httpReq: httpReq, validator: v}
}

func (s *HealthService) Check(
	ctx context.Context,
	endpoint, method, domain string,
	r *http.Request,
	body []byte,
) (*HealthCheckResponse, error) {
	if err := s.validator.Validate(domain); err != nil {
		return nil, err
	}

	info := model.NewRequestInfo(r, body)
	info.Endpoint = endpoint
	info.Method = method

	resp, err := s.httpReq.Request(ctx, domain, info)
	if err != nil {
		return &HealthCheckResponse{Healthy: false}, nil
	}

	return &HealthCheckResponse{
		Healthy: resp.StatusCode < 400,
		Body:    string(resp.Body),
	}, nil
}
