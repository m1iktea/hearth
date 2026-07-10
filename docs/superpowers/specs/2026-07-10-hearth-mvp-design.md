# Hearth MVP 设计文档

日期：2026-07-10
状态：已确认

## 1. 项目概述

Hearth 是一个自研家庭中枢系统，部署于飞牛 NAS 的 Docker 环境。MVP 阶段提供：

1. **导航页**：集中管理各节点/服务入口链接，支持分类、图标、自定义
2. **节点状态监控**：Proxmox VM 状态、飞牛 Docker 容器状态、ImmortalWrt 运行/网络状态
3. **统一仪表盘**：一个页面汇总三个节点的整体运行情况

架构需预留扩展能力：设备控制、自动化规则、通知聚合、数据看板。

### 环境约束

- PVE 是虚拟化底座，保持干净，不部署额外服务
- ImmortalWrt 职责聚焦网络，不部署额外服务（因此采集只能走远程 API）
- 部署位置只有飞牛 NAS 的 Docker
- 不考虑高可用，家用单机；低资源占用，长期常驻
- 不使用现成导航面板（Homepage/Dashy 等），自研保证扩展性

## 2. 已确认的技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 后端 | Go，单二进制 | 低内存、无运行时依赖，适合 NAS 常驻 |
| 前端 | Vue 3 + Vite + TypeScript + Naive UI | 组件化、中文生态好、适合仪表盘快速迭代 |
| ImmortalWrt 接入 | LuCI ubus HTTP RPC | 路由器零改动，能拿到 CPU/内存/流量等详细数据 |
| 存储 | SQLite（modernc.org/sqlite 纯 Go 驱动） | 单文件随 volume 持久化，无 CGO，保持单二进制 |
| 认证 | MVP 不做登录，API 层预留 middleware 插槽 | 仅内网访问；后续加认证不动业务代码 |
| 采集模型 | 后台轮询 + 内存快照缓存 | API 响应毫秒级；单源故障隔离；后续加历史记录只需把快照落库，架构不变 |
| 凭据管理 | 全部走环境变量，不入库不进代码 | 安全基线 |

## 3. 仓库结构（monorepo）

```
hearth/
├── server/                 # Go 后端
│   ├── cmd/hearth/         # main 入口
│   └── internal/
│       ├── collector/      # 采集器接口 + proxmox/docker/openwrt 实现
│       ├── store/          # SQLite（导航项）+ 内存快照（实时状态）
│       ├── api/            # REST handler + 路由 + middleware
│       └── config/         # 环境变量加载与校验
├── web/                    # Vue 3 + Vite + TS 前端
├── deploy/                 # Dockerfile、docker-compose.yml、.env.example
└── docs/                   # 设计文档、部署与更新说明
```

## 4. 后端设计

### 4.1 Collector 接口

```go
type Collector interface {
    Name() string                                  // "proxmox" | "docker" | "openwrt"
    Collect(ctx context.Context) (Snapshot, error)
}
```

- **proxmox**：PVE API Token 认证，调 `/api2/json/nodes` 与 `/api2/json/nodes/{node}/qemu`，读节点资源与各 VM 的 CPU/内存/开关机状态
- **docker**：通过挂载的 `/var/run/docker.sock`（只读）调 Docker Engine API，读容器列表、状态、资源占用
- **openwrt**：登录 LuCI `/ubus` 接口获取 session，调 `system.info`、`network.interface` 等读 CPU 负载/内存/运行时长/网口状态与流量

### 4.2 调度与快照

- 调度器每 10s（可配置）并发触发所有 collector，带超时 context
- 结果写入内存快照（`sync.RWMutex` 保护），含 `collected_at` 时间戳
- 单源失败：该源标记 `status: offline` + `last_error`，保留上次成功快照供参考，不影响其他源与页面加载

### 4.3 存储

- SQLite 表：`nav_categories`（分类）、`nav_items`（名称、URL、图标、排序、所属分类）
- 数据库文件位于 `/data/hearth.db`（volume 挂载）
- 实时状态只在内存，MVP 不落库（后续数据看板阶段再加采样落库）

### 4.4 API

前缀 `/api/v1`，统一响应包裹：`{ "success": bool, "data": ..., "error": ... }`

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /status | 全部数据源汇总快照 |
| GET | /status/{source} | 单数据源快照 |
| GET | /nav | 导航分类+条目列表 |
| POST/PUT/DELETE | /nav/categories, /nav/items | 导航 CRUD |
| GET | /healthz | 存活探针 |

- middleware 链预留 auth 插槽（MVP 为 no-op）
- 前端构建产物 `go:embed` 进二进制，`/` 直接返回 SPA

### 4.5 配置（环境变量）

`PVE_URL`、`PVE_TOKEN_ID`、`PVE_TOKEN_SECRET`、`DOCKER_HOST`（默认 unix socket）、`OPENWRT_URL`、`OPENWRT_USERNAME`、`OPENWRT_PASSWORD`、`HEARTH_POLL_INTERVAL`、`HEARTH_DATA_DIR`。启动时校验必填项，缺失则报错退出。

## 5. 前端设计

- 技术：Vue 3 + Vite + TypeScript + Naive UI + Vue Router + Pinia
- 页面：
  - **Dashboard**：三节点汇总卡片（在线状态、CPU/内存概览、告警色标）
  - **Nav**：分类分组的卡片式导航，点击跳转目标服务；管理模式下可增删改
  - **Nodes**：单节点详情（PVE 的 VM 列表 / 飞牛的容器列表 / ImmortalWrt 网口详情）
- 数据获取：每 10s 轮询 `/api/v1/status`，Pinia 存放状态
- 离线展示：数据源离线时显示离线徽标与最后成功时间，不阻塞其他区域渲染

## 6. 部署

- **多阶段 Dockerfile**：`node:20-alpine` 构建前端 → `golang:1.22-alpine` 构建后端（embed 前端 dist）→ 最终 `alpine` 运行镜像，非 root 用户运行
- **docker-compose**：
  - volume：`./data:/data`（SQLite 持久化）
  - `env_file: .env`（凭据注入，`.env` 加入 .gitignore，提供 `.env.example`）
  - `/var/run/docker.sock:/var/run/docker.sock:ro`
- **更新流程**：git push → 本地/CI 构建镜像 → 飞牛上 `docker compose pull && docker compose up -d`

## 7. 错误处理

- collector 层：超时/认证失败/网络错误统一归为源级 offline，记录 last_error，日志输出详细上下文
- API 层：统一错误响应格式，不泄露凭据等敏感信息
- 前端：接口失败显示友好提示与重试，不白屏

## 8. 测试策略

- collector：`httptest` mock Proxmox/ubus/Docker API 响应，覆盖正常/超时/认证失败路径
- store：SQLite CRUD 单测（内存模式）
- api：handler 单测（mock store 与快照）
- 目标：核心逻辑覆盖率 80%+

## 9. 扩展预留

- 新数据源 = 新增一个 Collector 实现 + 注册
- 新功能模块 = 后端加路由 + 前端加页面
- 通知聚合/自动化：调度器与快照层已是事件产生点，后续可在快照变更处挂钩子
- 历史数据：在调度器写快照处增加 SQLite 采样落库即可

## 10. 明确不做（MVP）

- 登录认证（预留插槽）
- 历史数据与趋势图
- 智能家居控制、自动化规则、通知聚合
- 高可用、多实例
