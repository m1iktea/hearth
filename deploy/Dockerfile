# syntax=docker/dockerfile:1

# 前端资源与 Go 编译都不依赖目标 CPU 架构。固定在 Buildx 的原生构建平台执行，
# 避免在 x86 Runner 上通过 QEMU 模拟 ARM64 跑 npm ci。
# --- 前端构建 ---
FROM --platform=$BUILDPLATFORM node:20-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- 后端构建（embed 前端产物） ---
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS server
ARG TARGETOS
ARG TARGETARCH
WORKDIR /app/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=web /app/web/dist ./internal/webdist/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-s -w" -o /hearth ./cmd/hearth

# --- 运行镜像（非 root） ---
FROM alpine:3.20
RUN apk add --no-cache iputils arp-scan libcap-utils \
    && setcap cap_net_raw+ep /usr/bin/arp-scan \
    && adduser -D -u 1000 hearth \
    && mkdir -p /data \
    && chown hearth:hearth /data
COPY --from=server /hearth /usr/local/bin/hearth
USER hearth
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["hearth"]
