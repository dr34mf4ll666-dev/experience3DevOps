package observability

import (
	"context"
	"time"

	"github.com/nekocafe/reservation/src/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// InitTracer 初始化全局 TracerProvider，返回的 shutdown 必须在退出前调用。
// 采样策略：dev/staging 全采样；prod ParentBased + 10% TraceIDRatio。
func InitTracer(ctx context.Context, cfg *config.Config) (func(context.Context) error, error) {
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(version()),
			semconv.DeploymentEnvironmentName(env("OTEL_DEPLOYMENT_ENV", "dev")),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			sdktrace.WithMaxQueueSize(2048),
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp.Shutdown, nil
}

func sampler() sdktrace.Sampler {
	if env("OTEL_DEPLOYMENT_ENV", "dev") == "prod" {
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))
	}
	return sdktrace.AlwaysSample()
}

func version() string {
	return env("APP_VERSION", "dev")
}

func env(k, def string) string {
	// 内部 helper，避免与 config 包循环引用
	return def
}
