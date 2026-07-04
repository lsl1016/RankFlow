package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	_ "rankflow/docs" // 由 swag 生成的 OpenAPI 文档，供 /swagger 路由加载

	"rankflow/internal/api/handler"
	"rankflow/internal/api/router"
	"rankflow/internal/config"
	"rankflow/internal/observability"
	"rankflow/internal/queue"
	"rankflow/internal/service"
	"rankflow/internal/store/mysql"
	"rankflow/internal/store/redis"
)

// @title			RankFlow 通用榜单服务 API
// @version		1.0
// @description	可配置、可复用的榜单基础服务：榜单配置、分数更新、排名查询。
// @BasePath		/api
func main() {
	log, err := observability.NewLogger()
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("load config failed", zap.Error(err))
	}

	myStore, err := mysql.New(cfg.MySQLDSN)
	if err != nil {
		log.Fatal("connect mysql failed", zap.Error(err))
	}
	if err := myStore.AutoMigrate(); err != nil {
		log.Fatal("auto migrate failed", zap.Error(err))
	}

	rdStore, err := redis.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatal("connect redis failed", zap.Error(err))
	}

	metrics := observability.NewMetrics()
	svc := service.New(myStore, rdStore, log, metrics)
	h := handler.New(svc, metrics)
	engine := router.New(h, log)

	// Async persist worker.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker := queue.NewWorker(rdStore, myStore, log)
	worker.Run(ctx, cfg.PersistWorkers)

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: engine}
	go func() {
		log.Info("rankflow api listening", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("http server error", zap.Error(err))
		}
	}()

	// Graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Info("shutting down")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}
}
