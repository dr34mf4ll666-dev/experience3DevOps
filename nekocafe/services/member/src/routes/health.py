"""健康检查路由."""
from fastapi import APIRouter
from sqlalchemy import text

from ..storage import SessionLocal

router = APIRouter(tags=["health"])


@router.get("/healthz")
async def healthz() -> dict[str, str]:
    """Liveness：进程活着就 200。"""
    return {"status": "ok"}


@router.get("/readyz")
async def readyz() -> dict[str, str]:
    """Readiness：能查数据库才算就绪。"""
    async with SessionLocal() as session:
        await session.execute(text("SELECT 1"))
    return {"status": "ready"}
