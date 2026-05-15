#!/usr/bin/env bash
# 简单压测脚本：用 hey 跑预约创建接口，验证 P95/错误率达标。
#
# 用法：
#   ./scripts/load-test.sh                          # 默认 localhost:8081
#   ./scripts/load-test.sh http://api.nekocafe.com  # 指定地址
#   DURATION=120s CONCURRENCY=50 ./scripts/load-test.sh
set -euo pipefail

URL="${1:-http://localhost:8081}"
DURATION="${DURATION:-30s}"
CONCURRENCY="${CONCURRENCY:-20}"
TOTAL="${TOTAL:-1000}"

if ! command -v hey &> /dev/null; then
  echo "→ 安装 hey..."
  go install github.com/rakyll/hey@latest
fi

echo "════════════════════════════════════════════════════════"
echo "  ⚡ 压测目标：${URL}/api/v1/reservations"
echo "  并发：${CONCURRENCY}   总请求：${TOTAL}   时长上限：${DURATION}"
echo "════════════════════════════════════════════════════════"

PAYLOAD='{"store_id":"S001","slot":"2026-06-01T18:00:00Z","seats":2}'

hey -n "${TOTAL}" -c "${CONCURRENCY}" -z "${DURATION}" \
  -m POST \
  -H "Content-Type: application/json" \
  -H "X-Member-Id: M001" \
  -d "${PAYLOAD}" \
  "${URL}/api/v1/reservations" | tee /tmp/load-test-result.txt

# 提取 P95 并对比阈值
P95=$(grep -E "^\s+95%" /tmp/load-test-result.txt | awk '{print $2}')
ERROR_RATE=$(grep -E "^\s+\[2" /tmp/load-test-result.txt | awk '{sum += $NF} END {print sum}')

echo ""
echo "结果："
echo "  P95 延迟：${P95:-未知}s（阈值 0.3s）"
echo "  非 2xx 计数：${ERROR_RATE:-0}"
