# Rollback — 回滚操作手册

> 「先回滚，再排查」是 SRE 的黄金法则。本文给三种回滚路径：自动、半自动、纯手动。

## 0. 决策树

```
线上异常 → 错误率/延迟突增？
   ├─ 是 → 与最近一次发布吻合？
   │      ├─ 是 → 立即回滚（本文第 1 节，30 秒动手）
   │      └─ 否 → 走 runbook 排查
   └─ 否 → 走 runbook 排查
```

**回滚优先级永远高于排查**。线上稳定后再去找根因。

---

## 1. 自动回滚（默认）

Argo Rollouts 在金丝雀阶段持续跑 AnalysisTemplate：

- **P95 延迟 > 300 ms**（连续 3 次）→ 自动 abort + undo
- **错误率 > 1%**（连续 3 次）→ 自动 abort + undo

回滚完成会发 Slack `#incident` 通知，无需人工介入。但请打开 Grafana 确认服务确实恢复。

---

## 2. 半自动一键脚本

任何环境（dev/staging/prod）通用：

```bash
# 回滚到上一个稳定版本
make rollback ENV=prod

# 等价命令
./scripts/rollback.sh prod

# 指定具体 helm revision
./scripts/rollback.sh prod 7
```

脚本会自动判断使用 `kubectl argo rollouts undo` 还是 `helm rollback`，并等到所有 Pod Ready 后才返回。退出码：

| 码 | 含义 |
| --- | --- |
| 0 | 成功，已健康 |
| 1 | 参数错误 |
| 2 | 回滚命令本身失败（K8s API 报错） |
| 3 | 回滚后服务仍未恢复健康 → 升级 Sev1 |

---

## 3. 纯手动 kubectl

万一脚本挂了：

```bash
# 方式 A：Argo Rollouts
NS=nekocafe-prod
for svc in reservation member; do
  kubectl argo rollouts abort nekocafe-prod-${svc} -n $NS
  kubectl argo rollouts undo  nekocafe-prod-${svc} -n $NS
done
kubectl argo rollouts status rollout/nekocafe-prod-reservation -n $NS

# 方式 B：Helm
helm history nekocafe-prod -n $NS          # 找到要回的版本号
helm rollback nekocafe-prod 7 -n $NS --wait --timeout 5m

# 方式 C：原生 Deployment（最后手段）
kubectl rollout undo deployment/nekocafe-prod-reservation -n $NS
kubectl rollout status deployment/nekocafe-prod-reservation -n $NS
```

---

## 4. 数据相关的回滚（特别小心）

**镜像可以无脑回滚，schema 不能。** 如果新版本做了 DB migration：

- **前向兼容的 migration**（加字段、加表）→ 可以直接回滚应用镜像，schema 保留
- **破坏性 migration**（删字段、改类型）→ 需要执行 down migration；这就是为什么本项目要求所有 migration 都成对（up + down）

如果不确定，立刻升级到 Sev1，叫上数据 owner。

---

## 5. 回滚后的复盘 checklist

回滚 24 小时内必须开复盘会，输出：

- [ ] 故障时间线（首次告警 → 决策回滚 → 服务恢复）
- [ ] 根因（5 Whys）
- [ ] 当时 CI 是否拦截了？为什么没拦截？
- [ ] 对应的回归测试是否要加？
- [ ] DORA 失败计入：在 `scripts/dora-collector.py` 用 `incident` label 自动记录 MTTR

复盘模板见 `docs/postmortem-template.md`（按 Google SRE Blameless Postmortem 范式）。
