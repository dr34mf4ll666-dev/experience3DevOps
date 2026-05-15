# Runbook — NekoCafé 运维手册

> 故障来了先翻这里。每条告警都对应到下面的一段，沿着指令做就能定位到根因；做不下去再升级。

## 0. 通用上下文

| 项 | 值 |
| --- | --- |
| 命名空间 | `nekocafe-dev / nekocafe-staging / nekocafe-prod` |
| 服务 | `reservation`（Go）、`member`（Python） |
| 镜像仓库 | `ghcr.io/nekocafe/{reservation,member}` |
| 监控入口 | Grafana `http://grafana.observability:3000` |
| 紧急联系 | Slack `#nekocafe-oncall`，PagerDuty 轮值 |

---

## 1. High Error Rate（5xx > 1%）

**症状**：`HighErrorRate` 告警 → Slack `#incident`。

**第一步：定位是哪个服务、哪个接口**

```bash
# 看哪个 service 的错误率高
kubectl -n nekocafe-prod logs -l app.kubernetes.io/component=reservation --tail=200 \
  | jq -c 'select(.level=="error")' | head -20
```

**第二步：用 traceId 串起全链路**

1. Grafana → Dashboards → NekoCafé Overview → 看"最近错误日志"面板
2. 复制任一 `trace_id`
3. 切换到 Tempo（左侧 Explore） → 粘贴 trace_id → 看完整调用链
4. 在 trace 视图里直接看到是 `reservation→member` 还是 DB 哪一段慢/挂

**常见根因 / 处置**：

| 根因 | 判定 | 处置 |
| --- | --- | --- |
| 上游会员服务挂 | trace 显示 `member.check_quota` 4xx/5xx | 看 member 日志；若必要 `make rollback ENV=prod` |
| 数据库连接耗尽 | `DBPoolNearExhaustion` 同时触发 | 临时扩 pool size，长期排查慢 SQL |
| 内存 OOM | Pod `OOMKilled` | 调高 `resources.limits.memory` 或排查泄漏 |
| 上一次发布引入 bug | 错误率与发布时间吻合 | **立刻回滚**：见 [rollback.md](rollback.md) |

---

## 2. High Latency（P99 > 500ms）

```bash
# 1. 看慢在哪
# Grafana → NekoCafé Overview → P99 延迟面板，确认是 reservation 还是 member

# 2. 抽样 trace 看具体 span 时长
# Tempo → Service Graph → 选目标 service → 看耗时分布

# 3. 看数据库慢查询（如果有 pg_stat_statements）
kubectl exec -it -n nekocafe-prod postgres-0 -- \
  psql -U neko -d nekocafe -c \
  "SELECT query, mean_exec_time FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 5;"
```

**处置**：

- 慢 SQL → 加索引、改写
- 缓存击穿（Redis miss 飙升）→ 加预热或 singleflight
- GC 抖动（Go）→ `GOGC=200` 提高阈值
- 客户端激增（QPS 翻倍）→ 触发 HPA 扩容；若 HPA 已到上限 → 提高 maxReplicas

---

## 3. Pod CrashLooping

```bash
NS=nekocafe-prod
kubectl -n $NS get pods | grep -v Running
kubectl -n $NS describe pod <pod>          # 看 Event
kubectl -n $NS logs <pod> --previous       # 看上一轮死前日志
```

**典型表现 → 处置**：

- `ImagePullBackOff`：GHCR 鉴权失效 → 重新 patch `image-pull-secret`
- `CrashLoopBackOff` + 启动期报 `db_dsn missing`：ExternalSecret 没同步 → 检查 vault 连通性
- `OOMKilled`：内存 limit 太小，先 patch + 排查泄漏

---

## 4. 金丝雀卡住或失败

```bash
# 看状态
kubectl argo rollouts get rollout nekocafe-prod-reservation -n nekocafe-prod

# 看 Analysis 结果
kubectl argo rollouts get rollout nekocafe-prod-reservation -n nekocafe-prod --watch

# 手动跳过当前 step（仅在确认无害时）
kubectl argo rollouts promote nekocafe-prod-reservation -n nekocafe-prod

# 立刻回滚
kubectl argo rollouts abort nekocafe-prod-reservation -n nekocafe-prod
kubectl argo rollouts undo  nekocafe-prod-reservation -n nekocafe-prod
```

**常见原因**：

- **Analysis 总是 Inconclusive**：流量太低，Prometheus 查询返回空 → 增大 `analysisInterval` 到 2-5 min，或先做小规模压测制造样本
- **Successful Step 但流量不增**：Ingress controller 没识别 stableIngress → 检查 nginx-ingress 版本是否支持 canary annotation
- **Analysis Failed but 服务看着正常**：阈值过严 → 暂时调宽 successThreshold，事后复盘

---

## 5. 紧急联系矩阵

| 严重度 | 谁先响应 | SLA | 沟通通道 |
| --- | --- | --- | --- |
| Sev1（业务停摆） | oncall → 主管 | 5 min | PagerDuty 电话 + #incident |
| Sev2（部分降级） | oncall | 15 min | Slack #incident |
| Sev3（单服务异常） | 当班开发 | 1 h | Slack #nekocafe-oncall |

---

## 6. 演练（Chaos Engineering）

每月 1 次，用 Chaos Mesh 注入以下故障，验证告警与自愈：

```bash
kubectl apply -f infra/observability/chaos/pod-kill.yaml      # 随机杀 Pod
kubectl apply -f infra/observability/chaos/network-delay.yaml # 注入 200ms 网络延迟
```

成功标准：**3 分钟内告警触发 → 5 分钟内 oncall 在 Grafana 定位到具体节点/接口**。
