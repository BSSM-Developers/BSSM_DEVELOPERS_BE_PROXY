package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/cache"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/config"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/handler"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/query"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/repository"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/service"
	logrepository "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/repository"
	logservice "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/service"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/middleware"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/queue"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/requester"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/validator"

	corsmw "github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()
	logger := newLogger()
	defer logger.Sync()

	// --- 인프라 초기화 ---
	db := newMySQL(cfg.MySQL, logger)
	redisClient := newRedis(cfg.Redis)
	mongoDB := newMongoDB(cfg.MongoDB, logger)

	// --- 캐시 ---
	cacheService := cache.NewTwoLevelCache(cfg.Cache, redisClient, logger)

	// --- 레포지토리 ---
	tokenRepo := repository.NewTokenRepository(db)
	usageRepo := repository.NewUsageRepository(db)
	domainRepo := repository.NewDomainRepository(db)
	logRepo := logrepository.NewLogRepository(mongoDB)

	// --- 쿼리 서비스 ---
	tokenQuery := query.NewTokenQueryService(tokenRepo, cacheService)
	usageQuery := query.NewUsageQueryService(usageRepo, cacheService)
	domainQuery := query.NewDomainQueryService(domainRepo, cacheService)

	// --- 로그 발행자 ---
	logPublisher := logservice.NewLogPublisher(logRepo, logger, cfg.Log.Workers)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logPublisher.Start(ctx)

	// --- 도메인 서비스 ---
	domainValidator := validator.New()
	httpRequester := requester.NewHTTPRequester(domainValidator)
	rateLimiter := service.NewRedisRateLimiter(redisClient, cfg.RateLimit)
	tokenStateSvc := service.NewTokenStateService(tokenRepo, logger)

	pipeline := service.NewPipeline(
		tokenQuery, usageQuery, httpRequester,
		logPublisher, rateLimiter, tokenStateSvc, logger,
	)
	browserSvc := service.NewBrowserService(domainQuery, pipeline)
	serverSvc := service.NewServerService(pipeline)
	healthSvc := service.NewHealthService(httpRequester, domainValidator)

	// --- 큐 ---
	requestQueue := queue.NewHRNQueue(cfg.Queue)
	prioritySvc := queue.NewPriorityService(redisClient, cfg.Queue)

	// --- 핸들러 ---
	proxyHandler := handler.NewProxyHandler(browserSvc, serverSvc, logger)
	healthHandler := handler.NewHealthHandler(healthSvc)

	// --- 미들웨어 ---
	queueMW := middleware.NewQueueMiddleware(requestQueue, prioritySvc, logger)
	errorMW := middleware.NewErrorMiddleware(logger)

	// --- Gin 라우터 ---
	r := newRouter(cfg, queueMW, errorMW, proxyHandler, healthHandler)

	// --- 서버 시작 및 Graceful Shutdown ---
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  35 * time.Second,
		WriteTimeout: 35 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("프록시 서버 시작", zap.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("서버 시작 실패", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("서버 종료 중...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("서버 종료 오류", zap.Error(err))
	}
	logger.Info("서버 종료 완료")
}

func newRouter(
	cfg *config.Config,
	queueMW *middleware.QueueMiddleware,
	errorMW *middleware.ErrorMiddleware,
	proxyHandler *handler.ProxyHandler,
	healthHandler *handler.HealthHandler,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(newCORSMiddleware(cfg.CORS))
	r.Use(errorMW.Handle)
	r.Use(queueMW.Handle)

	r.POST("/healthy", healthHandler.Check)

	// 명시적 경로 우선 등록 후 catch-all 등록
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		r.Handle(method, "/proxy-browser/*path", proxyHandler.Handle)
		r.Handle(method, "/proxy-server/*path", proxyHandler.Handle)
	}
	r.NoRoute(proxyHandler.Handle)

	return r
}

func newCORSMiddleware(cfg config.CORSConfig) gin.HandlerFunc {
	corsConfig := corsmw.Config{
		AllowOriginFunc: func(origin string) bool {
			for _, allowed := range cfg.AllowedOrigins {
				if strings.EqualFold(origin, allowed) {
					return true
				}
			}
			// localhost:* 패턴 허용
			if strings.HasPrefix(origin, "http://localhost:") ||
				strings.HasPrefix(origin, "https://localhost:") {
				return true
			}
			return false
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	return corsmw.New(corsConfig)
}

func newLogger() *zap.Logger {
	logger, _ := zap.NewProduction()
	return logger
}

func newMySQL(cfg config.MySQLConfig, logger *zap.Logger) *gorm.DB {
	db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		logger.Fatal("MySQL 연결 실패", zap.Error(err))
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db
}

func newRedis(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
}

func newMongoDB(cfg config.MongoDBConfig, logger *zap.Logger) *mongo.Database {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.URI))
	if err != nil {
		logger.Fatal("MongoDB 연결 실패", zap.Error(err))
	}
	if err := client.Ping(ctx, nil); err != nil {
		logger.Fatal("MongoDB ping 실패", zap.Error(err))
	}
	return client.Database(cfg.Database)
}
