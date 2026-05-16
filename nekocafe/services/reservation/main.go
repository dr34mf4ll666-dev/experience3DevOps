package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nekocafe/reservation/api"
	"github.com/nekocafe/reservation/config"
	"github.com/nekocafe/reservation/observability"
	"github.com/nekocafe/reservation/repository"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(os.Stdout).With().
		Str("service", "reservation").
		Timestamp().
		Logger()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("加载配置失败")
	}

	shutdownTracer, err := observability.InitTracer(
		context.Background(),
		cfg.ServiceName,
		cfg.OTLPEndpoint,
		observability.Getenv("OTEL_DEPLOYMENT_ENV", "dev"),
	)
	if err != nil {
		log.Warn().Err(err).Msg("初始化 OTel 失败，继续以无追踪模式运行")
		shutdownTracer = func(ctx context.Context) error { return nil }
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracer(ctx)
	}()

	repo, err := repository.New(cfg.DBDsn, cfg.RedisAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("初始化数据层失败")
	}
	defer repo.Close()

	handler := api.NewRouter(repo, cfg)
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

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
