package middleware

import (
	"net/http"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/queue"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// QueueMiddleware는 요청 큐를 통해 동시 처리 수를 제한하는 미들웨어다.
// bssm-dev-token 헤더가 없는 요청(헬스체크 등)은 큐를 통과하지 않는다.
// Java의 ProxyRequestQueueFilter에 대응한다.
type QueueMiddleware struct {
	requestQueue    queue.RequestQueue
	priorityService *queue.PriorityService
	logger          *zap.Logger
}

func NewQueueMiddleware(
	requestQueue queue.RequestQueue,
	priorityService *queue.PriorityService,
	logger *zap.Logger,
) *QueueMiddleware {
	return &QueueMiddleware{
		requestQueue:    requestQueue,
		priorityService: priorityService,
		logger:          logger,
	}
}

func (m *QueueMiddleware) Handle(c *gin.Context) {
	clientID := c.GetHeader("bssm-dev-token")
	if clientID == "" {
		c.Next()
		return
	}

	priority, err := m.priorityService.GetPriority(c.Request.Context(), clientID)
	if err != nil {
		m.logger.Warn("우선순위 조회 실패, 기본값 사용", zap.Error(err))
		priority = 1.0
	}

	acquired, err := m.requestQueue.Acquire(c.Request.Context(), clientID, priority)
	if err != nil || !acquired {
		if err == nil {
			// 타임아웃: 실패 횟수 증가 (비동기)
			go m.priorityService.IncrementFailure(c.Request.Context(), clientID)
		}
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"statusCode": apperrors.ErrTooManyRequests.StatusCode,
			"message":    apperrors.ErrTooManyRequests.Message,
		})
		return
	}

	defer m.requestQueue.Release()
	c.Next()
}
