package service

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/query"
	logservice "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/service"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/requester"
	"go.uber.org/zap"
)


// TokenValidator는 브라우저/서버 각 접근 방식의 검증 로직을 주입받는 함수 타입이다.
// Java의 Function<ApiTokenR2dbc, Mono<Void>> tokenValidator에 대응한다.
type TokenValidator func(ctx context.Context, token *model.ApiToken) error

// Pipeline은 프록시 요청의 공통 실행 흐름을 정의한다.
// 토큰 조회 → WARNING 복구 → 접근 검증 → 차단 확인 → IP 체크 → 사용량 조회 → 외부 API 호출 → 로그 발행
// Java의 ApiProxyPipeline에 대응한다.
type Pipeline struct {
	tokenQuery   *query.TokenQueryService
	usageQuery   *query.UsageQueryService
	httpReq      requester.Requester
	logPublisher *logservice.LogPublisher
	tracker      rateLimitTracker
}

func NewPipeline(
	tokenQuery *query.TokenQueryService,
	usageQuery *query.UsageQueryService,
	httpReq requester.Requester,
	logPublisher *logservice.LogPublisher,
	rateLimiter RateLimiter,
	stateSvc *TokenStateService,
	logger *zap.Logger,
) *Pipeline {
	return &Pipeline{
		tokenQuery:   tokenQuery,
		usageQuery:   usageQuery,
		httpReq:      httpReq,
		logPublisher: logPublisher,
		tracker:      rateLimitTracker{rateLimiter: rateLimiter, stateSvc: stateSvc, logger: logger},
	}
}

// Execute는 브라우저/서버 공통 프록시 파이프라인을 실행한다.
// validator는 호출자(BrowserService/ServerService)가 주입하는 접근 검증 전략이다.
func (p *Pipeline) Execute(
	ctx context.Context,
	clientID string,
	info *model.RequestInfo,
	r *http.Request,
	direction string,
	validator TokenValidator,
) (*requester.ProxyResponse, error) {
	startedAt := time.Now().UnixMilli()

	// 토큰 조회
	token, err := p.tokenQuery.FindByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}

	// WARNING TTL 만료 시 NORMAL 자동복구
	p.tracker.tryRecoverWarning(ctx, token)

	// 접근 방식별 검증 (브라우저: Origin, 서버: SecretKey)
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

	// 비동기: Rate limit 카운터 증가 + 상태 전환 검사
	p.tracker.trackAsync(token.ApiTokenID)

	// 사용 가능한 엔드포인트 조회
	usage, err := p.usageQuery.FindByTokenAndEndpoint(ctx, token.ApiTokenID, info.Endpoint)
	if err != nil {
		return nil, err
	}

	// 외부 API 호출
	resp, err := p.httpReq.Request(ctx, usage.Domain, info)

	// 비동기: 동시 요청 수 감소
	p.tracker.releaseAsync(token.ApiTokenID)

	if err != nil {
		p.logPublisher.PublishError(direction, token, info, r, err, startedAt)
		return nil, err
	}

	p.logPublisher.PublishSuccess(direction, token, info, r, resp, startedAt)
	return resp, nil
}

// extractClientIP는 X-Forwarded-For → X-Real-IP → RemoteAddr 순으로 클라이언트 IP를 추출한다.
// 서비스가 X-Forwarded-For를 전달하면 실제 최종 사용자 IP로 추적된다.
func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
