// Package main 是预约服务入口。
//
// 预约服务负责门店时段、座位预订、与会员服务联动校验权益。
// 所有 HTTP 接口都通过 otelgin 中间件自动注入 traceId，
// 并以结构化 JSON 写入 stdout（由 Loki 抓取）。
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nekocafe/reservation/src/api"
	"github.com/nekocafe/reservation/src/config"
	"github.com/nekocafe/reservation/src/observability"
	"github.com/nekocafe/reservation/src/repository"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// --- 日志：结构化 JSON 写 stdout ---
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(os.Stdout).With().
		Str("service", "reservation").
		Timestamp().
		Logger()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("加载配置失败")
	}

	// --- OpenTelemetry 初始化 ---
	shutdownTracer, err := observability.InitTracer(context.Background(), cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("初始化 OTel 失败")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracer(ctx)
	}()

	// --- 数据层 ---
	repo, err := repository.New(cfg.DBDsn, cfg.RedisAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("初始化数据层失败")
	}
	defer repo.Close()

	// --- HTTP 路由 ---
	handler := api.NewRouter(repo, cfg)
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 优雅退出
	go func() {
		log.Info().Str("addr", srv.Addr).Msg("Reservation 服务启动")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("HTTP 服务异常退出")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("收到退出信号，开始优雅关闭")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("优雅关闭失败")
	}
}
