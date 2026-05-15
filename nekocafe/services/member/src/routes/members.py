"""会员业务路由."""
from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from sqlalchemy import select

from ..storage import Member, SessionLocal

router = APIRouter(tags=["members"])


class MemberDTO(BaseModel):
    """会员 DTO."""

    id: str
    name: str
    level: str
    monthly_quota: int
    used: int


class QuotaDTO(BaseModel):
    """月度额度 DTO."""

    member_id: str
    remaining: int
    level: str


@router.get("/members/{member_id}", response_model=MemberDTO)
async def get_member(member_id: str) -> MemberDTO:
    """读取会员档案."""
    async with SessionLocal() as session:
        m = await session.get(Member, member_id)
        if m is None:
            raise HTTPException(404, "会员不存在")
        return MemberDTO(
            id=m.id, name=m.name, level=m.level,
            monthly_quota=m.monthly_quota, used=m.used,
        )


@router.get("/members/{member_id}/quota", response_model=QuotaDTO)
async def get_quota(member_id: str) -> QuotaDTO:
    """查月度剩余额度.

    预约服务调用此接口前置校验。
    """
    async with SessionLocal() as session:
        m = await session.get(Member, member_id)
        if m is None:
            raise HTTPException(404, "会员不存在")
        remaining = max(0, m.monthly_quota - m.used)
        return QuotaDTO(member_id=m.id, remaining=remaining, level=m.level)


@router.get("/members", response_model=list[MemberDTO])
async def list_members(limit: int = 20) -> list[MemberDTO]:
    """会员列表（分页）."""
    async with SessionLocal() as session:
        result = await session.execute(select(Member).limit(limit))
        return [
            MemberDTO(
                id=m.id, name=m.name, level=m.level,
                monthly_quota=m.monthly_quota, used=m.used,
            )
            for m in result.scalars().all()
        ]
