package service

import (
	"context"
	"fmt"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/config"
	"github.com/redis/go-redis/v9"
)

const (
	rateLimitKeyPrefix  = "api:ratelimit:"
	concurrentKeyPrefix = "api:concurrent:"
	bucketTTL           = 5 * time.Minute
	concurrentTTL       = 10 * time.Second
)

// RedisRateLimiter는 Redis String + INCR + TTL 방식으로 분 단위 요청 제한을 추적한다.
// Java의 RedisApiRateLimiterService에 대응한다.
type RedisRateLimiter struct {
	redis               *redis.Client
	thresholdMultiplier int
}

func NewRedisRateLimiter(redisClient *redis.Client, cfg config.RateLimitConfig) RateLimiter {
	return &RedisRateLimiter{
		redis:               redisClient,
		thresholdMultiplier: cfg.ThresholdMultiplier,
	}
}

func (r *RedisRateLimiter) IncrementAndGet(ctx context.Context, apiTokenID int64) (int64, error) {
	key := r.bucketKey(apiTokenID)
	count, err := r.redis.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count == 1 {
		r.redis.Expire(ctx, key, bucketTTL)
	}
	return count, nil
}

func (r *RedisRateLimiter) IncrementConcurrent(ctx context.Context, apiTokenID int64) (int64, error) {
	key := concurrentKeyPrefix + fmt.Sprint(apiTokenID)
	count, err := r.redis.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	r.redis.Expire(ctx, key, concurrentTTL)
	return count, nil
}

func (r *RedisRateLimiter) DecrementConcurrent(ctx context.Context, apiTokenID int64) (int64, error) {
	key := concurrentKeyPrefix + fmt.Sprint(apiTokenID)
	count, err := r.redis.Decr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if count < 0 {
		r.redis.Set(ctx, key, 0, concurrentTTL)
		return 0, nil
	}
	return count, nil
}

func (r *RedisRateLimiter) GetConcurrent(ctx context.Context, apiTokenID int64) (int64, error) {
	key := concurrentKeyPrefix + fmt.Sprint(apiTokenID)
	val, err := r.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func (r *RedisRateLimiter) CheckAndUpdateState(ctx context.Context, apiTokenID int64, stateSvc *TokenStateService) error {
	concurrent, err := r.GetConcurrent(ctx, apiTokenID)
	if err != nil {
		return err
	}

	key := r.bucketKey(apiTokenID)
	requestCount, err := r.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}

	threshold := concurrent * int64(r.thresholdMultiplier)
	if requestCount > threshold {
		_, err = stateSvc.TransitionState(ctx, apiTokenID)
		return err
	}
	return nil
}

// bucketKey는 현재 분(minute)을 기준으로 버킷 키를 생성한다.
func (r *RedisRateLimiter) bucketKey(apiTokenID int64) string {
	bucket := time.Now().Format("200601021504") // yyyyMMddHHmm
	return fmt.Sprintf("%s%d:%s", rateLimitKeyPrefix, apiTokenID, bucket)
}
