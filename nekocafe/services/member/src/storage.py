"""数据访问层：SQLAlchemy async + Postgres."""
from __future__ import annotations

from datetime import datetime

from sqlalchemy import Column, DateTime, Integer, String
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine
from sqlalchemy.orm import DeclarativeBase

from .config import settings


class Base(DeclarativeBase):
    pass


class Member(Base):
    """会员实体."""

    __tablename__ = "members"
    id = Column(String, primary_key=True)
    name = Column(String, nullable=False)
    level = Column(String, nullable=False, default="basic")  # basic/gold/diamond
    monthly_quota = Column(Integer, nullable=False, default=4)
    used = Column(Integer, nullable=False, default=0)
    created_at = Column(DateTime, default=datetime.utcnow)


dsn = settings.db_dsn.replace("postgresql://", "postgresql+asyncpg://", 1)
engine = create_async_engine(dsn, echo=False, pool_pre_ping=True)
SessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)


async def init_db() -> None:
    """启动期建表 + seed 演示数据."""
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

    async with SessionLocal() as session:
        existing = await session.get(Member, "M001")
        if existing is None:
            session.add_all([
                Member(id="M001", name="Alice", level="gold", monthly_quota=10, used=2),
                Member(id="M002", name="Bob", level="basic", monthly_quota=4, used=4),
                Member(id="M003", name="Carol", level="diamond", monthly_quota=99, used=5),
            ])
            await session.commit()
