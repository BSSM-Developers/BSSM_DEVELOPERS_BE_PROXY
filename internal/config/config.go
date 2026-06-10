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
	Log       LogConfig
	Queue     QueueConfig
	Stream    StreamConfig
	RateLimit RateLimitConfig
	CORS      CORSConfig
	Ntfy      NtfyConfig
}

type LogConfig struct {
	Workers int    `mapstructure:"workers"`
	Level   string `mapstructure:"level"`
}

type ServerConfig struct {
	Port        string `mapstructure:"port"`
	MaxBodyBytes int64  `mapstructure:"max_body_bytes"`
	PublicURL   string `mapstructure:"public_url"`
}

type MySQLConfig struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
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
	MaxInflight       int           `mapstructure:"max_inflight"`
	AcquireTimeout    time.Duration `mapstructure:"acquire_timeout"`
	BasePriority      float64       `mapstructure:"base_priority"`
	PriorityIncrement float64       `mapstructure:"priority_increment"`
	MaxPriority       float64       `mapstructure:"max_priority"`
}

type StreamConfig struct {
	Queue           QueueConfig   `mapstructure:"queue"`
	MaxBytesPerConn int64         `mapstructure:"max_bytes_per_conn"`
	MaxDuration     time.Duration `mapstructure:"max_duration"`
}

type RateLimitConfig struct {
	Enabled             bool          `mapstructure:"enabled"`
	ThresholdMultiplier int           `mapstructure:"threshold_multiplier"`
	WarningTTL          time.Duration `mapstructure:"warning_ttl"`
	IPRateLimitRPM      int64         `mapstructure:"ip_rate_limit_rpm"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
}

type NtfyConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	ServerURL     string `mapstructure:"server_url"`
	Topic         string `mapstructure:"topic"`
	WebhookSecret string `mapstructure:"webhook_secret"`
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
	v.SetDefault("server.max_body_bytes", 10*1024*1024) // 10 MiB

	v.SetDefault("mysql.max_open_conns", 50)
	v.SetDefault("mysql.max_idle_conns", 10)
	v.SetDefault("mysql.conn_max_lifetime", 5*time.Minute)

	v.SetDefault("cache.local_ttl", 5*time.Minute)
	v.SetDefault("cache.redis_ttl", 10*time.Minute)

	v.SetDefault("queue.max_inflight", 80)
	v.SetDefault("queue.acquire_timeout", time.Second)
	v.SetDefault("queue.base_priority", 1.0)
	v.SetDefault("queue.priority_increment", 0.2)
	v.SetDefault("queue.max_priority", 3.0)

	v.SetDefault("stream.queue.max_inflight", 10)
	v.SetDefault("stream.queue.acquire_timeout", 3*time.Second)
	v.SetDefault("stream.queue.base_priority", 1.0)
	v.SetDefault("stream.queue.priority_increment", 0.2)
	v.SetDefault("stream.queue.max_priority", 3.0)
	v.SetDefault("stream.max_bytes_per_conn", int64(100*1024*1024))
	v.SetDefault("stream.max_duration", 5*time.Minute)

	v.SetDefault("redis.pool_size", 100)

	v.SetDefault("log.workers", 4)
	v.SetDefault("log.level", "info")

	v.SetDefault("rate_limit.enabled", true)
	v.SetDefault("rate_limit.threshold_multiplier", 200)
	v.SetDefault("rate_limit.warning_ttl", 5*time.Minute)
	v.SetDefault("rate_limit.ip_rate_limit_rpm", int64(120))

	v.SetDefault("ntfy.enabled", false)
	v.SetDefault("ntfy.server_url", "https://ntfy.sh")
	v.SetDefault("ntfy.topic", "bssm-developers-blocked")
	v.SetDefault("ntfy.webhook_secret", "")
	v.SetDefault("server.public_url", "http://localhost:8080")

	v.SetDefault("cors.allowed_origins", []string{
		"https://bssmdev.com",
		"https://dev.bssm-dev.com",
		"http://localhost",
		"https://localhost",
	})
}
