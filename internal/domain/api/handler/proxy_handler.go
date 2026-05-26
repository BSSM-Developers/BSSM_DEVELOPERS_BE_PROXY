package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/config"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/service"
	logservice "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/service"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/requester"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const tokenHeader  = "bssm-dev-token"
const secretHeader = "bssm-dev-secret"

// ProxyHandler는 모든 프록시 요청을 수신하고 Browser/Server 서비스로 위임한다.
// Accept: text/event-stream 요청은 스트리밍 경로로 분기된다.
// Java의 UseApiController에 대응한다.
type ProxyHandler struct {
	browserSvc       *service.BrowserService
	serverSvc        *service.ServerService
	browserStreamSvc *service.BrowserStreamService
	serverStreamSvc  *service.ServerStreamService
	logger           *zap.Logger
	maxBodyBytes     int64
	streamCfg        config.StreamConfig
}

func NewProxyHandler(
	browserSvc *service.BrowserService,
	serverSvc *service.ServerService,
	browserStreamSvc *service.BrowserStreamService,
	serverStreamSvc *service.ServerStreamService,
	logger *zap.Logger,
	serverCfg config.ServerConfig,
	streamCfg config.StreamConfig,
) *ProxyHandler {
	return &ProxyHandler{
		browserSvc:       browserSvc,
		serverSvc:        serverSvc,
		browserStreamSvc: browserStreamSvc,
		serverStreamSvc:  serverStreamSvc,
		logger:           logger,
		maxBodyBytes:     serverCfg.MaxBodyBytes,
		streamCfg:        streamCfg,
	}
}

// Handle은 모든 HTTP 메서드의 프록시 요청을 처리한다.
func (h *ProxyHandler) Handle(c *gin.Context) {
	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
		h.handleStream(c)
		return
	}
	h.handleNormal(c)
}

func (h *ProxyHandler) handleNormal(c *gin.Context) {
	token := c.GetHeader(tokenHeader)
	if token == "" {
		c.JSON(http.StatusBadRequest, errorResponse(apperrors.ErrApiTokenNotFound))
		return
	}
	secretKey := c.GetHeader(secretHeader)

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, h.maxBodyBytes+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "요청 본문 읽기 실패"})
		return
	}
	if int64(len(body)) > h.maxBodyBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"statusCode": http.StatusRequestEntityTooLarge,
			"message":    "요청 바디가 허용 크기를 초과했습니다",
		})
		return
	}

	h.logRequest(c.Request, token, secretKey)

	var resp *requester.ProxyResponse
	if secretKey != "" {
		resp, err = h.serverSvc.Handle(c.Request.Context(), secretKey, token, c.Request, body)
	} else {
		resp, err = h.browserSvc.Handle(c.Request.Context(), token, c.Request, body)
	}

	if err != nil {
		writeError(c, err)
		return
	}

	writeProxyResponse(c, resp)
}

func (h *ProxyHandler) handleStream(c *gin.Context) {
	token := c.GetHeader(tokenHeader)
	if token == "" {
		c.JSON(http.StatusBadRequest, errorResponse(apperrors.ErrApiTokenNotFound))
		return
	}
	secretKey := c.GetHeader(secretHeader)

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, h.maxBodyBytes+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "요청 본문 읽기 실패"})
		return
	}
	if int64(len(body)) > h.maxBodyBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"statusCode": http.StatusRequestEntityTooLarge,
			"message":    "요청 바디가 허용 크기를 초과했습니다",
		})
		return
	}

	// 최대 스트리밍 연결 시간 제한
	streamCtx, cancel := context.WithTimeout(c.Request.Context(), h.streamCfg.MaxDuration)
	defer cancel()

	var session *service.StreamSession
	if secretKey != "" {
		session, err = h.serverStreamSvc.Handle(streamCtx, secretKey, token, c.Request, body)
	} else {
		session, err = h.browserStreamSvc.Handle(streamCtx, token, c.Request, body)
	}

	if err != nil {
		writeError(c, err)
		return
	}
	defer session.Response.Body.Close()

	// 응답 헤더 전송
	for key, values := range session.Response.Headers {
		for _, v := range values {
			c.Header(key, v)
		}
	}
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no") // nginx 프록시 버퍼링 비활성화
	c.Status(session.Response.StatusCode)

	// 서버 수준 WriteTimeout을 이 요청에 한해 비활성화 (Go 1.20+)
	rc := http.NewResponseController(c.Writer)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		h.logger.Warn("스트리밍 write deadline 해제 실패", zap.Error(err))
	}

	// 전송량 상한선 적용 및 SSE 이벤트 즉시 플러시
	limited := &io.LimitedReader{R: session.Response.Body, N: h.streamCfg.MaxBytesPerConn}
	fw := &flushWriter{w: c.Writer, flusher: c.Writer}
	_, copyErr := io.Copy(fw, limited)

	session.Complete(fw.count, copyErr)
}

// flushWriter는 각 Write 후 즉시 플러시해 SSE 이벤트가 클라이언트에 즉시 전달되도록 한다.
type flushWriter struct {
	w       io.Writer
	flusher http.Flusher
	count   int64
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	fw.count += int64(n)
	if err == nil {
		fw.flusher.Flush()
	}
	return n, err
}

func writeProxyResponse(c *gin.Context, resp *requester.ProxyResponse) {
	for key, values := range resp.Headers {
		for _, v := range values {
			c.Header(key, v)
		}
	}
	contentType := resp.Headers.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Data(resp.StatusCode, contentType, resp.Body)
}

func writeError(c *gin.Context, err error) {
	var proxyErr *apperrors.ProxyError
	if errors.As(err, &proxyErr) {
		c.JSON(proxyErr.StatusCode, errorResponse(proxyErr))
		return
	}
	var extErr *apperrors.ExternalAPIError
	if errors.As(err, &extErr) {
		c.JSON(extErr.UpstreamStatusCode, gin.H{
			"statusCode":    extErr.UpstreamStatusCode,
			"message":       extErr.Message,
			"apiStatusCode": extErr.UpstreamStatusCode,
			"apiBody":       extErr.UpstreamBody,
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"statusCode": 500, "message": "Internal server error"})
}

func (h *ProxyHandler) logRequest(r *http.Request, token, secretKey string) {
	if !h.logger.Core().Enabled(zap.DebugLevel) {
		return
	}
	pathWithQuery := r.URL.Path
	if r.URL.RawQuery != "" {
		pathWithQuery += "?" + r.URL.RawQuery
	}
	h.logger.Debug("proxy request",
		zap.String("type", map[bool]string{true: "Server", false: "Browser"}[secretKey != ""]),
		zap.String("method", r.Method),
		zap.String("path", pathWithQuery),
		zap.String("token", logservice.MaskToken(token)),
		zap.String("remoteAddr", r.RemoteAddr),
		zap.String("userAgent", r.Header.Get("User-Agent")),
	)
}

func errorResponse(e *apperrors.ProxyError) gin.H {
	return gin.H{
		"statusCode": e.StatusCode,
		"message":    e.Message,
	}
}
