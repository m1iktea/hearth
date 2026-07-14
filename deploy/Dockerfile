# --- 前端构建 ---
FROM node:20-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- 后端构建（embed 前端产物） ---
FROM golang:1.25-alpine AS server
WORKDIR /app/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=web /app/web/dist ./internal/webdist/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /hearth ./cmd/hearth

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
