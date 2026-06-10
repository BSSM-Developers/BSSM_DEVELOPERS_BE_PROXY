package service

import (
	"context"
	"net/http"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/query"
	logservice "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/service"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/requester"
	"go.uber.org/zap"
)

// StreamSession은 스트리밍 파이프라인 실행 결과다.
// 핸들러는 io.Copy 완료 후 반드시 Complete를 호출해야 한다.
type StreamSession struct {
	Response *requester.StreamResponse
	// Complete는 스트리밍 완료 시 로그를 발행하고 rate limit 카운터를 감소시킨다.
	// streamErr이 nil이면 SUCCESS, 아니면 ERROR로 기록한다.
	Complete func(bytesTransferred int64, streamErr error)
}

// StreamPipeline은 스트리밍 프록시 요청의 공통 실행 흐름을 정의한다.
// Pipeline과 달리 응답 바디를 버퍼링하지 않고 io.ReadCloser로 반환한다.
type StreamPipeline struct {
	tokenQuery   *query.TokenQueryService
	usageQuery   *query.UsageQueryService
	httpReq      requester.StreamRequester
	logPublisher *logservice.LogPublisher
	tracker      rateLimitTracker
}

func NewStreamPipeline(
	tokenQuery *query.TokenQueryService,
	usageQuery *query.UsageQueryService,
	httpReq requester.StreamRequester,
	logPublisher *logservice.LogPublisher,
	rateLimiter RateLimiter,
	stateSvc *TokenStateService,
	logger *zap.Logger,
) *StreamPipeline {
	return &StreamPipeline{
		tokenQuery:   tokenQuery,
		usageQuery:   usageQuery,
		httpReq:      httpReq,
		logPublisher: logPublisher,
		tracker:      rateLimitTracker{rateLimiter: rateLimiter, stateSvc: stateSvc, logger: logger},
	}
}

// Execute는 스트리밍 프록시 파이프라인을 실행한다.
// 성공 시 StreamSession을 반환하며, 핸들러가 io.Copy 후 Complete를 호출해야 로그가 발행된다.
func (p *StreamPipeline) Execute(
	ctx context.Context,
	clientID string,
	info *model.RequestInfo,
	r *http.Request,
	direction string,
	validator TokenValidator,
) (*StreamSession, error) {
	startedAt := time.Now().UnixMilli()

	token, err := p.tokenQuery.FindByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}

	// WARNING TTL 만료 시 NORMAL 자동복구
	p.tracker.tryRecoverWarning(ctx, token)

	if err := validator(ctx, token); err != nil {
		return nil, err
	}

	// 차단 상태 확인 (관리자 수동 BLOCKED만 해당)
	if err := token.ValidateNotBlocked(); err != nil {
		return nil, err
	}

	// IP 단위 rate limit — 공격자 IP 차단, 동일 토큰 정상 유저 보호
	clientIP := extractClientIP(r)
	if err := p.tracker.checkClientIP(ctx, token.ApiTokenID, clientIP); err != nil {
		return nil, err
	}

	p.tracker.trackAsync(token.ApiTokenID)

	usage, err := p.usageQuery.FindByTokenAndEndpoint(ctx, token.ApiTokenID, info.Endpoint)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpReq.RequestStream(ctx, usage.Domain, info)
	if err != nil {
		p.logPublisher.PublishError(direction, token, info, r, err, startedAt)
		p.tracker.releaseAsync(token.ApiTokenID)
		return nil, err
	}

	complete := func(bytesTransferred int64, streamErr error) {
		p.logPublisher.PublishStreamResult(direction, token, info, r, resp.StatusCode, bytesTransferred, streamErr, startedAt)
		p.tracker.releaseAsync(token.ApiTokenID)
	}

	return &StreamSession{Response: resp, Complete: complete}, nil
}
