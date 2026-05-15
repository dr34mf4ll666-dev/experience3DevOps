#!/usr/bin/env bash
# 一键回滚脚本
#
# 用法：
#   ./scripts/rollback.sh <env> [revision]
#   ./scripts/rollback.sh prod              # 回滚到 PREVIOUS
#   ./scripts/rollback.sh prod 7            # 回滚到 helm release rev 7
#
# 逻辑：
#   1) 如果是 Argo Rollouts 部署 → 调 kubectl-argo-rollouts undo
#   2) 否则 → helm rollback
#   3) 等待健康 → 通知 Slack/PagerDuty
#
# 退出码：0 成功 / 1 参数错误 / 2 回滚失败 / 3 回滚后未恢复健康
set -euo pipefail

ENV="${1:-}"
REV="${2:-PREVIOUS}"
SERVICES=(reservation member)
TIMEOUT="${TIMEOUT:-5m}"

if [[ -z "$ENV" ]]; then
  echo "❌ 用法：$0 <dev|staging|prod> [revision]" >&2
  exit 1
fi

NS="nekocafe-${ENV}"
RELEASE="nekocafe-${ENV}"

echo "════════════════════════════════════════════════════════"
echo "  🔙 NekoCafé 回滚"
echo "  环境：${ENV}   命名空间：${NS}"
echo "  目标版本：${REV}"
echo "════════════════════════════════════════════════════════"

# 1. 探测：是 Argo Rollouts 还是普通 Deployment？
if kubectl get rollout -n "${NS}" -l app.kubernetes.io/instance="${RELEASE}" 2>/dev/null | grep -q nekocafe; then
  STRATEGY="argo-rollouts"
else
  STRATEGY="helm"
fi
echo "→ 检测到策略：${STRATEGY}"

# 2. 记录回滚开始时间（用于 MTTR 计算）
START_TS=$(date -u +%FT%TZ)
echo "{\"event\":\"rollback_started\",\"env\":\"${ENV}\",\"at\":\"${START_TS}\"}" \
  >> /tmp/nekocafe-rollback.log

# 3. 执行回滚
case "$STRATEGY" in
  argo-rollouts)
    for svc in "${SERVICES[@]}"; do
      echo "→ 中止并撤销 ${svc} 的当前 Rollout..."
      kubectl argo rollouts abort "${RELEASE}-${svc}" -n "${NS}" || true
      kubectl argo rollouts undo "${RELEASE}-${svc}" -n "${NS}"
    done
    ;;
  helm)
    if [[ "$REV" == "PREVIOUS" ]]; then
      echo "→ helm rollback 到上一版本..."
      helm rollback "${RELEASE}" 0 -n "${NS}" --wait --timeout "${TIMEOUT}"
    else
      helm rollback "${RELEASE}" "${REV}" -n "${NS}" --wait --timeout "${TIMEOUT}"
    fi
    ;;
esac

# 4. 健康验证
echo "→ 等待 Pod 就绪..."
for svc in "${SERVICES[@]}"; do
  if ! kubectl rollout status deployment/"${RELEASE}-${svc}" -n "${NS}" --timeout="${TIMEOUT}" \
       && ! kubectl argo rollouts status "${RELEASE}-${svc}" -n "${NS}" --timeout="${TIMEOUT}"; then
    echo "❌ ${svc} 回滚后未就绪" >&2
    exit 3
  fi
done

END_TS=$(date -u +%FT%TZ)
ELAPSED=$(( $(date -d "$END_TS" +%s) - $(date -d "$START_TS" +%s) ))

echo ""
echo "✅ 回滚完成，用时 ${ELAPSED}s"
echo "════════════════════════════════════════════════════════"

# 5. 通知（Slack/钉钉，URL 从环境变量取）
if [[ -n "${SLACK_WEBHOOK:-}" ]]; then
  curl -sS -X POST "$SLACK_WEBHOOK" \
    -H 'Content-Type: application/json' \
    -d "{\"text\":\"🔙 NekoCafé ${ENV} 已回滚（用时 ${ELAPSED}s）\"}" \
    > /dev/null || true
fi

# 6. 记录 DORA：MTTR
echo "{\"event\":\"rollback_completed\",\"env\":\"${ENV}\",\"at\":\"${END_TS}\",\"mttr_seconds\":${ELAPSED}}" \
  >> /tmp/nekocafe-rollback.log
