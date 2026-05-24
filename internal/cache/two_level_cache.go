package cache

import (
	"context"
	"encoding/json"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/config"
	"go.uber.org/zap"
)

// TwoLevelCache는 L1(go-cache 인메모리) + L2(Redis) 2레벨 캐시 구현체다.
// Java의 ReactiveCacheService에 대응한다.
type TwoLevelCache struct {
	local    *gocache.Cache
	redis    *redis.Client
	redisTTL time.Duration
	logger   *zap.Logger
}

func NewTwoLevelCache(cfg config.CacheConfig, redisClient *redis.Client, logger *zap.Logger) Service {
	local := gocache.New(cfg.LocalTTL, cfg.LocalTTL*2)
	return &TwoLevelCache{
		local:    local,
		redis:    redisClient,
		redisTTL: cfg.RedisTTL,
		logger:   logger,
	}
}

func (c *TwoLevelCache) GetOrFetch(ctx context.Context, key string, fetch func() (any, error), dest any) error {
	// L1 조회
	if raw, ok := c.local.Get(key); ok {
		if data, ok := raw.([]byte); ok {
			if err := json.Unmarshal(data, dest); err == nil {
				c.logger.Debug("L1 cache hit", zap.String("key", key))
				return nil
			}
		}
	}

	// L2 조회
	data, err := c.redis.Get(ctx, key).Bytes()
	if err == nil {
		if jerr := json.Unmarshal(data, dest); jerr == nil {
			c.logger.Debug("L2 cache hit", zap.String("key", key))
			c.local.Set(key, data, gocache.DefaultExpiration)
			return nil
		}
	}

	// DB에서 조회 후 캐시 저장
	result, err := fetch()
	if err != nil {
		return err
	}

	serialized, err := json.Marshal(result)
	if err != nil {
		c.logger.Warn("cache serialize failed", zap.String("key", key), zap.Error(err))
		return json.Unmarshal(mustMarshal(result), dest)
	}

	c.redis.Set(ctx, key, serialized, c.redisTTL)
	c.local.Set(key, serialized, gocache.DefaultExpiration)
	c.logger.Debug("cache set", zap.String("key", key))

	return json.Unmarshal(serialized, dest)
}

func (c *TwoLevelCache) GetOrFetchSlice(ctx context.Context, key string, fetch func() (any, error), dest any) error {
	return c.GetOrFetch(ctx, key, fetch, dest)
}

func (c *TwoLevelCache) Evict(ctx context.Context, key string) error {
	c.local.Delete(key)
	return c.redis.Del(ctx, key).Err()
}

func mustMarshal(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}
