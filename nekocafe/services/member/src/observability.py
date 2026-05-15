"""可观测性初始化：JSON 日志 + OTLP trace."""
from __future__ import annotations

import logging
import sys

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.trace.sampling import ParentBased, TraceIdRatioBased
from pythonjsonlogger import jsonlogger

from .config import Settings


class TraceContextFilter(logging.Filter):
    """把当前 span 的 trace_id / span_id 注入每条日志."""

    def filter(self, record: logging.LogRecord) -> bool:
        span = trace.get_current_span()
        ctx = span.get_span_context() if span else None
        if ctx and ctx.is_valid:
            record.trace_id = format(ctx.trace_id, "032x")
            record.span_id = format(ctx.span_id, "016x")
        else:
            record.trace_id = ""
            record.span_id = ""
        return True


def setup_logging(level: str = "INFO") -> None:
    """配置 JSON 结构化日志到 stdout."""
    root = logging.getLogger()
    for h in list(root.handlers):
        root.removeHandler(h)

    handler = logging.StreamHandler(sys.stdout)
    fmt = jsonlogger.JsonFormatter(
        "%(asctime)s %(levelname)s %(name)s %(message)s %(trace_id)s %(span_id)s",
        rename_fields={"asctime": "timestamp", "levelname": "level"},
    )
    handler.setFormatter(fmt)
    handler.addFilter(TraceContextFilter())
    root.addHandler(handler)
    root.setLevel(level.upper())

    # 静音 uvicorn 噪音
    for n in ("uvicorn", "uvicorn.access"):
        logging.getLogger(n).setLevel(logging.WARNING)


def setup_tracing(settings: Settings) -> None:
    """配置 OTLP exporter."""
    resource = Resource.create({
        "service.name": settings.service_name,
        "service.version": settings.version,
        "deployment.environment": settings.otel_deployment_env,
    })
    # prod 用 10% 采样，否则全采样
    sampler = (
        ParentBased(TraceIdRatioBased(0.1))
        if settings.otel_deployment_env == "prod"
        else None
    )
    provider = TracerProvider(resource=resource, sampler=sampler) if sampler else TracerProvider(resource=resource)
    exporter = OTLPSpanExporter(endpoint=settings.otel_exporter_otlp_endpoint, insecure=True)
    provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)
