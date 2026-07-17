# Hearth 技术文档

**维护范围**：架构、运行方式、配置、接口和日常运维。  
**产品需求**：见 [product.md](product.md)。

## 1. 架构概览

Hearth 是以 Docker 单实例运行的家庭中枢。它以低资源、长期运行和本地优先为原则；不依赖外部云服务，也不要求在 PVE 或 ImmortalWrt 上部署额外程序。

```text
Proxmox VE ─┐
Docker     ├─ collectors（默认每 10 秒）─> 内存快照 ─> REST API ─> Vue SPA
ImmortalWrt ┘                                      │
ARP 扫描、资产、健康巡检、事件 ────────────────────> SQLite
```

- **采集器**：独立轮询 Proxmox VE、Docker、ImmortalWrt。未配置的 PVE/OpenWrt 会跳过；单一来源失败不会阻塞其他来源。
- **快照**：实时采集结果保留在内存中，读取延迟低；失败时保留上一次成功数据并附带 `last_error`。
- **持久化**：SQLite 保存导航、设备资产、健康检查和离线/恢复事件，默认路径为 `/data/hearth.db`。
- **前端**：Vue 3 SPA 在构建时嵌入 Go 二进制，生产环境不需要额外 Web 服务。

## 2. 组件职责

| 组件 | 职责 |
|---|---|
| `collector/proxmox` | 通过 PVE API Token 读取节点和 QEMU VM 状态、资源使用率 |
| `collector/docker` | 通过只读 Docker socket 读取容器列表；对每个 running 容器并发调用 one-shot stats，差分计算 CPU%、内存用量 |
| `collector/openwrt` | 通过 LuCI ubus HTTP RPC 读取系统、网络接口和流量状态 |
| `discovery` | 主机网络模式下执行 ARP 扫描，按 MAC 地址去重纳管 |
| `health` | 对设备执行 Ping、TCP、HTTP 检查，记录最新结果和状态变化事件 |
| `store` | 内存快照与 SQLite 数据访问 |
| `api` | REST API、SPA 托管、请求日志和未来认证插槽 |
| `web` | Vue 前端，提供运维概览、导航、节点、设备和健康中心；仪表盘资源趋势区块（`TrendSection` / `MetricChart`）使用 `echarts` + `vue-echarts` 渲染折线图 |

## 3. API 概览

所有业务接口以 `/api/v1` 为前缀，响应使用 `{ "success", "data", "error" }` 包装。

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/healthz` | 存活探针 |
| GET | `/status`、`/status/{source}` | collector 实时快照 |
| GET/POST/PUT/DELETE | `/nav`、`/nav/categories`、`/nav/items` | 导航分类与链接管理；`nav_items` 含可空 `device_id`，一设备最多关联一个导航项；写操作校验设备存在性（422）与唯一关联（409） |
| GET/POST/PUT/DELETE | `/devices` | 设备资产管理 |
| GET | `/devices/{id}` | 设备详情，响应 `data` 附带 `nav_item` 字段（未关联时为 `null`） |
| POST/PUT/DELETE | `/devices/{id}/checks` | Ping/TCP/HTTP 检查管理 |
| GET | `/health`、`/events` | 当前健康结果与最近状态事件 |
| POST | `/discovery/arp` | 执行 ARP 扫描（仅启用时可用） |
| GET | `/capabilities` | 返回部署能力开关（如 `arp_discovery`），前端据此显隐功能入口 |
| GET | `/metrics` | 查询黑匣子时序指标；支持 `source`/`object`/`metric`/`since`/`limit` 过滤 |

首页的详细数据口径和后续聚合接口约束见 [product.md](product.md)。

## 4. 部署

### 4.1 前置条件

- Docker 与 Docker Compose 可用；
- Docker 主机可访问需要监控的 PVE/OpenWrt；
- 如需监控 Docker，Docker 主机必须提供 `/var/run/docker.sock`；
- 如需 ARP 扫描，Docker 主机必须直连目标二层网络。

### 4.2 配置

复制示例文件后填写凭据：

```bash
cp deploy/.env.example deploy/.env
```

`.env` 含凭据，必须保留在版本控制之外。PVE 请使用专用只读 Token；创建方式见根目录 README。ImmortalWrt 建议使用专用只读账号。

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PVE_URL`、`PVE_TOKEN_ID`、`PVE_TOKEN_SECRET` | 空 | 三者同时配置后启用 PVE 采集 |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker Engine 地址 |
| `DOCKER_GID` | `999` | Docker socket 所属组 GID，用于 Compose `group_add` |
| `OPENWRT_URL`、`OPENWRT_USERNAME`、`OPENWRT_PASSWORD` | 空 | 三者同时配置后启用 ImmortalWrt 采集 |
| `HEARTH_POLL_INTERVAL` | `10s` | collector 轮询周期，必须为正时长 |
| `HEARTH_HEALTH_INTERVAL` | `30s` | 健康检查周期，必须为正时长 |
| `HEARTH_SCAN_NETWORKS` | 空 | 逗号分隔的 CIDR；为空时扫描本地网络 |
| `HEARTH_ARP_DISCOVERY_ENABLED` | `false` | 由 Compose 模式设置，不建议手工覆盖 |
| `HEARTH_LISTEN` | `:8080` | 监听地址 |
| `HEARTH_DATA_DIR` | `/data` | SQLite 数据目录 |
| `HEARTH_EVENT_RETENTION_DAYS` | `90` | 事件（`events`/`system_events`）保留天数 |
| `HEARTH_METRIC_RETENTION_DAYS` | `30` | 黑匣子指标采样保留天数 |
| `HEARTH_METRIC_SAMPLE_INTERVAL` | `60s` | 指标落盘节流间隔（每个数据源） |
| `PVE_SSH_HOST`、`PVE_SSH_USER`、`PVE_SSH_PASSWORD` | 空 | PVE SSH 温度采集；三者同时配置后通过 SSH 读取节点温度 |
| `PVE_SSH_KEY_FILE` | 空 | PVE SSH 私钥路径（与密码二选一） |

PVE SSH 温度采集需在 PVE 主机以 root 运行 `deploy/pve-setup-sensors.sh`，一键创建受限账号并安装 `sensors` 依赖（详见根目录 README）。

读取 Docker socket 前，在 Docker 主机执行下列命令，并将结果填入 `DOCKER_GID`：

```bash
stat -c %g /var/run/docker.sock
```

### 4.3 启动模式

标准 Docker 模式适合纯监控和健康检查，ARP 扫描被明确禁用：

```bash
cd deploy
docker compose up -d --build
```

通过 `http://<Docker主机IP>:8080` 访问。

只有需要扫描同一二层 VLAN 的设备时才使用主机网络模式：

```bash
cd deploy
docker compose -f docker-compose.host-network.yml up -d --build
```

ARP 广播不能跨 VLAN；位于路由后网段的设备需要依赖 Ping/TCP/HTTP 检查，或让 Docker 主机接入该 VLAN。

### 4.4 更新、备份与恢复

```bash
git pull
cd deploy
docker compose up -d --build
```

主机网络部署请使用对应的 Compose 文件。持久化数据位于 `deploy/data/`；备份该目录即可：

```bash
cp -r deploy/data/ ~/hearth-backup-$(date +%Y%m%d)/
```

恢复时停止服务、还原 `deploy/data/`，再重新启动服务。

## 5. 黑匣子（Blackbox）

黑匣子在采集链路上旁路记录关键指标与最新快照，使 Hearth 自身随宿主机崩溃重启后，仍能通过 `GET /api/v1/metrics` 回看故障前的资源走势和最后已知状态。

### 存储结构

| 表 | 用途 |
|---|---|
| `metric_samples` | 时序指标采样（source/object/metric/value/created_at） |
| `snapshots` | 每个数据源的最后一次成功快照（JSON） |
| `system_events` | 系统级事件，如节点重启（uptime 回落检测） |

SQLite 开启 WAL 模式（`journal_mode=WAL`），降低高频小事务的 fsync 开销。

### 采样口径

| 来源 | 指标 |
|---|---|
| PVE 节点 | `cpu_pct`、`mem_pct`、`uptime_sec`、`vms_running` |
| Docker 容器（running） | `cpu_pct`（one-shot stats 差分）、`mem_pct`（used/limit） |
| Docker 汇总 | `containers_running`、`containers_total` |
| ImmortalWrt | `mem_used_pct`、`load1`、`uptime_sec` |

Docker 容器 CPU% 采用两轮差分（`cpu_delta / system_delta × online_cpus × 100`）。首轮无基线时 `cpu_pct` 为 `null`；容器重启导致累计值回退时同样按无基线处理，下一轮恢复正常差分。内存口径与 `docker stats` 一致：`used = usage − inactive_file`（cgroup v2）或 `usage − cache`（cgroup v1）。

### 保留策略

保留期由环境变量控制（见 4.2 配置表），超期数据由后台定时任务清理。

## 6. 运行与排障

### 数据源显示 offline

先查看服务日志：

```bash
docker logs hearth
```

| 现象 | 优先检查项 |
|---|---|
| Docker offline | `DOCKER_GID`、socket 挂载、Docker daemon 可用性 |
| PVE offline | URL、Token、网络连通性、防火墙和证书配置 |
| ImmortalWrt offline | URL、账号密码、LuCI ubus 可用性、网络连通性 |
| Ping 检查失败 | 目标地址、容器 `NET_RAW` capability、目标是否允许 ICMP |
| ARP 扫描不可用 | 是否使用主机网络 Compose、主机是否在目标二层网络 |

### 安全边界

- 凭据仅通过环境变量注入，不写入 SQLite 或代码库。
- Docker socket 以只读方式挂载；当前产品只读取 Docker 信息。
- 容器以非 root 用户运行；Ping 和 ARP 扫描需要 `NET_RAW`。
- 默认假设仅在受信任局域网访问。若通过公网访问，应在 Hearth 外使用 VPN 或反向代理等网络访问控制。

## 7. 本地开发与测试

```bash
# 后端
cd server
HEARTH_DATA_DIR=/tmp/hearth-dev go run ./cmd/hearth

# 前端（另开终端）
cd web
npm install
npm run dev

# 测试
cd server && go test ./...
cd web && npm test
```

## 8. 技术演进边界

后续需要新增数据源时，实现 `Collector` 并注册即可；新增产品模块可在 API 和前端分别增加路由。历史数据、通知与自动化应建立在稳定的状态变化事件和数据保留策略之上，避免直接耦合到单个采集器。
