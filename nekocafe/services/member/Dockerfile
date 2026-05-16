# ─── 第 1 阶段：构建依赖 ──────────────────────────────
FROM python:3.12-slim-bookworm AS builder

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    POETRY_VERSION=1.8.2 \
    POETRY_VIRTUALENVS_CREATE=false \
    POETRY_NO_INTERACTION=1

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential curl \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir poetry==${POETRY_VERSION}

WORKDIR /build
COPY pyproject.toml poetry.lock ./
RUN pip install --no-cache-dir setuptools && \
    poetry install --only main --no-root --no-cache

COPY src ./src

# ─── 第 2 阶段：运行 ──────────────────────────────
FROM python:3.12-slim-bookworm AS runtime

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

RUN apt-get update && apt-get install -y --no-install-recommends \
        libpq5 ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r nekocafe --gid 65532 \
    && useradd -r -g nekocafe --uid 65532 --shell /sbin/nologin --create-home nekocafe

LABEL org.opencontainers.image.title="nekocafe-member"
LABEL org.opencontainers.image.licenses="MIT"

WORKDIR /app
COPY --from=builder /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin
COPY --from=builder /build/src ./src

USER 65532:65532
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=15s --retries=3 \
    CMD ["python", "-c", "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://localhost:8080/healthz').status==200 else 1)"]

CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8080", "--workers", "2"]
