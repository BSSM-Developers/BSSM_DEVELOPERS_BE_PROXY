package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/service"
	logservice "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/service"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/requester"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const tokenHeader  = "bssm-dev-token"
const secretHeader = "bssm-dev-secret"

// ProxyHandler는 모든 프록시 요청을 수신하고 Browser/Server 서비스로 위임한다.
// Java의 UseApiController에 대응한다.
type ProxyHandler struct {
	browserSvc *service.BrowserService
	serverSvc  *service.ServerService
	logger     *zap.Logger
}

func NewProxyHandler(
	browserSvc *service.BrowserService,
	serverSvc *service.ServerService,
	logger *zap.Logger,
) *ProxyHandler {
	return &ProxyHandler{browserSvc: browserSvc, serverSvc: serverSvc, logger: logger}
}

// Handle은 모든 HTTP 메서드의 프록시 요청을 처리한다.
func (h *ProxyHandler) Handle(c *gin.Context) {
	token := c.GetHeader(tokenHeader)
	if token == "" {
		c.JSON(http.StatusBadRequest, errorResponse(apperrors.ErrApiTokenNotFound))
		return
	}
	secretKey := c.GetHeader(secretHeader)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "요청 본문 읽기 실패"})
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
		c.JSON(extErr.StatusCode, errorResponse(&extErr.ProxyError))
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
