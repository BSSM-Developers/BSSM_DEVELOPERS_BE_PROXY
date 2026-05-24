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

// TokenValidator는 브라우저/서버 각 접근 방식의 검증 로직을 주입받는 함수 타입이다.
// Java의 Function<ApiTokenR2dbc, Mono<Void>> tokenValidator에 대응한다.
type TokenValidator func(ctx context.Context, token *model.ApiToken) error

// Pipeline은 프록시 요청의 공통 실행 흐름을 정의한다.
// 토큰 조회 → 접근 검증 → 차단 확인 → 사용량 조회 → 외부 API 호출 → 로그 발행
// Java의 ApiProxyPipeline에 대응한다.
type Pipeline struct {
	tokenQuery   *query.TokenQueryService
	usageQuery   *query.UsageQueryService
	httpReq      requester.Requester
	logPublisher *logservice.LogPublisher
	rateLimiter  RateLimiter
	stateSvc     *TokenStateService
	logger       *zap.Logger
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
		rateLimiter:  rateLimiter,
		stateSvc:     stateSvc,
		logger:       logger,
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

	// 접근 방식별 검증 (브라우저: Origin, 서버: SecretKey)
	if err := validator(ctx, token); err != nil {
		return nil, err
	}

	// 차단 상태 확인
	if err := token.ValidateNotBlocked(); err != nil {
		return nil, err
	}

	// 비동기: Rate limit 카운터 증가 + 상태 전환 검사
	go p.trackRequest(token.ApiTokenID)

	// 사용 가능한 엔드포인트 조회
	usage, err := p.usageQuery.FindByTokenAndEndpoint(ctx, token.ApiTokenID, info.Endpoint)
	if err != nil {
		return nil, err
	}

	// 외부 API 호출
	resp, err := p.httpReq.Request(ctx, usage.Domain, info)

	// 비동기: 동시 요청 수 감소
	go p.releaseRequest(token.ApiTokenID)

	if err != nil {
		p.logPublisher.PublishError(direction, token, info, r, err, startedAt)
		return nil, err
	}

	p.logPublisher.PublishSuccess(direction, token, info, r, resp, startedAt)
	return resp, nil
}

// trackRequest는 동시 요청 수와 분당 요청 수를 비동기로 증가시키고 상태를 검사한다.
// Java의 ApiRequestStartedEvent 핸들러에 대응한다.
func (p *Pipeline) trackRequest(apiTokenID int64) {
	ctx := context.Background()
	if _, err := p.rateLimiter.IncrementConcurrent(ctx, apiTokenID); err != nil {
		p.logger.Warn("동시 요청 카운터 증가 실패", zap.Int64("tokenId", apiTokenID), zap.Error(err))
	}
	if _, err := p.rateLimiter.IncrementAndGet(ctx, apiTokenID); err != nil {
		p.logger.Warn("분당 요청 카운터 증가 실패", zap.Int64("tokenId", apiTokenID), zap.Error(err))
		return
	}
	if err := p.rateLimiter.CheckAndUpdateState(ctx, apiTokenID, p.stateSvc); err != nil {
		p.logger.Warn("상태 검사 실패", zap.Int64("tokenId", apiTokenID), zap.Error(err))
	}
}

// releaseRequest는 동시 요청 수를 비동기로 감소시킨다.
// Java의 ApiRequestCompletedEvent 핸들러에 대응한다.
func (p *Pipeline) releaseRequest(apiTokenID int64) {
	if _, err := p.rateLimiter.DecrementConcurrent(context.Background(), apiTokenID); err != nil {
		p.logger.Warn("동시 요청 카운터 감소 실패",
			zap.Int64("tokenId", apiTokenID),
			zap.Error(err),
		)
	}
}

