# Docker 部署指南

## 前置条件

- 已安装 Docker 与 Docker Compose（`docker compose version` 可正常输出）
- 若使用主机网络模式的 ARP 发现，Docker 主机须直接连接目标局域网

---

## 准备工作

### 1. 创建 PVE 只读用户与 API Token（如需监控 PVE）

命令行操作见 [README「在 PVE 中创建只读用户」](../README.md#在-pve-中创建只读用户)，
三条 `pveum` 命令即可完成，得到 `PVE_TOKEN_ID=hearth@pve!monitor` 和 Token Secret。

### 2. 准备 ImmortalWrt 账号（如需监控路由器）

建议在 ImmortalWrt 上新建一个只读账号专用于 Hearth（通过 rpcd 配置），
或直接使用 `root` 账号（内网自担风险）。

---

## 首次部署

### 步骤 1：拉取代码

```bash
git clone https://github.com/m1iktea/hearth.git
cd hearth
```

### 步骤 2：配置环境变量

```bash
cp deploy/.env.example deploy/.env
```

用编辑器打开 `deploy/.env`，按实际填写：

```env
# ── Proxmox VE（不填则该源不启用）──────────────────────────────
PVE_URL=https://192.168.x.x:8006
PVE_TOKEN_ID=hearth@pve!monitor
PVE_TOKEN_SECRET=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# ── Docker（默认读 /var/run/docker.sock，通常不需改）──────────────
# DOCKER_HOST=unix:///var/run/docker.sock

# ── ImmortalWrt（不填则该源不启用）────────────────────────────────
OPENWRT_URL=http://192.168.x.x
OPENWRT_USERNAME=readonly
OPENWRT_PASSWORD=your_password

# ── 宿主机 docker.sock 属组 GID（见下方说明）──────────────────────
DOCKER_GID=999

# ── 可选，以下为默认值 ──────────────────────────────────────────
# HEARTH_POLL_INTERVAL=10s
# HEARTH_HEALTH_INTERVAL=30s
# HEARTH_SCAN_NETWORKS=192.168.1.0/24,192.168.10.0/24
# HEARTH_LISTEN=:8080
# HEARTH_DATA_DIR=/data
```

### 步骤 3：获取 docker.sock 的 GID

容器以非 root 用户运行，需要知道 Docker 主机上 `docker.sock` 所属组 GID 才能读取 socket：

```bash
stat -c %g /var/run/docker.sock
```

把输出的数字填进 `deploy/.env` 的 `DOCKER_GID=`。

### 步骤 4：选择部署模式并启动

| 模式 | Compose 文件 | 网络方式 | ARP 主动发现 |
|---|---|---|---|
| 标准 Docker | `docker-compose.yml` | bridge + `8080:8080` | 不支持（明确禁用） |
| 主机网络 Docker | `docker-compose.host-network.yml` | host | 支持同一二层 VLAN 的设备扫描 |

标准 Docker 模式：

```bash
cd deploy
docker compose up -d --build
```

服务通过 `http://<Docker主机IP>:8080` 访问。

需要主动扫描局域网设备时，使用主机网络 Docker 模式：

```bash
cd deploy
docker compose -f docker-compose.host-network.yml up -d --build
```

主机网络模式没有端口映射，服务直接监听 `HEARTH_LISTEN`（默认 `:8080`），通过 `http://<Docker主机IP>:8080` 访问。

## 健康检查说明

在「设备中心」手工录入设备后，可在设备详情添加三类检查：

- **Ping**：使用设备 IP（或自定义目标）检测网络可达性；Compose 已授予 `NET_RAW` 能力。
- **TCP**：检测指定 IP/主机名与端口，例如 NAS 的 `443` 或 SSH 的 `22`。
- **HTTP**：检测完整 URL；默认接受 2xx/3xx，也可指定期望状态码。

Hearth 不假设 ImmortalWrt 是主路由或 DHCP 服务器；设备发现以主动 ARP 为主，仍可在设备详情补充管理入口和备注。状态切换会记录在「健康中心」的站内事件列表中。

### 主动 ARP 发现

主机网络 Docker 模式下，「设备中心」的“扫描局域网”会运行 ARP 扫描，并按 MAC 地址自动去重、更新已有设备的 IP、新增首次发现的设备。默认扫描 Docker 主机所在的本地二层网段；若主机直接连接多个 VLAN/网段，可设置 `HEARTH_SCAN_NETWORKS` 为以逗号分隔的 CIDR。标准 Docker 模式明确禁用该操作。

ARP 广播只能发现**同一二层 VLAN**内的设备；位于路由后的 VLAN 需要让 Docker 主机接入对应 VLAN，或在后续版本使用 L3 的 Ping/TCP 探测补充。

---

## 更新

```bash
git pull
cd deploy
# 按当前使用的部署模式二选一
docker compose up -d --build
# 或：docker compose -f docker-compose.host-network.yml up -d --build
```

---

## 数据持久化

SQLite 数据库文件位于 `deploy/data/hearth.db`（对应容器内 `/data`）。

备份只需拷贝该目录：

```bash
cp -r deploy/data/ ~/hearth-backup-$(date +%Y%m%d)/
```

---

## 常见问题

### 某数据源显示 offline

查看日志，定位对应 source 的 warn 信息：

```bash
docker logs hearth
```

常见原因：

| 现象 | 原因 |
|------|------|
| Docker 源 offline | `DOCKER_GID` 填错，容器内用户没有读 socket 的权限 |
| PVE 源 offline | `PVE_URL` / Token 填错，或网络不通（防火墙） |
| OpenWrt 源 offline | `OPENWRT_URL` 不可达，或账号密码错误 |

### 端口冲突

修改 `deploy/docker-compose.yml` 中 `ports` 的宿主机端口（左侧），同步更新 `HEARTH_LISTEN` 环境变量（如果修改了容器内监听端口）。

---

## 安全说明

- `deploy/.env` 含有凭据，已被 `.gitignore` 排除，**不要提交到版本库**
- 容器以非 root 用户运行（Dockerfile 末阶段 `USER` 非 root）
- `docker.sock` 以只读方式挂载（`:ro`），Hearth 只读取容器信息，不控制容器
- Ping 健康检查和 ARP 扫描需要 `NET_RAW` capability；镜像仅给 `arp-scan` 可执行文件授予原始包能力，Hearth 服务仍以非 root 用户运行。若部署策略不允许，应禁用这两项能力并只使用 TCP/HTTP 检查
