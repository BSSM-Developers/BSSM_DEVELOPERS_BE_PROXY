package middleware

import (
	"errors"
	"net/http"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
	logservice "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ErrorMiddleware는 핸들러에서 발생한 panic 및 에러를 통일된 JSON 형식으로 변환한다.
// Java의 ProxyWebExceptionHandler에 대응한다.
type ErrorMiddleware struct {
	logger *zap.Logger
}

func NewErrorMiddleware(logger *zap.Logger) *ErrorMiddleware {
	return &ErrorMiddleware{logger: logger}
}

func (m *ErrorMiddleware) Handle(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("panic recovered",
				zap.Any("error", r),
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
			)
			if !c.Writer.Written() {
				c.JSON(http.StatusInternalServerError, gin.H{
					"statusCode": 500,
					"message":    "Internal server error",
				})
			}
		}
	}()

	c.Next()

	// Gin 에러가 있으면 처리
	if len(c.Errors) > 0 && !c.Writer.Written() {
		err := c.Errors.Last().Err

		var proxyErr *apperrors.ProxyError
		if errors.As(err, &proxyErr) {
			m.logProxyError(c, proxyErr)
			c.JSON(proxyErr.StatusCode, gin.H{
				"statusCode": proxyErr.StatusCode,
				"message":    proxyErr.Message,
			})
			return
		}

		m.logger.Error("unhandled error",
			zap.Error(err),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("token", logservice.MaskToken(c.GetHeader("bssm-dev-token"))),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"statusCode": 500,
			"message":    "Internal server error",
		})
	}
}

func (m *ErrorMiddleware) logProxyError(c *gin.Context, e *apperrors.ProxyError) {
	m.logger.Warn("proxy error",
		zap.String("code", e.Code),
		zap.Int("status", e.StatusCode),
		zap.String("message", e.Message),
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("origin", c.Request.Header.Get("Origin")),
		zap.String("token", logservice.MaskToken(c.GetHeader("bssm-dev-token"))),
	)
}
