"""会员服务入口.

提供会员档案、月度预约额度、等级权益查询。
所有接口经 FastAPI 的 OpenTelemetry 自动埋点；
日志以 JSON 写入 stdout，含 trace_id / span_id 字段。
"""
from __future__ import annotations

import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from prometheus_client import make_asgi_app

from .config import settings
from .observability import setup_logging, setup_tracing
from .routes import health, members
from .storage import init_db


@asynccontextmanager
async def lifespan(app: FastAPI):  # noqa: ARG001
    """应用生命周期：启动期初始化数据库、退出期清理."""
    setup_logging(settings.log_level)
    setup_tracing(settings)
    await init_db()
    logging.getLogger(__name__).info("member service started", extra={"version": settings.version})
    yield
    logging.getLogger(__name__).info("member service stopping")


def create_app() -> FastAPI:
    app = FastAPI(
        title="NekoCafé Member Service",
        version=settings.version,
        lifespan=lifespan,
        # 关闭默认 /docs 到 prod；这里演示用全开。
    )
    app.include_router(health.router)
    app.include_router(members.router, prefix="/api/v1")

    # Prometheus
    app.mount("/metrics", make_asgi_app())

    # 自动给 FastAPI 路由埋点（含 traceparent 透传）
    FastAPIInstrumentor.instrument_app(app, excluded_urls="healthz,readyz,metrics")
    return app


app = create_app()


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8080, log_config=None)  # noqa: S104
