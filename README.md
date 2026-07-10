# Hearth

自研家庭中枢（home hub）。MVP 功能：导航页 + Proxmox VE / Docker / ImmortalWrt 状态监控 + 统一仪表盘。

## 架构

```
collectors ──(每 10s 轮询)──▶ snapshot store（内存）──▶ REST API ──▶ Vue SPA
                                                                    （go:embed 单二进制）
导航数据 ──────────────────────────────────────────────────────────▶ SQLite
```

- **collectors**：Proxmox VE、Docker、ImmortalWrt 三路独立采集，互不干扰；未配置的源自动跳过
- **snapshot store**：内存快照，按 source 键入，读写加锁；`/api/v1/status` 实时返回
- **REST API**：`/api/v1/status*`（状态）、`/api/v1/nav*`（导航 CRUD），统一 `{"success","data","error"}` envelope
- **前端**：Vue 3 SPA 通过 `go:embed` 编译进单二进制，生产零额外依赖

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go（stdlib `net/http`）+ `modernc.org/sqlite`（纯 Go，无 CGO） |
| 前端 | Vue 3 + Vite + TypeScript + Naive UI + Pinia |
| 构建 | Docker 多阶段（node:20-alpine → golang:1.25-alpine → alpine:3.20，非 root 运行） |

## 目录结构

```
hearth/
├── server/
│   ├── cmd/hearth/main.go              # 组装：config→collectors→scheduler→http
│   └── internal/
│       ├── config/                     # 环境变量加载与分组校验
│       ├── collector/                  # Collector 接口 + scheduler + proxmox/docker/openwrt
│       ├── store/                      # 内存快照 + SQLite 导航 CRUD
│       ├── api/                        # 路由、status/nav handlers、SPA 托管
│       └── webdist/                    # go:embed 前端构建产物
├── web/                                # Vue 3 + Vite + TS 前端
├── deploy/                             # Dockerfile / docker-compose.yml / .env.example
└── docs/deploy.md                      # 部署指南
```

## 快速开始（本地开发）

### 后端

```bash
cd server
# PVE 和 OpenWrt 不配置则对应源不启用；Docker 源默认读本机 /var/run/docker.sock
HEARTH_DATA_DIR=/tmp/hearth-dev go run ./cmd/hearth
```

按需追加环境变量（参考 `deploy/.env.example`）：

```bash
PVE_URL=https://192.168.x.x:8006 \
PVE_TOKEN_ID=hearth@pam!hearth \
PVE_TOKEN_SECRET=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
OPENWRT_URL=http://192.168.x.x \
OPENWRT_USERNAME=readonly \
OPENWRT_PASSWORD=xxx \
HEARTH_DATA_DIR=/tmp/hearth-dev \
go run ./cmd/hearth
```

后端默认监听 `:8080`。

### 前端

```bash
cd web
npm install
npm run dev    # dev proxy 已指向 :8080
```

### 测试

```bash
cd server && go test ./...
cd web && npm test
```

## 部署

见 [docs/deploy.md](docs/deploy.md)（飞牛 NAS Docker 一键部署）。

## Roadmap

- 设备控制：VM / 容器启停
- 自动化联动规则
- 通知聚合（微信 / 飞书）
- 历史数据看板
