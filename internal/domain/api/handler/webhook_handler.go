package handler

import (
	"crypto/subtle"
	"net/http"
	"strconv"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const webhookSecretHeader = "X-Webhook-Secret"

// WebhookHandler는 ntfy 액션 버튼 등 외부 웹훅 요청을 처리한다.
type WebhookHandler struct {
	stateSvc *service.TokenStateService
	secret   string
	logger   *zap.Logger
}

func NewWebhookHandler(stateSvc *service.TokenStateService, secret string, logger *zap.Logger) *WebhookHandler {
	return &WebhookHandler{stateSvc: stateSvc, secret: secret, logger: logger}
}

// BlockToken은 관리자 확인 후 토큰을 즉시 차단한다.
// POST /webhook/api/token/:tokenId/block
func (h *WebhookHandler) BlockToken(c *gin.Context) {
	incoming := c.GetHeader(webhookSecretHeader)
	if subtle.ConstantTimeCompare([]byte(incoming), []byte(h.secret)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	tokenIDStr := c.Param("tokenId")
	tokenID, err := strconv.ParseInt(tokenIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid tokenId"})
		return
	}

	if err := h.stateSvc.ForceBlock(c.Request.Context(), tokenID); err != nil {
		h.logger.Error("웹훅 토큰 차단 실패", zap.Int64("tokenId", tokenID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"message": "차단 처리 실패"})
		return
	}

	h.logger.Info("웹훅 토큰 차단 완료", zap.Int64("tokenId", tokenID))
	c.JSON(http.StatusOK, gin.H{"message": "차단 완료"})
}
