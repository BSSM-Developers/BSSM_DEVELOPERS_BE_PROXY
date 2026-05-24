package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig
	MySQL     MySQLConfig
	Redis     RedisConfig
	MongoDB   MongoDBConfig
	Cache     CacheConfig
	Queue     QueueConfig
	RateLimit RateLimitConfig
	CORS      CORSConfig
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

type MySQLConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type MongoDBConfig struct {
	URI      string `mapstructure:"uri"`
	Database string `mapstructure:"database"`
}

type CacheConfig struct {
	LocalTTL  time.Duration `mapstructure:"local_ttl"`
	RedisTTL  time.Duration `mapstructure:"redis_ttl"`
}

type QueueConfig struct {
	MaxInflight    int           `mapstructure:"max_inflight"`
	AcquireTimeout time.Duration `mapstructure:"acquire_timeout"`
	BasePriority   float64       `mapstructure:"base_priority"`
	PriorityIncrement float64    `mapstructure:"priority_increment"`
	MaxPriority    float64       `mapstructure:"max_priority"`
}

type RateLimitConfig struct {
	ThresholdMultiplier int `mapstructure:"threshold_multiplier"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

func Load() *Config {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// 환경변수 바인딩: PROXY_SERVER_PORT → server.port
	v.SetEnvPrefix("PROXY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		// 설정 파일이 없으면 환경변수와 기본값으로 동작
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic("config load error: " + err.Error())
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		panic("config unmarshal error: " + err.Error())
	}
	return cfg
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", "8080")

	v.SetDefault("cache.local_ttl", 5*time.Minute)
	v.SetDefault("cache.redis_ttl", 10*time.Minute)

	v.SetDefault("queue.max_inflight", 80)
	v.SetDefault("queue.acquire_timeout", time.Second)
	v.SetDefault("queue.base_priority", 1.0)
	v.SetDefault("queue.priority_increment", 0.2)
	v.SetDefault("queue.max_priority", 3.0)

	v.SetDefault("rate_limit.threshold_multiplier", 200)

	v.SetDefault("cors.allowed_origins", []string{
		"https://bssmdev.com",
		"https://dev.bssm-dev.com",
		"http://localhost",
		"https://localhost",
	})
}
