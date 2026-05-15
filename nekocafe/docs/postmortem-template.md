# Postmortem — &lt;一句话标题&gt;

**事故日期**：YYYY-MM-DD
**严重度**：Sev1 / Sev2 / Sev3
**作者**：&lt;主笔人&gt;
**参与者**：&lt;名单&gt;
**状态**：草稿 / 评审中 / 已归档

> 本模板遵循 Google SRE Blameless Postmortem 范式。**不指责人，只指责系统。**

---

## 摘要（TL;DR）

一句话说清楚发生了什么、影响多大、修复了多久。

## 影响

- 业务影响：&lt;预约失败 X 次 / 影响用户 X 人 / 损失 X 元&gt;
- 服务影响：&lt;reservation 503 持续 8 分钟&gt;
- 数据影响：&lt;无 / 有：见下&gt;

## 时间线（UTC+8）

| 时间 | 事件 |
| --- | --- |
| 14:02 | 新版本镜像通过 staging，开始 prod canary 5% |
| 14:06 | 告警 `HighErrorRate` 触发 |
| 14:07 | oncall 在 Slack 响应 |
| 14:08 | 通过 Grafana 定位到 reservation 服务 |
| 14:09 | 触发 `make rollback ENV=prod` |
| 14:12 | 服务恢复，错误率回落 |

## 根因（5 Whys）

1. **现象**：reservation 在 v2 部署后开始大量返回 500
2. 为什么 500？→ Redis 连接超时
3. 为什么超时？→ Pool 大小被 v2 改成了 10
4. 为什么改 10？→ 误把 dev 配置带到 prod values 文件
5. 为什么没被 CI 拦下？→ values-prod 没有 schema 校验

→ **根因**：缺少环境配置漂移检测。

## 已做的修复

- [x] 回滚到 v1
- [x] 临时手动改 pool 为 100
- [x] 发布修正版 v3

## 待办（Action Items）

> 每条都要有 owner + due date，否则不算复盘完成。

- [ ] 为 helm values-* 加 JSON schema 校验（CI 阶段） — @alice — 2026-05-22
- [ ] 在 staging 增加 prod 流量回放 — @bob — 2026-06-01
- [ ] 把 Redis pool size 加入 Grafana Saturation 面板 — @carol — 2026-05-20

## 做得好的地方

- 5 分钟内告警 → 6 分钟内回滚 → 3 分钟内 MTTR
- 一键回滚脚本顺利工作，没有跑去翻 wiki

## 教训

- **环境配置不能靠 review，要靠机器**
- 金丝雀阈值有效拦住了影响扩大（仅 5% 流量受波及）
