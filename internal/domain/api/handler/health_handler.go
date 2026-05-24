package handler

import (
	"io"
	"net/http"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/service"
	"github.com/gin-gonic/gin"
)

// HealthHandler는 외부 API 상태 확인 엔드포인트를 담당한다.
// Java의 HealthCheckApiController에 대응한다.
type HealthHandler struct {
	svc *service.HealthService
}

func NewHealthHandler(svc *service.HealthService) *HealthHandler {
	return &HealthHandler{svc: svc}
}

// Check는 POST /healthy 요청을 처리한다.
// 쿼리 파라미터: endpoint, method, domain
func (h *HealthHandler) Check(c *gin.Context) {
	endpoint := c.Query("endpoint")
	method := c.Query("method")
	domain := c.Query("domain")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "요청 본문 읽기 실패"})
		return
	}

	result, err := h.svc.Check(c.Request.Context(), endpoint, method, domain, c.Request, body)
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "health check ok",
		"data":    result,
	})
}
