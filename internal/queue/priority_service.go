package queue

import (
	"context"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/config"
	"github.com/redis/go-redis/v9"
)

const (
	failureKeyPrefix = "proxy:queue:failure:"
	failureKeyTTL    = time.Hour
)

// PriorityService는 클라이언트별 실패 횟수를 Redis에 저장하고 우선순위를 계산한다.
// Java의 UserPriorityService에 대응한다.
type PriorityService struct {
	redis             *redis.Client
	basePriority      float64
	priorityIncrement float64
	maxPriority       float64
}

func NewPriorityService(redisClient *redis.Client, cfg config.QueueConfig) *PriorityService {
	return &PriorityService{
		redis:             redisClient,
		basePriority:      cfg.BasePriority,
		priorityIncrement: cfg.PriorityIncrement,
		maxPriority:       cfg.MaxPriority,
	}
}

// GetPriority는 클라이언트의 실패 횟수를 기반으로 우선순위를 반환한다.
// 실패가 많을수록 우선순위가 낮아진다.
func (s *PriorityService) GetPriority(ctx context.Context, clientID string) (float64, error) {
	if clientID == "" {
		return s.basePriority, nil
	}
	key := failureKeyPrefix + clientID
	count, err := s.redis.Get(ctx, key).Int()
	if err == redis.Nil {
		return s.basePriority, nil
	}
	if err != nil {
		return s.basePriority, err
	}
	return s.calculatePriority(count), nil
}

// IncrementFailure는 클라이언트의 실패 횟수를 증가시킨다.
func (s *PriorityService) IncrementFailure(ctx context.Context, clientID string) error {
	if clientID == "" {
		return nil
	}
	key := failureKeyPrefix + clientID
	pipe := s.redis.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, failureKeyTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// ResetFailure는 클라이언트의 실패 횟수를 초기화한다.
func (s *PriorityService) ResetFailure(ctx context.Context, clientID string) error {
	if clientID == "" {
		return nil
	}
	return s.redis.Del(ctx, failureKeyPrefix+clientID).Err()
}

func (s *PriorityService) calculatePriority(failureCount int) float64 {
	priority := s.basePriority - (float64(failureCount) * s.priorityIncrement)
	if priority < (s.basePriority - s.maxPriority) {
		return s.basePriority - s.maxPriority
	}
	return priority
}

