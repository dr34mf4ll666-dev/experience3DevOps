
# ─── 第 1 阶段：编译 ──────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache ca-certificates git && update-ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=local
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

RUN go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /out/reservation .

# ─── 第 2 阶段：运行 ──────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="nekocafe-reservation"
LABEL org.opencontainers.image.licenses="MIT"

USER nonroot:nonroot

COPY --from=builder /out/reservation /app/reservation
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

EXPOSE 8080

ENTRYPOINT ["/app/reservation"]
