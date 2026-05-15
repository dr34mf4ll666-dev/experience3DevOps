# syntax=docker/dockerfile:1.7
# ─── 第 1 阶段：编译 ──────────────────────────────
FROM golang:1.22-alpine AS builder

# 1) 系统层依赖（CA 根证书 + git，git 给私有 module 用）
RUN apk add --no-cache ca-certificates git=2.45.2-r0 && \
    update-ca-certificates

WORKDIR /build

# 2) 先拷 mod，命中 Docker layer cache
COPY go.mod go.sum ./
RUN go mod download

# 3) 拷源码，编译为静态可执行文件
COPY src ./src
ARG VERSION=dev
ARG COMMIT=local
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -trimpath \
    -o /out/reservation ./src

# ─── 第 2 阶段：运行 ──────────────────────────────
# distroless：无 shell、无包管理器，攻击面 ≈ 二进制本身
FROM gcr.io/distroless/static-debian12:nonroot

# 元数据
LABEL org.opencontainers.image.title="nekocafe-reservation"
LABEL org.opencontainers.image.source="https://github.com/nekocafe/nekocafe"
LABEL org.opencontainers.image.licenses="MIT"

# distroless 自带 nonroot 用户（uid=65532）
USER nonroot:nonroot

COPY --from=builder /out/reservation /app/reservation
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

EXPOSE 8080

# HEALTHCHECK 由 K8s readinessProbe / livenessProbe 接管，这里仅 Compose 用
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/app/reservation", "healthcheck"]

ENTRYPOINT ["/app/reservation"]
