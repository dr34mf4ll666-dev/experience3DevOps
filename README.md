仓库结构
nekocafe/
├── services/
│   ├── reservation/          # 预约服务（Go + Gin）
│   │   ├── Dockerfile        # 多阶段构建，distroless 基础镜像
│   │   ├── main.go
│   │   ├── api/              # HTTP 路由 & 处理器
│   │   ├── repository/       # 数据访问层（Postgres + Redis）
│   │   ├── observability/    # OTel 追踪初始化
│   │   └── config/           # 环境变量配置
│   └── member/               # 会员服务（Python + FastAPI）
│       ├── Dockerfile        # 多阶段构建，slim 基础镜像
│       ├── src/
│       │   ├── main.py       # FastAPI 应用入口
│       │   ├── storage.py    # 数据库模型（SQLAlchemy async）
│       │   ├── config.py     # Pydantic Settings
│       │   └── routes/       # 业务路由
│       └── pyproject.toml
├── infra/
│   ├── helm/                 # Helm Chart（dev / staging / prod 三套 values）
│   ├── k8s-manifests/        # 原生 K8s YAML（兜底）
│   └── observability/        # Prometheus / Grafana / Loki / Tempo / OTel 配置
├── .github/workflows/
│   ├── ci.yml                # CI：路径过滤 → Lint → 单测 → SAST → 构建 → 扫描
│   └── cd.yml                # CD：dev → staging → prod 金丝雀（手动审批）
├── scripts/
│   ├── rollback.sh           # 一键回滚脚本
│   ├── dora-collector.py     # DORA 四指标采集
│   └── load-test.sh          # 压测脚本
├── docs/
│   ├── runbook.md            # 运维手册（故障处置流程）
│   ├── rollback.md           # 回滚操作手册
│   └── architecture.md       # 架构详解
├── docker-compose.yml        # 本地一键起全栈（含可观测性）
└── Makefile                  # 常用命令封装

第一步：一次性准备（首次克隆后执行）
# 生成 Go 依赖锁文件
cd services/reservation
go env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=sum.golang.org
go mod tidy
cd ../..

# 生成 Python 依赖锁文件
cd services/member
del poetry.lock          # Windows 用 del，macOS/Linux 用 rm
poetry lock
cd ../..

第二步：启动全栈
docker compose up -d --build

第三步：验证服务
# 健康检查
curl http://localhost:8081/healthz   # → {"status":"ok"}
curl http://localhost:8082/healthz   # → {"status":"ok"}

# 创建一条预约
curl -X POST http://localhost:8081/api/v1/reservations \
  -H "Content-Type: application/json" \
  -H "X-Member-Id: M001" \
  -d "{\"store_id\":\"S001\",\"slot\":\"2026-06-01T18:00:00Z\",\"seats\":2}"

# 查询会员额度
curl http://localhost:8082/api/v1/members/M001/quota

第四步：查看监控
浏览器打开 http://localhost:3000，用户名/密码均为 admin。
进入 Dashboards → NekoCafé 文件夹：

NekoCafé Overview：QPS、P99 延迟、错误率、资源使用
NekoCafé DORA：部署频率、变更前置时间等四项指标

停止服务
docker compose down        # 停止（保留数据）
docker compose down -v     # 停止并清空数据库

❓ 常见问题
Q：go mod tidy 报 GOPROXY list is not the empty string？
bashgo env -w GOPROXY=https://goproxy.cn,direct
go env -w GOSUMDB=sum.golang.org
Q：Docker build 报 TLS 证书错误？
Docker Desktop → Settings → Docker Engine，添加国内镜像源：
json{
  "registry-mirrors": ["https://hub-mirror.c.163.com", "https://mirror.baidubce.com"]
}
Q：member 服务报 duplicate key value？
bashdocker compose down -v
docker compose up -d --build
Q：Grafana Dashboard 显示 No data？
正常现象，Prometheus 每 15 秒抓取一次，多发几次请求后稍等片刻刷新即可。访问 http://localhost:9090/targets 可确认抓取状态。

作者：周济
