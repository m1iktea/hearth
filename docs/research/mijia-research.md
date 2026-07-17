# 米家智能设备接入调研（2026-07）

## 背景

Hearth 的 future-plan 已确定米家接入策略：**不直接实现米家云协议**，而是通过 Home Assistant 中转。本文在此基础上深入调研 Go 生态可用库、token 获取现状、三条备选路径的对比，以及与现有 collector 架构的整合设计。

Hearth collector 契约（`server/internal/collector/collector.go`）：
```go
type Collector interface {
    Name() string
    Collect(ctx context.Context) (any, error)
}
```
`Collect` 返回任意结构体，由 SnapshotStore 封装为 `Snapshot{Source, Status, CollectedAt, Data}`。

---

## Go 库生态评估（2026 现状）

| 库 | Stars | 最后活跃 | 协议覆盖 | 设备支持 | 可用性评分 |
|---|---|---|---|---|---|
| **ysh0566/go-mihome** | 新兴 | 2026-04（最新提交） | 云 OAuth + MIoT spec + 本地 MIPS + LAN UDP | 全设备（依赖 spec） | ★★★★★ |
| **uole/miio** | 2 | 2024-03 创建 | miIO UDP + 米云 API | 通用（需手写 spec） | ★★★☆☆ |
| **icepie/miio.go** | 10 | 2022-06 | miIO UDP + MIoT GetProperties | 通用 | ★★☆☆☆ |
| **ofen/miio-go** | 3 | 2022-05 | miIO UDP JSON-RPC | 通用 | ★★☆☆☆ |
| **vkorn/go-miio** | 已归档 | 2019 | 网关子设备广播 | 温湿传感器/开关/人体感应 | ★☆☆☆☆ |
| **nickw444/miio-go** | 37 | 2019 | miIO UDP | 仅插座、Yeelight | ★☆☆☆☆ |
| **mkelcik/go-ha-client** | 活跃 | 2025-12 稳定 v2 | HA REST + WebSocket | HA 所有实体 | ★★★★★ |

**关键发现**：`ysh0566/go-mihome` 是 2026 年 Go 生态中唯一同时覆盖云 OAuth、MIoT spec、MIPS 推送、LAN 直连的完整实现，设计参考小米官方 HA 集成 `XiaoMi/ha_xiaomi_home`，且提供完整示例，是直连路径中唯一生产可用的选项。

Go HA 客户端中，`mkelcik/go-ha-client`（v2，Go 1.25+）支持 REST + WebSocket，API 稳定，文档齐全，与 Hearth 轮询架构匹配最好。

---

## Token 获取方式现状

| 方式 | 难度 | 2026 可用性 | 说明 |
|---|---|---|---|
| **Xiaomi-cloud-tokens-extractor**（PiotrMachowski） | 低 | 可用（需 2FA 配合） | Python 脚本/Docker 一键运行，支持 QR 码登录绕过密码限制；2025 年小米加了 2FA，需每次用信箱/手机验证码 |
| **macOS 米家 App 数据库提取** | 中 | 可用 | 从 `_mihome.sqlite` ZDEVICE 表取 ZTOKEN，再用 AES-ECB 全零密钥解密为 32 字符 token |
| **iOS 备份提取** | 中 | 可用（需旧版 App） | 解压 iTunes 备份，查询 `miio2.db` |
| **设备初次配网握手** | 高 | 部分可用 | 设备联网前广播明文 token，已配网设备不再暴露 |
| **Telnet/UART 进网关** | 高 | 型号限定 | 需要开启 Telnet（`522222` 按键法）或拆机接 UART |

**结论**：日常使用推荐 `Xiaomi-cloud-tokens-extractor` + QR 码方式，一次性操作；token 不会频繁变化，写入 Hearth 配置文件即可。macOS 用户可备选数据库提取。

---

## 三条路径对比与推荐

### 路径 A：直连 miIO/MIoT 局域网协议（纯 Go）

```
米家设备 <--UDP:54321--> go-mihome LAN Client <--> Hearth mijia collector
```

- **优点**：无额外依赖组件，单二进制部署不变，延迟最低（局域网直连）
- **缺点**：
  - token 需手动获取并配置，每台设备单独维护
  - 仅支持 WiFi 直连设备；Zigbee/BLE 子设备必须经网关，网关型号覆盖有限
  - `go-mihome` 是新兴库，API 尚未冻结，有维护风险
  - Docker 容器需 `host` 网络或开放 UDP 54321 + 组播，部署配置复杂

### 路径 B：经 Home Assistant 中转（**future-plan 推荐路径**）

```
米家设备 <-> 米家云/本地网关 <-> Home Assistant <--HTTP/WS--> Hearth homeassistant collector
```

- **优点**：
  - HA 负责所有设备兼容性（WiFi/Zigbee/BLE/网关）和小米 OAuth
  - Hearth 只调 HA REST API (`GET /api/states`)，零协议实现负担
  - `mkelcik/go-ha-client` 现成可用，接入成本极低
  - HA 已是 HomeLab 标配，不增加额外基础设施
  - HA 不可用时 Hearth 明确显示该数据源异常，不影响其他 collector
- **缺点**：
  - 强依赖 HA 部署（额外维护一个服务）
  - 状态延迟取决于 HA 轮询周期（通常 30–60s，或订阅 WebSocket 实时推送）

### 路径 C：python-miio 子进程桥接

```
米家设备 <--UDP--> python-miio subprocess <--stdin/stdout JSON--> Hearth
```

- **优点**：python-miio 设备覆盖最广（社区维护多年）
- **缺点**：
  - 破坏"单 Go 二进制"部署形态，镜像需打包 Python + pip 依赖
  - 子进程生命周期管理复杂（崩溃、僵尸进程、超时）
  - 运维成本高，不推荐用于 Hearth

### 推荐

**首选路径 B（HA 中转）**，与 future-plan 一致。  
**备选路径 A**，仅适用于：不想部署 HA、设备均为 WiFi 直连、可接受手动管理 token。  
**路径 C 不推荐**。

---

## 与现有架构整合设计

### homeassistant collector 形态

新增 `server/internal/collector/homeassistant/` 包，实现 `Collector` 接口：

```go
// Name 返回 "homeassistant"
func (c *Collector) Name() string { return "homeassistant" }

// Collect 调用 GET /api/states，过滤用户配置的 entity_id 列表
func (c *Collector) Collect(ctx context.Context) (any, error) {
    states, err := c.haClient.GetStates(ctx)
    // 过滤 config.EntityIDs，映射为 []DeviceState
}
```

**配置示例**（`config.yaml`）：
```yaml
collectors:
  homeassistant:
    url: "http://homeassistant.local:8123"
    token: "long-lived-access-token"
    entities:
      - sensor.living_room_temperature
      - sensor.bedroom_humidity
      - switch.kitchen_plug
      - binary_sensor.front_door
```

### 数据结构映射

`Collect` 返回的 `Data` 字段（`Snapshot.Data`）建议结构：

```go
type HASnapshot struct {
    Entities []EntityState `json:"entities"`
}

type EntityState struct {
    EntityID    string            `json:"entity_id"`
    State       string            `json:"state"`        // "on"/"off"/"23.5"/etc.
    Attributes  map[string]any    `json:"attributes"`   // temperature, unit_of_measurement 等
    LastChanged time.Time         `json:"last_changed"`
}
```

**指标命名约定**（与现有 proxmox/docker collector 对齐）：
- `source`: `"homeassistant"`
- `object`: entity_id（如 `sensor.living_room_temperature`）
- `metric`: HA state 字段（`state`、`attributes.temperature`、`attributes.humidity`）

### 轮询 vs 订阅

| 方式 | 适配性 | 说明 |
|---|---|---|
| **REST 轮询**（推荐首期） | 与 Hearth 10s 轮询架构天然匹配 | `GET /api/states` 批量拉取所有实体，简单可靠 |
| WebSocket 订阅 | 延迟更低，但需维护长连接 | 二期优化；`mkelcik/go-ha-client` 已支持 `subscribe_events` |

### 局域网权限与 Docker 要求

- HA 中转路径：Hearth 仅需 HTTP 访问 HA（端口 8123），**无需 host 网络，无需 UDP**，Docker bridge 网络即可。
- 直连路径（备选）：需要 `network_mode: host` 或开放 UDP 54321，以及容器内发送 UDP 广播（255.255.255.255）的权限。

---

## 风险与暂不做

### 已知风险

| 风险 | 级别 | 缓解 |
|---|---|---|
| HA 不可用导致米家数据源整体离线 | 中 | Hearth collector 独立失败不影响其他数据源；已在 future-plan 中明确 |
| 米家 2FA 强制要求导致 token 更新麻烦 | 低 | token 长期有效，初始化一次即可；可配套文档说明更新步骤 |
| `go-mihome` 库 API 不稳定（直连路径） | 中 | 在库接口外套一层 adapter，隔离变动 |
| HA WebSocket 长连接断线重连 | 低（首期不用） | `mkelcik/go-ha-client` 内置 auto-reconnect |

### 暂不做（与 future-plan 一致）

- Hearth 直接登录米家账号或直调米家云 API
- 所有米家设备的控制操作（只读监控）
- 为无 HA 用户实现通用 miIO/MIoT 本地兼容（路径 A 作为可选高级配置保留）
- BLE 网关子设备的直连支持（协议复杂，覆盖有限）

---

## 参考链接

- [ysh0566/go-mihome](https://github.com/ysh0566/go-mihome) — 2026 最完整 Go MIoT 库
- [mkelcik/go-ha-client](https://github.com/mkelcik/go-ha-client) — Go HA REST/WS 客户端（v2 稳定）
- [PiotrMachowski/Xiaomi-cloud-tokens-extractor](https://github.com/PiotrMachowski/Xiaomi-cloud-tokens-extractor) — token 提取工具
- [XiaoMi/ha_xiaomi_home](https://github.com/XiaoMi/ha_xiaomi_home) — 小米官方 HA 集成（go-mihome 参考来源）
- [Home Assistant REST API](https://developers.home-assistant.io/docs/api/rest/) — 官方文档
- [Home Assistant WebSocket API](https://developers.home-assistant.io/docs/api/websocket/) — 官方文档
- [OpenMiHome/mihome-binary-protocol](https://github.com/OpenMiHome/mihome-binary-protocol) — miIO 协议规范
- [uole/miio](https://github.com/uole/miio) — 备选轻量 Go miIO 实现（2024 创建）
