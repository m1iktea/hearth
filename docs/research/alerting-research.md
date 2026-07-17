# Hearth 告警功能调研报告

> 日期：2026-07-17  
> 范围：通知渠道选型、规则引擎设计、数据模型、分阶段路线

---

## 1. 背景与现有数据基础

### 现有采集链路

Hearth 已有完整的指标采集-存储链路：

- **采集器**：`server/internal/metrics/` 下 `recorder.go` 驱动 PVE/Docker/OpenWrt 采集，约 10 s 轮询一次。
- **黑匣子存储**：`server/internal/store/blackbox.go` 中的 `metric_samples` 表，字段为 `(source, object, metric, value, created_at)`。
- **已有表**：`metric_samples`（时序指标）、`snapshots`（采集器快照/状态）、`system_events`（系统事件）。
- **现有离线检测**：`QuerySamples` 已可查特定 source/object/metric 在时间窗口内的数据；`uptime_sec` 指标已在 GROUP BY 中使用，说明离线判断的数据基础已就绪。

告警引擎需要的核心能力——"查询某时间窗口内指标是否持续超阈值"——可直接在 `metric_samples` 上用 `MIN/MAX/AVG + GROUP BY` 窗口查询实现，无需引入外部时序数据库。

---

## 2. 通知渠道对比与推荐

### 对比表

| 渠道 | 接入成本 | 消息格式 | 频控限制 | 需要公网 | Go 生态库 | 家用适用性 |
|------|----------|----------|----------|----------|-----------|------------|
| **飞书自定义机器人 Webhook** | 极低（群设置 → 添加机器人，拿 Webhook URL） | 文本/富文本/卡片（interactive） | 100次/分钟，5次/秒；单消息 ≤ 20 KB | 否（出站 HTTPS POST 即可） | 标准库 `net/http` 足够；签名用 `crypto/hmac` | 高（用户主生态） |
| **Telegram Bot** | 低（BotFather 建 bot，`/start` 拿 chat_id） | Markdown/HTML 富文本 | 无硬性全局限制；同群 1 msg/s 建议上限 | 需要出站访问 `api.telegram.org`（在中国大陆需代理） | `github.com/go-telegram/bot` v1.21（最活跃）| 中（网络可达性是主要障碍） |
| **SMTP 邮件** | 中（需要 SMTP 账号配置，163/Gmail 均可） | 纯文本/HTML | 各服务商差异大；无即时性 | 否（出站 SMTP） | `net/smtp` 标准库 | 低（即时性差，易入垃圾箱） |

### 飞书 Webhook 2026 年签名校验机制（已验证）

签名算法：`sign = Base64(HmacSHA256(key="", data=timestamp+"\n"+secret))`，注意 HMAC 的 data 是**空字符串**，key 是 `timestamp+"\n"+secret` 的拼接——这是飞书特有的"反直觉"用法。请求体必须带 `timestamp`（距当前 ≤ 3600 s）和 `sign` 字段。签名过期返回 code 19021。

Go 实现（约 10 行）：

```go
func feishuSign(timestamp int64, secret string) string {
    str := fmt.Sprintf("%d\n%s", timestamp, secret)
    mac := hmac.New(sha256.New, []byte(str))
    return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
```

### 推荐方案

**MVP 首选飞书 Webhook**（用户主生态，无需公网，接入 < 1 小时）。

设计上以**通用 `NotifierChannel` 接口**封装，MVP 只实现飞书；后续可插拔 Telegram、SMTP，无需改业务逻辑。频控（100次/分钟）在家用 HomeLab 场景下几乎不会触发，但需实现去重冷却避免轮询期间重复推送。

---

## 3. 规则引擎设计

参考 Gatus（Go 语言，15 MB 内存，开源）的核心概念，结合 Hearth 的嵌入式单机场景：

### 核心概念

| 概念 | 说明 |
|------|------|
| **AlertRule**（规则） | 一条告警配置：监控对象、指标、阈值、持续时间、通知渠道 |
| **AlertState**（状态机） | 每条规则的运行时状态：`ok` → `pending`（连续违规中）→ `firing`（已告警）→ `ok`（恢复） |
| **EvalLoop**（评估循环） | 定时（与采集器同周期或独立）查询黑匣子，对每条规则评估，驱动状态机转换 |
| **Dedup + Cooldown**（去重冷却） | `firing` 状态下同一规则不重复发送；恢复后重置，并发送恢复通知 |
| **Silence**（静默窗口） | 可选：指定时间段内不发送通知（如夜间维护） |

### 评估循环与现有采集的整合

推荐**独立 goroutine**，每 30 s 或 60 s 运行一次评估循环（不绑定 10 s 采集节奏，避免耦合）。评估时直接查询 `metric_samples` 近 N 分钟窗口数据：

```sql
-- 检查 cpu_usage_pct 是否在过去 5 分钟内持续 > 90%
SELECT MIN(value) FROM metric_samples
WHERE source=? AND object=? AND metric=?
  AND created_at >= datetime('now', '-5 minutes')
```

若 `MIN(value) > threshold`，说明整个窗口内都高于阈值（持续违规），触发状态机转换。

**离线检测**（设备 uptime_sec 长时间未更新）通过查询 `MAX(created_at)` 与当前时间差来判断，无需额外采集。

---

## 4. 数据模型草案

### 新增表

```sql
-- 告警规则
CREATE TABLE IF NOT EXISTS alert_rules (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    enabled     INTEGER NOT NULL DEFAULT 1,
    source      TEXT,           -- 采集源，空=所有
    object      TEXT,           -- 设备对象，空=所有
    metric      TEXT NOT NULL,  -- 指标名，如 cpu_usage_pct
    condition   TEXT NOT NULL,  -- gt/lt/eq
    threshold   REAL NOT NULL,
    for_duration INTEGER NOT NULL DEFAULT 300, -- 持续秒数，默认5分钟
    channel     TEXT NOT NULL,  -- feishu / telegram / smtp
    channel_config TEXT NOT NULL, -- JSON，存 webhook_url + secret
    cooldown_sec INTEGER NOT NULL DEFAULT 3600, -- 冷却时间
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

-- 告警事件（活跃+历史）
CREATE TABLE IF NOT EXISTS alert_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id     INTEGER NOT NULL REFERENCES alert_rules(id),
    state       TEXT NOT NULL,  -- firing / resolved
    fired_at    DATETIME NOT NULL,
    resolved_at DATETIME,
    last_notified_at DATETIME,
    notify_count INTEGER NOT NULL DEFAULT 0,
    summary     TEXT            -- 快照摘要
);
```

### 运行时状态（内存）

```go
type ruleState struct {
    consecutiveFails int
    state            string // ok | pending | firing
    firedAt          time.Time
    lastNotifiedAt   time.Time
}
```

内存状态在进程重启时从 `alert_events` 中最新未 resolved 的记录恢复，保持持久化一致性。

---

## 5. 分阶段落地路线

### Phase 1：MVP（约 1-2 周）

目标：能发飞书通知，不轰炸。

- [ ] 新建 `alert_rules` / `alert_events` 两张表
- [ ] 实现飞书 Webhook 通知器（含签名、发送、错误记录）
- [ ] 实现评估循环（60 s 周期，`metric_samples` 窗口查询）
- [ ] 状态机：ok → pending → firing → resolved（含恢复通知）
- [ ] 去重：同一规则 `firing` 期间按 `cooldown_sec` 控频
- [ ] 在 server 启动时加载规则并启动 goroutine
- [ ] API：`GET/POST/PUT/DELETE /api/alert-rules` 基础 CRUD

验收：CPU 超阈值连续 5 分钟后飞书收到一条告警，恢复后收到一条恢复通知，期间不重复轰炸。

### Phase 2：完整功能（约 2-4 周）

- [ ] 前端规则配置页面（告警规则列表、新增/编辑/删除、测试发送按钮）
- [ ] 离线检测规则（source/object 最后上报时间超阈值）
- [ ] 静默窗口配置（时间段内不发送）
- [ ] 多渠道支持（Telegram Bot 作为第二渠道）
- [ ] 告警历史页（`alert_events` 查询展示）
- [ ] 通知失败重试（最多 3 次，指数退避）

### Phase 3：扩展（按需）

- [ ] SMTP 邮件渠道
- [ ] 自定义告警消息模板（卡片格式）
- [ ] 告警聚合（同时段多条规则合并为一条通知）

---

## 6. 参考链接

- 飞书自定义机器人官方文档：https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot
- 飞书签名校验说明：签名算法为 `HmacSHA256(key=timestamp+"\n"+secret, data="")`，时间戳有效期 3600 s
- Gatus 告警规则设计参考（Go 开源，MIT）：https://github.com/TwiN/gatus
- go-telegram/bot（Telegram Go 库，活跃维护）：https://github.com/go-telegram/bot
