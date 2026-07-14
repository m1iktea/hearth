# Hearth MVP 设计文档

日期：2026-07-14
状态：已确认并完成第一轮扩展

## 1. 项目概述

Hearth 是一个自研家庭中枢系统，以 Docker 单实例运行。当前阶段提供：

1. **导航页**：集中管理各节点/服务入口链接，支持分类、图标、自定义
2. **节点状态监控**：Proxmox VM 状态、Docker 容器状态、ImmortalWrt 运行/网络状态
3. **统一仪表盘**：一个页面汇总三个节点的整体运行情况
4. **设备中心**：ARP 主动发现并按 MAC 自动纳管设备；可维护管理入口、位置、备注与标签化类型
5. **健康中心**：设备 Ping、TCP、HTTP 检查，展示最近状态、延迟和离线/恢复事件

架构需预留扩展能力：设备控制、自动化规则、通知聚合、数据看板。

### 环境约束

- PVE 是虚拟化底座，保持干净，不部署额外服务
- ImmortalWrt 职责聚焦网络，不部署额外服务（因此采集只能走远程 API）
- 以 Docker 单实例运行；不绑定具体 NAS、发行版或云平台
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
| 设备发现 | 主机网络模式下以 `arp-scan` 主动 ARP 扫描 | 不依赖 DHCP 服务器或 OpenWrt 是否为主路由；MAC 用于设备去重 |
| 部署模式 | 标准 Docker / 主机网络 Docker 两套 Compose | 标准模式安全地禁用扫描；主机网络模式才能访问真实二层广播域 |

## 3. 仓库结构（monorepo）

```
hearth/
├── server/                 # Go 后端
│   ├── cmd/hearth/         # main 入口
│   └── internal/
│       ├── collector/      # 采集器接口 + proxmox/docker/openwrt 实现
│       ├── discovery/      # ARP 主动发现与 arp-scan 输出解析
│       ├── health/         # Ping/TCP/HTTP 后台巡检
│       ├── store/          # SQLite（导航、资产、检查、事件）+ 内存快照
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

- SQLite 表：`nav_categories`、`nav_items`（导航）；`devices`（资产台账）；`health_checks`（巡检配置与最近状态）；`events`（离线/恢复事件）
- 数据库文件位于 `/data/hearth.db`（volume 挂载）
- collector 的实时状态保留在内存；巡检的最近状态和事件持久化，后续数据看板再增加时序采样

### 4.4 API

前缀 `/api/v1`，统一响应包裹：`{ "success": bool, "data": ..., "error": ... }`

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /status | 全部数据源汇总快照 |
| GET | /status/{source} | 单数据源快照 |
| GET | /nav | 导航分类+条目列表 |
| POST/PUT/DELETE | /nav/categories, /nav/items | 导航 CRUD |
| GET/POST/PUT/DELETE | /devices | 设备资产 CRUD；单设备响应附带健康检查 |
| POST/PUT/DELETE | /devices/{id}/checks | Ping/TCP/HTTP 检查 CRUD |
| GET | /health | 启用检查的当前状态 |
| GET | /events | 最近离线/恢复事件 |
| POST | /discovery/arp | 主机网络模式下发起 ARP 扫描并自动纳管 |
| GET | /healthz | 存活探针 |

- middleware 链预留 auth 插槽（MVP 为 no-op）
- 前端构建产物 `go:embed` 进二进制，`/` 直接返回 SPA

### 4.5 配置（环境变量）

`PVE_URL`、`PVE_TOKEN_ID`、`PVE_TOKEN_SECRET`、`DOCKER_HOST`（默认 unix socket）、`OPENWRT_URL`、`OPENWRT_USERNAME`、`OPENWRT_PASSWORD`、`HEARTH_POLL_INTERVAL`、`HEARTH_HEALTH_INTERVAL`、`HEARTH_DATA_DIR`。`HEARTH_ARP_DISCOVERY_ENABLED` 由 Compose 模式设置；`HEARTH_SCAN_NETWORKS` 可指定多个扫描 CIDR。启动时校验必填项和 CIDR 格式，缺失或非法则报错退出。

## 5. 前端设计

- 技术：Vue 3 + Vite + TypeScript + Naive UI + Vue Router + Pinia
- 页面：
  - **Dashboard**：三节点汇总卡片（在线状态、CPU/内存概览、告警色标）
  - **Nav**：分类分组的卡片式导航，点击跳转目标服务；管理模式下可增删改
  - **Nodes**：单节点详情（PVE 的 VM 列表 / Docker 容器列表 / ImmortalWrt 网口详情）
  - **Devices**：扫描局域网、资产列表、设备编辑与管理入口
  - **Device Detail**：设备资料、健康检查配置与最新结果
  - **Health**：全部检查状态和离线/恢复事件
- 数据获取：每 10s 轮询 `/api/v1/status`，Pinia 存放状态
- 离线展示：数据源离线时显示离线徽标与最后成功时间，不阻塞其他区域渲染

## 6. 部署

- **多阶段 Dockerfile**：`node:20-alpine` 构建前端 → `golang:1.25-alpine` 构建后端（embed 前端 dist）→ 最终 `alpine` 运行镜像；Hearth 以非 root 用户运行，只有 `arp-scan` 可执行文件拥有最小 `NET_RAW` file capability
- **标准 Docker Compose**（`docker-compose.yml`）：
  - volume：`./data:/data`（SQLite 持久化）
  - `env_file: .env`（凭据注入，`.env` 加入 .gitignore，提供 `.env.example`）
  - `/var/run/docker.sock:/var/run/docker.sock:ro`
  - bridge 网络与 `8080:8080` 端口映射，明确禁用 ARP 扫描
- **主机网络 Docker Compose**（`docker-compose.host-network.yml`）：`network_mode: host` + `NET_RAW`，启用 ARP 扫描同一二层 VLAN 的设备

## 7. 错误处理

- collector 层：超时/认证失败/网络错误统一归为源级 offline，记录 last_error，日志输出详细上下文
- API 层：统一错误响应格式，不泄露凭据等敏感信息
- 前端：接口失败显示友好提示与重试，不白屏

## 8. 测试策略

- collector：`httptest` mock Proxmox/ubus/Docker API 响应，覆盖正常/超时/认证失败路径
- store：SQLite CRUD 单测（内存模式）
- api：handler 单测（mock store 与快照）
- discovery：验证 `arp-scan` 输出解析和 MAC 去重
- 目标：核心逻辑覆盖率 80%+

## 9. 扩展预留

- 跨 VLAN / 路由后网段的 L3 Ping/TCP 主动发现
- 新数据源 = 新增一个 Collector 实现 + 注册
- 新功能模块 = 后端加路由 + 前端加页面
- 通知聚合/自动化：调度器与快照层已是事件产生点，后续可在快照变更处挂钩子
- 历史数据：在调度器写快照处增加 SQLite 采样落库即可

## 10. 明确不做（MVP）

- 登录认证（预留插槽）
- 历史数据与趋势图
- 智能家居控制、自动化规则、通知聚合
- 高可用、多实例
