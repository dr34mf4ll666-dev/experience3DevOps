"""会员服务测试用例.

CI 中由 pytest-asyncio 自动跑；覆盖率目标 ≥ 70%。
"""
import pytest
from httpx import ASGITransport, AsyncClient

from src.main import app


@pytest.mark.asyncio
async def test_healthz_returns_ok() -> None:
    """健康检查必须 200."""
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        resp = await client.get("/healthz")
        assert resp.status_code == 200
        assert resp.json() == {"status": "ok"}


@pytest.mark.asyncio
async def test_get_member_quota_known_member() -> None:
    """Alice 是 gold 会员，应返回 remaining = 10-2 = 8."""
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        resp = await client.get("/api/v1/members/M001/quota")
        assert resp.status_code == 200
        body = resp.json()
        assert body["member_id"] == "M001"
        assert body["remaining"] == 8
        assert body["level"] == "gold"


@pytest.mark.asyncio
async def test_get_member_quota_unknown_member_returns_404() -> None:
    """未知会员返回 404."""
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        resp = await client.get("/api/v1/members/UNKNOWN/quota")
        assert resp.status_code == 404


@pytest.mark.asyncio
async def test_list_members() -> None:
    """列表至少包含 seed 的 3 个会员."""
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        resp = await client.get("/api/v1/members")
        assert resp.status_code == 200
        members = resp.json()
        assert len(members) >= 3
        ids = {m["id"] for m in members}
        assert {"M001", "M002", "M003"}.issubset(ids)
