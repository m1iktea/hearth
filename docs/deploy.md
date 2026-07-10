# 部署指南（飞牛 NAS Docker）

## 前置条件

- 飞牛 NAS 可 SSH 访问
- 已安装 `docker` 和 `docker compose`（`docker compose version` 可正常输出）

---

## 准备工作

### 1. 创建 Proxmox VE API Token（如需监控 PVE）

1. 登录 PVE Web 界面 → **Datacenter → Permissions → API Tokens**
2. 点击 **Add**，User 填 `hearth@pam`，Token ID 填 `hearth`
3. **Privilege Separation** 建议勾选（Token 权限独立于用户权限）
4. 记录生成的 Token Secret（仅显示一次）
5. 给 Token 赋只读角色：**Datacenter → Permissions → Add → API Token Permission**
   - Path：`/`，Token：`hearth@pam!hearth`，Role：`PVEAuditor`，勾选 **Propagate**

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
PVE_TOKEN_ID=hearth@pam!hearth
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
# HEARTH_LISTEN=:8080
# HEARTH_DATA_DIR=/data
```

### 步骤 3：获取 docker.sock 的 GID

容器以非 root 用户运行，需要知道宿主机 `docker.sock` 所属组 GID 才能读取 socket：

```bash
stat -c %g /var/run/docker.sock
```

把输出的数字填进 `deploy/.env` 的 `DOCKER_GID=`。

### 步骤 4：启动

```bash
cd deploy
docker compose up -d --build
```

服务启动后访问 `http://<NAS-IP>:8080`。

---

## 更新

```bash
git pull
cd deploy
docker compose up -d --build
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
