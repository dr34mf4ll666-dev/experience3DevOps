// Package api HTTP 接口层：路由、参数校验、错误映射。
// 所有路由经 otelgin → traceId 自动注入 → response header X-Trace-Id 透出。
package api

import (
	"net/http"
	"time"

	"github.com/nekocafe/reservation/src/config"
	"github.com/nekocafe/reservation/src/repository"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/trace"
)

// NewRouter 构造 *gin.Engine：包含可观测中间件、健康/指标/业务路由。
func NewRouter(repo *repository.Repo, cfg *config.Config) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(otelgin.Middleware(cfg.ServiceName))
	r.Use(traceHeaderMiddleware())
	r.Use(structuredLogMiddleware())

	r.GET("/healthz", func(c *gin.Context) {
		if err := repo.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "err": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := r.Group("/api/v1")
	{
		h := &Handlers{repo: repo, cfg: cfg}
		v1.POST("/reservations", h.Create)
		v1.GET("/reservations/:id", h.Get)
	}

	return r
}

// traceHeaderMiddleware 把当前 span 的 traceId 写到 response header，方便客户端排障。
func traceHeaderMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		span := trace.SpanFromContext(c.Request.Context())
		if sc := span.SpanContext(); sc.IsValid() {
			c.Writer.Header().Set("X-Trace-Id", sc.TraceID().String())
		}
		c.Next()
	}
}

// structuredLogMiddleware 用 zerolog 打结构化 JSON 日志，含 trace_id。
func structuredLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		span := trace.SpanFromContext(c.Request.Context())
		evt := log.Info()
		if sc := span.SpanContext(); sc.IsValid() {
			evt = evt.Str("trace_id", sc.TraceID().String()).
				Str("span_id", sc.SpanID().String())
		}
		evt.Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency_ms", time.Since(start)).
			Str("client_ip", c.ClientIP()).
			Msg("http_request")
	}
}
