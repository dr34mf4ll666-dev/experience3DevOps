.PHONY: help up down clean smoke build lint test deploy-dev deploy-staging deploy-prod rollback dora

ENV ?= dev
REV ?= PREVIOUS
NAMESPACE = nekocafe-$(ENV)

help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

up: ## 启动本地全栈
	docker compose up -d --build
	@echo "✅ Stack is up. Try: make smoke"

down: ## 停止本地栈
	docker compose down

clean: ## 清理（含数据卷）
	docker compose down -v
	docker system prune -f

smoke: ## 烟雾测试
	@curl -fsS http://localhost:8081/healthz && echo " ✓ reservation OK"
	@curl -fsS http://localhost:8082/healthz && echo " ✓ member OK"
	@curl -fsS http://localhost:3000/api/health > /dev/null && echo " ✓ Grafana OK"
	@echo "🎉 All services healthy."

build: ## 构建两个服务镜像
	docker build -t nekocafe/reservation:local services/reservation
	docker build -t nekocafe/member:local services/member

lint: ## 跑所有 linter
	hadolint services/reservation/Dockerfile services/member/Dockerfile
	yamllint -c .yamllint.yml infra/ .github/
	cd services/reservation && golangci-lint run
	cd services/member && ruff check src tests

test: ## 跑所有单元测试
	cd services/reservation && go test ./... -cover
	cd services/member && poetry run pytest --cov=src --cov-report=term

scan: ## Trivy 扫描镜像
	trivy image --severity HIGH,CRITICAL --exit-code 1 nekocafe/reservation:local
	trivy image --severity HIGH,CRITICAL --exit-code 1 nekocafe/member:local

deploy-dev: ## 部署到 dev
	helm upgrade --install nekocafe-dev infra/helm \
		-f infra/helm/values-dev.yaml \
		-n nekocafe-dev --create-namespace --wait --timeout 5m

deploy-staging: ## 部署到 staging
	helm upgrade --install nekocafe-staging infra/helm \
		-f infra/helm/values-staging.yaml \
		-n nekocafe-staging --create-namespace --wait --timeout 5m

deploy-prod: ## 部署到 prod（金丝雀）
	helm upgrade --install nekocafe-prod infra/helm \
		-f infra/helm/values-prod.yaml \
		-n nekocafe-prod --create-namespace --wait --timeout 10m

rollback: ## 一键回滚（ENV=prod REV=PREVIOUS）
	./scripts/rollback.sh $(ENV) $(REV)

uninstall: ## 卸载（ENV=dev/staging/prod）
	helm uninstall nekocafe-$(ENV) -n $(NAMESPACE) || true
	kubectl delete namespace $(NAMESPACE) --ignore-not-found

dora: ## 采集 DORA 指标
	python3 scripts/dora-collector.py --since 14d --output 07_DORA指标报告.xlsx

load-test: ## 压测（默认 reservation）
	./scripts/load-test.sh http://localhost:8081
