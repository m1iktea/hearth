# 设计：仪表盘资源趋势图表 + 设备导航互通

日期：2026-07-17
状态：已批准

## 背景

- 前端（Vue 3 + Naive UI）目前无图表库，CPU/内存仅以文字百分比展示（`web/src/components/dashboard/InfraSection.vue`）。
- 后端黑匣子已留存时序指标（SQLite `metric_samples` 表），查询接口 `GET /api/v1/metrics?source=&object=&metric=&since=&limit=`，默认 24h 内最多 1000 条、升序。
- `docs/future-plan.md` 三项计划中，本次开发"设备中心与导航页互通"（风险最低），告警与米家接入仅做调研。

## 范围

**开发**：
1. 仪表盘新增"资源趋势"图表区块
2. 设备中心与导航页互通

**调研（只出文档，不写代码）**：
3. 告警功能调研
4. 米家设备接入调研

## 1. 资源趋势区块

### 方案

仪表盘（`DashboardView.vue`）新增独立区块 `TrendSection.vue`，不改动现有 InfraSection 卡片布局。

- **图表内容**：
  - PVE 节点 CPU / 内存趋势（每节点一条序列）
  - Docker 容器 CPU / 内存趋势（按当前 CPU 使用率取 top 5 容器，避免序列爆炸）
  - OpenWrt 内存趋势
- **时间范围切换**：1h / 6h / 24h（映射到 `since` 参数）
- **数据源**：现有 `GET /api/v1/metrics`，前端按序列并发请求，不改后端
- **依赖**：`echarts` + `vue-echarts`，按需注册（LineChart、GridComponent、TooltipComponent、LegendComponent、CanvasRenderer），控制打包体积
- **降级**：某序列无数据时显示空态提示，不阻塞其他图表

### 组件拆分

- `TrendSection.vue`：区块容器，时间范围状态，编排各图表
- `MetricChart.vue`：通用折线图组件（props: 序列定义数组 + 时间范围），内部负责请求 `/api/v1/metrics`、组装 ECharts option
- `web/src/api/metrics.ts`：metrics 接口客户端 + 类型定义

## 2. 设备中心与导航互通

### 数据模型

`nav_items` 表增加可空 `device_id` 列（SQLite `ALTER TABLE ADD COLUMN`，非破坏性迁移）。
规则：一个设备最多关联一个导航项作为默认管理入口（应用层校验，重复关联时报错）。

### API

- nav CRUD 的请求/响应体增加可选 `device_id` 字段；创建/更新时校验设备存在性与唯一关联
- 设备详情接口返回其关联的导航项（如已有）

### 前端

- `DeviceDetailView.vue`：新增"关联导航入口"操作——从现有导航项中选择绑定，或解绑
- `NavView.vue`：已关联设备的导航卡片显示设备在线/离线状态角标（数据来自现有设备台账接口，不新增轮询）

## 3 & 4. 调研文档

由 subagent 联网调研，输出到 `docs/research/`：

- `alerting-research.md`：告警规则引擎设计（阈值/离线检测、去重/静默）、通知渠道对比（飞书 webhook / Telegram / 邮件），给出推荐方案与分阶段落地路径
- `mijia-research.md`：go-miio / miot 局域网协议可行性、设备 token 获取方式、与现有 collector 架构（10s 轮询 + SnapshotStore + 黑匣子）的整合点

## 测试

- 后端：store 迁移与 nav API 单测（含 device_id 校验分支），`go test ./...`
- 前端：`MetricChart` 数据组装逻辑、设备关联组件的 vitest 单测；`npm run test`
- 遵循 TDD：先写失败测试再实现

## 实施方式

subagent 驱动开发：主会话只做编排与验收；探索、实现、code review 均由 subagent 承担，模型按任务复杂度分配（调研/探索 sonnet，机械改动 haiku，核心实现与 review 用主模型）。
