package logservice

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	logmodel "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/model"
	logrepository "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/repository"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/requester"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

const (
	logBufferSize = 512
	zoneKST       = "Asia/Seoul"
)

// LogPublisher는 프록시 요청/응답 로그를 비동기로 MongoDB에 저장한다.
// Java의 ProxyLogEventPublisher + ProxyLogEventListener에 대응한다.
// 이벤트 기반 대신 버퍼드 채널 + 다중 worker goroutine으로 비동기 처리한다.
type LogPublisher struct {
	repo    logrepository.LogRepository
	ch      chan *logmodel.ProxyLog
	workers int
	logger  *zap.Logger
}

func NewLogPublisher(repo logrepository.LogRepository, logger *zap.Logger, workers int) *LogPublisher {
	if workers <= 0 {
		workers = 1
	}
	return &LogPublisher{
		repo:    repo,
		ch:      make(chan *logmodel.ProxyLog, logBufferSize),
		workers: workers,
		logger:  logger,
	}
}

// Start는 workers 수만큼 로그 소비 goroutine을 시작한다.
// ctx가 취소되면 각 worker가 채널을 드레인하고 종료한다.
func (p *LogPublisher) Start(ctx context.Context) {
	for i := 0; i < p.workers; i++ {
		go p.runWorker(ctx)
	}
}

func (p *LogPublisher) runWorker(ctx context.Context) {
	for {
		select {
		case log := <-p.ch:
			if err := p.repo.Save(context.Background(), log); err != nil {
				p.logger.Error("proxy log 저장 실패", zap.Error(err))
			}
		case <-ctx.Done():
			for {
				select {
				case log := <-p.ch:
					p.repo.Save(context.Background(), log)
				default:
					return
				}
			}
		}
	}
}

// PublishSuccess는 성공 응답 로그를 비동기로 발행한다.
func (p *LogPublisher) PublishSuccess(
	direction string,
	token *model.ApiToken,
	info *model.RequestInfo,
	r *http.Request,
	resp *requester.ProxyResponse,
	startedAt int64,
) {
	log := p.buildLog(direction, token, info, r, startedAt)
	log.Result = logmodel.ResultSuccess
	log.Response = buildResponseLog(resp)
	p.publish(log)
}

// PublishError는 에러 응답 로그를 비동기로 발행한다.
func (p *LogPublisher) PublishError(
	direction string,
	token *model.ApiToken,
	info *model.RequestInfo,
	r *http.Request,
	err error,
	startedAt int64,
) {
	log := p.buildLog(direction, token, info, r, startedAt)
	log.Result = logmodel.ResultError
	log.Error = buildErrorLog(err)

	status := 500
	var body string
	if extErr, ok := err.(*apperrors.ExternalAPIError); ok {
		status = extErr.UpstreamStatusCode
		body = extErr.UpstreamBody
	}
	log.Response = &logmodel.ResponseLog{
		Status:  status,
		Headers: map[string]string{},
		Body:    body,
	}
	p.publish(log)
}

func (p *LogPublisher) publish(log *logmodel.ProxyLog) {
	select {
	case p.ch <- log:
	default:
		p.logger.Warn("log buffer full, dropping log", zap.String("traceId", log.TraceID))
	}
}

func (p *LogPublisher) buildLog(
	direction string,
	token *model.ApiToken,
	info *model.RequestInfo,
	r *http.Request,
	startedAt int64,
) *logmodel.ProxyLog {
	kst, _ := time.LoadLocation(zoneKST)
	now := time.Now().In(kst)

	return &logmodel.ProxyLog{
		ID:        primitive.NewObjectID(),
		TraceID:   uuid.NewString(),
		Timestamp: now,
		Timezone:  zoneKST,
		Direction: direction,
		Request:   buildRequestLog(info, r),
		LatencyMs: time.Now().UnixMilli() - startedAt,
		TokenID:   token.ApiTokenID,
		UserID:    token.UserID,
		Origin:    buildOriginLog(r),
	}
}

func buildRequestLog(info *model.RequestInfo, r *http.Request) *logmodel.RequestLog {
	body := TruncateBody(info.Body)
	return &logmodel.RequestLog{
		Method:        info.Method,
		Scheme:        r.URL.Scheme,
		Host:          r.Host,
		Path:          r.URL.Path,
		Query:         r.URL.RawQuery,
		Headers:       SanitizeHeaders(info.Headers),
		IP:            extractIP(r),
		UserAgent:     r.Header.Get("User-Agent"),
		Body:          body.Body,
		BodyTruncated: body.Truncated,
		ContentLength: body.ContentLength,
	}
}

func buildResponseLog(resp *requester.ProxyResponse) *logmodel.ResponseLog {
	body := TruncateBody(resp.Body)
	headers := make(map[string]string)
	for k, vs := range resp.Headers {
		if len(vs) > 0 {
			headers[k] = vs[0]
		}
	}
	return &logmodel.ResponseLog{
		Status:        resp.StatusCode,
		Headers:       headers,
		Body:          body.Body,
		BodyTruncated: body.Truncated,
		ContentLength: body.ContentLength,
	}
}

func buildOriginLog(r *http.Request) *logmodel.OriginLog {
	return &logmodel.OriginLog{
		Referer:      r.Header.Get("Referer"),
		ForwardedFor: r.Header.Get("X-Forwarded-For"),
		Geo:          r.Header.Get("CF-IPCountry"),
	}
}

func buildErrorLog(err error) *logmodel.ErrorLog {
	if err == nil {
		return nil
	}
	code := 500
	if pe, ok := err.(*apperrors.ProxyError); ok {
		code = pe.StatusCode
	}
	if extErr, ok := err.(*apperrors.ExternalAPIError); ok {
		code = extErr.UpstreamStatusCode
	}
	return &logmodel.ErrorLog{
		Type:    fmt.Sprintf("%T", err),
		Message: err.Error(),
		Code:    code,
	}
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if r.RemoteAddr != "" {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		return host
	}
	return ""
}
