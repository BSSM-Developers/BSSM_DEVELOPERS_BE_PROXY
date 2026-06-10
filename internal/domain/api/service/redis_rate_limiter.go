package service

import (
	"context"
	"fmt"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/config"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"github.com/redis/go-redis/v9"
)

const (
	rateLimitKeyPrefix      = "api:ratelimit:"
	ipRateLimitKeyPrefix    = "api:ratelimit:ip:"
	concurrentKeyPrefix     = "api:concurrent:"
	peakConcurrentKeyPrefix = "api:concurrent:peak:"
	warningKeyPrefix        = "api:warning:"
	bucketTTL               = 5 * time.Minute
	concurrentTTL           = 10 * time.Second
)

// incrAndUpdatePeakScript는 concurrent 증가와 분 버킷 피크 갱신을 원자적으로 처리한다.
// async 타이밍 문제로 스냅샷 concurrent가 0이 되어도 피크값은 보존된다.
var incrAndUpdatePeakScript = redis.NewScript(`
local cur = redis.call('INCR', KEYS[1])
redis.call('EXPIRE', KEYS[1], ARGV[1])
local peak = tonumber(redis.call('GET', KEYS[2]) or 0)
if cur > peak then
    redis.call('SET', KEYS[2], cur)
    redis.call('EXPIRE', KEYS[2], ARGV[2])
end
return cur
`)

// RedisRateLimiter는 Redis String + INCR + TTL 방식으로 분 단위 요청 제한을 추적한다.
// Java의 RedisApiRateLimiterService에 대응한다.
type RedisRateLimiter struct {
	redis               *redis.Client
	thresholdMultiplier int
	warningTTL          time.Duration
	ipRateLimitRPM      int64
}

func NewRedisRateLimiter(redisClient *redis.Client, cfg config.RateLimitConfig) RateLimiter {
	return &RedisRateLimiter{
		redis:               redisClient,
		thresholdMultiplier: cfg.ThresholdMultiplier,
		warningTTL:          cfg.WarningTTL,
		ipRateLimitRPM:      cfg.IPRateLimitRPM,
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
	concurrentKey := concurrentKeyPrefix + fmt.Sprint(apiTokenID)
	peakKey := r.peakKey(apiTokenID)
	count, err := incrAndUpdatePeakScript.Run(ctx, r.redis,
		[]string{concurrentKey, peakKey},
		int(concurrentTTL.Seconds()),
		int(bucketTTL.Seconds()),
	).Int64()
	if err != nil {
		return 0, err
	}
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

func (r *RedisRateLimiter) GetPeakConcurrent(ctx context.Context, apiTokenID int64) (int64, error) {
	key := r.peakKey(apiTokenID)
	val, err := r.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func (r *RedisRateLimiter) IsWarningActive(ctx context.Context, apiTokenID int64) (bool, error) {
	key := warningKeyPrefix + fmt.Sprint(apiTokenID)
	err := r.redis.Get(ctx, key).Err()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *RedisRateLimiter) CheckClientIP(ctx context.Context, apiTokenID int64, clientIP string) error {
	if r.ipRateLimitRPM <= 0 {
		return nil
	}
	key := fmt.Sprintf("%s%d:%s:%s", ipRateLimitKeyPrefix, apiTokenID, clientIP, time.Now().Format("200601021504"))
	count, err := r.redis.Incr(ctx, key).Result()
	if err != nil {
		return err
	}
	if count == 1 {
		r.redis.Expire(ctx, key, bucketTTL)
	}
	if count > r.ipRateLimitRPM {
		return apperrors.ErrTooManyRequests
	}
	return nil
}

func (r *RedisRateLimiter) CheckAndUpdateState(ctx context.Context, apiTokenID int64, stateSvc *TokenStateService) error {
	peak, err := r.GetPeakConcurrent(ctx, apiTokenID)
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

	threshold := peak * int64(r.thresholdMultiplier)
	if requestCount <= threshold {
		return nil
	}

	newState, err := stateSvc.TransitionState(ctx, apiTokenID, requestCount)
	if err != nil {
		return err
	}

	if newState == model.StateWarning {
		warnKey := warningKeyPrefix + fmt.Sprint(apiTokenID)
		r.redis.Set(ctx, warnKey, 1, r.warningTTL)
	}
	return nil
}

func (r *RedisRateLimiter) peakKey(apiTokenID int64) string {
	bucket := time.Now().Format("200601021504")
	return fmt.Sprintf("%s%d:%s", peakConcurrentKeyPrefix, apiTokenID, bucket)
}

// bucketKey는 현재 분(minute)을 기준으로 버킷 키를 생성한다.
func (r *RedisRateLimiter) bucketKey(apiTokenID int64) string {
	bucket := time.Now().Format("200601021504") // yyyyMMddHHmm
	return fmt.Sprintf("%s%d:%s", rateLimitKeyPrefix, apiTokenID, bucket)
}
