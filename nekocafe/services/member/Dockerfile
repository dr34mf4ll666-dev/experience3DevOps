# syntax=docker/dockerfile:1.7
# ─── 第 1 阶段：构建 wheel（含 C 扩展编译） ──────────────────────────────
FROM python:3.12-slim-bookworm AS builder

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    POETRY_VERSION=1.8.2 \
    POETRY_VIRTUALENVS_CREATE=false \
    POETRY_NO_INTERACTION=1

# 编译期依赖：装完后下一阶段不会带过去
RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential=12.9 \
        curl=7.88.1-10+deb12u5 \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir poetry==${POETRY_VERSION}

WORKDIR /build
COPY pyproject.toml poetry.lock ./
# 只装 main 依赖（不含 dev）；先解析依赖以利用 layer cache
RUN poetry install --only main --no-root --no-cache

COPY src ./src

# ─── 第 2 阶段：运行 ──────────────────────────────
FROM python:3.12-slim-bookworm AS runtime

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PATH="/usr/local/bin:$PATH"

# 运行期只装最小依赖
RUN apt-get update && apt-get install -y --no-install-recommends \
        libpq5=15.6-0+deb12u1 \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r nekocafe --gid 65532 \
    && useradd -r -g nekocafe --uid 65532 --shell /sbin/nologin --create-home nekocafe

LABEL org.opencontainers.image.title="nekocafe-member"
LABEL org.opencontainers.image.source="https://github.com/nekocafe/nekocafe"
LABEL org.opencontainers.image.licenses="MIT"

WORKDIR /app

# 拷贝已装好的 site-packages 与可执行
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin
COPY --from=builder /build/src ./src

USER 65532:65532
EXPOSE 8080

# Health：用 Python 自带 urllib（容器无 curl，最小化体积）
HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=3 \
    CMD ["python", "-c", "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://localhost:8080/healthz').status==200 else 1)"]

CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8080", "--workers", "2"]
