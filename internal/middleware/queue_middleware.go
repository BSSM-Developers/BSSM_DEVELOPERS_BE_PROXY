package middleware

import (
	"net/http"
	"strings"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/queue"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// QueueMiddleware는 요청 큐를 통해 동시 처리 수를 제한하는 미들웨어다.
// bssm-dev-token 헤더가 없는 요청(헬스체크 등)은 큐를 통과하지 않는다.
// Accept: text/event-stream 요청은 streamQueue를 사용한다 (슬롯 분리).
// Java의 ProxyRequestQueueFilter에 대응한다.
type QueueMiddleware struct {
	normalQueue     queue.RequestQueue
	streamQueue     queue.RequestQueue
	priorityService *queue.PriorityService
	logger          *zap.Logger
}

func NewQueueMiddleware(
	normalQueue queue.RequestQueue,
	streamQueue queue.RequestQueue,
	priorityService *queue.PriorityService,
	logger *zap.Logger,
) *QueueMiddleware {
	return &QueueMiddleware{
		normalQueue:     normalQueue,
		streamQueue:     streamQueue,
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

	q := m.selectQueue(c)

	priority, err := m.priorityService.GetPriority(c.Request.Context(), clientID)
	if err != nil {
		m.logger.Warn("우선순위 조회 실패, 기본값 사용", zap.Error(err))
		priority = 1.0
	}

	acquired, err := q.Acquire(c.Request.Context(), clientID, priority)
	if err != nil || !acquired {
		if err == nil {
			go m.priorityService.IncrementFailure(c.Request.Context(), clientID)
		}
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"statusCode": apperrors.ErrTooManyRequests.StatusCode,
			"message":    apperrors.ErrTooManyRequests.Message,
		})
		return
	}

	defer q.Release()
	c.Next()
}

func (m *QueueMiddleware) selectQueue(c *gin.Context) queue.RequestQueue {
	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
		return m.streamQueue
	}
	return m.normalQueue
}
