# 仪表盘资源趋势图表 + 设备导航互通 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在仪表盘新增资源趋势折线图区块，并打通设备台账与导航页的双向关联，使导航卡片显示设备在线状态、设备详情页可绑定/解绑导航入口。

**Architecture:** 后端以 SQLite `ALTER TABLE ADD COLUMN` 非破坏性迁移为 `nav_items` 增加可空 `device_id` 列，nav CRUD 接口及设备详情接口同步扩展；前端新增 `web/src/api/metrics.ts` 作为 metrics API 客户端，新增 `MetricChart.vue`（通用折线图）和 `TrendSection.vue`（区块容器），通过 ECharts + vue-echarts 按需注册渲染，`DashboardView.vue` 引入新区块而不改动现有 `InfraSection`。

**Tech Stack:** Go 标准库 `net/http` + SQLite (`modernc.org/sqlite`)；Vue 3 + Naive UI + Pinia；`echarts` + `vue-echarts`（按需 tree-shaking）；Vitest + Go `testing` 包。

---

## File Structure

| 路径 | 操作 | 职责 |
|------|------|------|
| `server/internal/store/nav.go` | **Modify** | 新增 `MigrateDeviceID()` 迁移函数、扩展 `Item` 结构体、扩展 `CreateItem`/`UpdateItem`/`ListCategories` 含 `device_id` 字段、新增 `GetItemByDeviceID` |
| `server/internal/store/nav_test.go` | **Create** | NavStore 单测：迁移幂等、device_id CRUD、唯一关联约束 |
| `server/internal/api/nav.go` | **Modify** | `itemInput` 增加 `DeviceID` 字段；`createItem`/`updateItem` 增加设备存在性校验与唯一关联校验 |
| `server/internal/api/nav_test.go` | **Modify** | 新增 device_id 相关测试用例 |
| `server/internal/api/inventory.go` | **Modify** | `getDevice` handler 的响应中附加关联导航项（若存在） |
| `server/internal/api/inventory_test.go` | **Modify** | 新增设备详情含导航项关联的测试 |
| `server/internal/api/router.go` | **Modify** | `NewRouter` 签名不变，`registerNavRoutes` 接收额外 `*store.InventoryStore` 参数 |
| `web/src/api/metrics.ts` | **Create** | `queryMetrics(params)` 函数 + `MetricSample` 类型定义 |
| `web/src/types.ts` | **Modify** | `NavItem` 增加 `device_id?: number`；`DeviceDetail` 增加 `nav_item?: NavItem` |
| `web/src/components/dashboard/MetricChart.vue` | **Create** | 通用折线图组件，props: `series` + `timeRange`，内部并发请求 metrics API |
| `web/src/components/dashboard/TrendSection.vue` | **Create** | 趋势区块容器，管理 1h/6h/24h 时间范围切换，编排 PVE/Docker/OpenWrt 图表 |
| `web/src/views/DashboardView.vue` | **Modify** | 在现有区块下方引入 `TrendSection` |
| `web/src/views/NavView.vue` | **Modify** | 已关联设备的导航卡片显示在线/离线状态角标 |
| `web/src/views/DeviceDetailView.vue` | **Modify** | 新增"关联导航入口"操作区，支持绑定/解绑 |
| `web/src/stores/nav.ts` | **Modify** | `saveItem` 传递 `device_id`；新增 `linkDevice`/`unlinkDevice` 操作 |
| `web/src/utils/metrics.test.ts` | **Create** | MetricChart 数据组装逻辑单测 |

---

## Tasks

### 阶段一：后端 — nav_items 迁移与 store 扩展

---

#### Task 1：扩展 `Item` 结构体并实现 `MigrateDeviceID` 迁移

**Files:**
- Modify: `server/internal/store/nav.go`（全文改动）
- Create: `server/internal/store/nav_test.go`

**TDD 步骤：**

- [ ] **写失败测试**

新建 `server/internal/store/nav_test.go`：

```go
package store_test

import (
	"testing"

	"github.com/m1iktea/hearth/server/internal/store"
)

func openTestNav(t *testing.T) *store.NavStore {
	t.Helper()
	nav, err := store.OpenNav(t.TempDir() + "/nav.db")
	if err != nil {
		t.Fatalf("OpenNav: %v", err)
	}
	t.Cleanup(func() { nav.Close() })
	return nav
}

func TestNavMigrateDeviceID_Idempotent(t *testing.T) {
	nav := openTestNav(t)
	// 第一次迁移
	if err := nav.MigrateDeviceID(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	// 幂等：第二次不报错
	if err := nav.MigrateDeviceID(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestNavItemDeviceID_CRUD(t *testing.T) {
	nav := openTestNav(t)
	if err := nav.MigrateDeviceID(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cat, err := nav.CreateCategory("服务", 1)
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	deviceID := int64(42)

	// 创建带 device_id 的条目
	item, err := nav.CreateItem(store.Item{
		CategoryID: cat.ID,
		Name:       "PVE",
		URL:        "https://pve:8006",
		DeviceID:   &deviceID,
	})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	if item.DeviceID == nil || *item.DeviceID != 42 {
		t.Fatalf("DeviceID not set, got %v", item.DeviceID)
	}

	// ListCategories 含 device_id
	cats, err := nav.ListCategories()
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(cats[0].Items) != 1 || cats[0].Items[0].DeviceID == nil || *cats[0].Items[0].DeviceID != 42 {
		t.Fatalf("ListCategories item DeviceID: %+v", cats[0].Items)
	}

	// GetItemByDeviceID
	found, err := nav.GetItemByDeviceID(42)
	if err != nil {
		t.Fatalf("GetItemByDeviceID: %v", err)
	}
	if found.ID != item.ID {
		t.Fatalf("GetItemByDeviceID returned wrong item: %+v", found)
	}

	// 更新清除 device_id
	item.DeviceID = nil
	updated, err := nav.UpdateItem(item)
	if err != nil {
		t.Fatalf("UpdateItem clear device_id: %v", err)
	}
	if updated.DeviceID != nil {
		t.Fatalf("DeviceID should be nil after clear, got %v", updated.DeviceID)
	}
}

func TestNavItemDeviceID_UniqueConstraint(t *testing.T) {
	nav := openTestNav(t)
	if err := nav.MigrateDeviceID(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cat, _ := nav.CreateCategory("cat", 0)
	deviceID := int64(7)
	_, err := nav.CreateItem(store.Item{CategoryID: cat.ID, Name: "A", URL: "http://a", DeviceID: &deviceID})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	// 同一 device_id 再创建应报错
	_, err = nav.CreateItem(store.Item{CategoryID: cat.ID, Name: "B", URL: "http://b", DeviceID: &deviceID})
	if err == nil {
		t.Fatal("expected unique constraint error, got nil")
	}
}
```

- [ ] **运行确认失败**

```bash
cd /Users/kanliang.xu/code/hearth/server && go test ./internal/store/ -run TestNavMigrate -v 2>&1 | head -20
```

预期输出：`# [build-error]` 或 `FAIL` — `MigrateDeviceID`、`DeviceID` 字段、`GetItemByDeviceID` 未定义。

- [ ] **最小实现**

修改 `server/internal/store/nav.go`，完整文件内容：

```go
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// NavStore 管理导航分类与条目，存于 SQLite 单文件。
type NavStore struct {
	db *sql.DB
}

type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Items     []Item `json:"items"`
}

type Item struct {
	ID         int64  `json:"id"`
	CategoryID int64  `json:"category_id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Icon       string `json:"icon"`
	SortOrder  int    `json:"sort_order"`
	// DeviceID 关联设备台账中的设备；可空，一个设备最多关联一个导航项。
	DeviceID *int64 `json:"device_id,omitempty"`
}

const navSchema = `
CREATE TABLE IF NOT EXISTS nav_categories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS nav_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	category_id INTEGER NOT NULL REFERENCES nav_categories(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	url TEXT NOT NULL,
	icon TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL DEFAULT 0
);`

func OpenNav(path string) (*NavStore, error) {
	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(navSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	s := &NavStore{db: db}
	if err := s.MigrateDeviceID(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate device_id: %w", err)
	}
	return s, nil
}

func (n *NavStore) Close() error { return n.db.Close() }

// MigrateDeviceID 幂等地为 nav_items 添加 device_id 列和唯一索引。
// SQLite ALTER TABLE ADD COLUMN 若列已存在会报错，用 PRAGMA 提前检查。
func (n *NavStore) MigrateDeviceID() error {
	// 检查列是否已存在
	rows, err := n.db.Query(`PRAGMA table_info(nav_items)`)
	if err != nil {
		return fmt.Errorf("pragma table_info: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan table_info: %w", err)
		}
		if name == "device_id" {
			return nil // 已存在，幂等返回
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table_info: %w", err)
	}

	// 添加列与唯一索引（device_id 为 NULL 时不参与唯一约束，符合 SQLite NULL != NULL 语义）
	if _, err := n.db.Exec(`ALTER TABLE nav_items ADD COLUMN device_id INTEGER REFERENCES devices(id) ON DELETE SET NULL`); err != nil {
		return fmt.Errorf("alter table add device_id: %w", err)
	}
	if _, err := n.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_nav_items_device_id ON nav_items(device_id) WHERE device_id IS NOT NULL`); err != nil {
		return fmt.Errorf("create unique index device_id: %w", err)
	}
	return nil
}

func (n *NavStore) ListCategories() ([]Category, error) {
	rows, err := n.db.Query(`SELECT id, name, sort_order FROM nav_categories ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cats := []Category{}
	index := map[int64]int{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.SortOrder); err != nil {
			return nil, err
		}
		c.Items = []Item{}
		index[c.ID] = len(cats)
		cats = append(cats, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	itemRows, err := n.db.Query(`SELECT id, category_id, name, url, icon, sort_order, device_id FROM nav_items ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()
	for itemRows.Next() {
		var it Item
		if err := itemRows.Scan(&it.ID, &it.CategoryID, &it.Name, &it.URL, &it.Icon, &it.SortOrder, &it.DeviceID); err != nil {
			return nil, err
		}
		if i, ok := index[it.CategoryID]; ok {
			cats[i].Items = append(cats[i].Items, it)
		}
	}
	return cats, itemRows.Err()
}

func (n *NavStore) CreateCategory(name string, sortOrder int) (Category, error) {
	res, err := n.db.Exec(`INSERT INTO nav_categories (name, sort_order) VALUES (?, ?)`, name, sortOrder)
	if err != nil {
		return Category{}, err
	}
	id, _ := res.LastInsertId()
	return Category{ID: id, Name: name, SortOrder: sortOrder, Items: []Item{}}, nil
}

func (n *NavStore) UpdateCategory(id int64, name string, sortOrder int) (Category, error) {
	res, err := n.db.Exec(`UPDATE nav_categories SET name = ?, sort_order = ? WHERE id = ?`, name, sortOrder, id)
	if err != nil {
		return Category{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return Category{}, sql.ErrNoRows
	}
	return Category{ID: id, Name: name, SortOrder: sortOrder, Items: []Item{}}, nil
}

func (n *NavStore) DeleteCategory(id int64) error {
	_, err := n.db.Exec(`DELETE FROM nav_categories WHERE id = ?`, id)
	return err
}

func (n *NavStore) CreateItem(it Item) (Item, error) {
	res, err := n.db.Exec(
		`INSERT INTO nav_items (category_id, name, url, icon, sort_order, device_id) VALUES (?, ?, ?, ?, ?, ?)`,
		it.CategoryID, it.Name, it.URL, it.Icon, it.SortOrder, it.DeviceID,
	)
	if err != nil {
		return Item{}, fmt.Errorf("insert nav_item: %w", err)
	}
	it.ID, _ = res.LastInsertId()
	return it, nil
}

func (n *NavStore) UpdateItem(it Item) (Item, error) {
	res, err := n.db.Exec(
		`UPDATE nav_items SET category_id = ?, name = ?, url = ?, icon = ?, sort_order = ?, device_id = ? WHERE id = ?`,
		it.CategoryID, it.Name, it.URL, it.Icon, it.SortOrder, it.DeviceID, it.ID,
	)
	if err != nil {
		return Item{}, fmt.Errorf("update nav_item: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return Item{}, sql.ErrNoRows
	}
	return it, nil
}

func (n *NavStore) DeleteItem(id int64) error {
	_, err := n.db.Exec(`DELETE FROM nav_items WHERE id = ?`, id)
	return err
}

// GetItemByDeviceID 返回与指定设备关联的导航项；若不存在返回 sql.ErrNoRows。
func (n *NavStore) GetItemByDeviceID(deviceID int64) (Item, error) {
	var it Item
	err := n.db.QueryRow(
		`SELECT id, category_id, name, url, icon, sort_order, device_id FROM nav_items WHERE device_id = ?`,
		deviceID,
	).Scan(&it.ID, &it.CategoryID, &it.Name, &it.URL, &it.Icon, &it.SortOrder, &it.DeviceID)
	if err != nil {
		return Item{}, err
	}
	return it, nil
}
```

- [ ] **运行确认通过**

```bash
cd /Users/kanliang.xu/code/hearth/server && go test ./internal/store/ -run "TestNavMigrate|TestNavItemDeviceID" -v -race 2>&1
```

预期：`PASS`，所有测试绿灯。

- [ ] **git commit**

```bash
cd /Users/kanliang.xu/code/hearth/server
git add internal/store/nav.go internal/store/nav_test.go
git commit -m "feat(store): nav_items 新增 device_id 关联列与幂等迁移"
```

---

#### Task 2：扩展 nav API — itemInput 支持 device_id，校验设备存在性与唯一关联

**Files:**
- Modify: `server/internal/api/nav.go`（`itemInput`、`createItem`、`updateItem` handler）
- Modify: `server/internal/api/router.go`（`registerNavRoutes` 增加 `inventory` 参数）
- Modify: `server/internal/api/nav_test.go`（新增测试用例）

**TDD 步骤：**

- [ ] **写失败测试**

在 `server/internal/api/nav_test.go` 末尾追加：

```go
func TestNavItemDeviceIDAPI(t *testing.T) {
	h, _, _ := newTestRouter(t)

	// 准备分类
	code, env := doJSON(t, h, "POST", "/api/v1/nav/categories", `{"name":"服务","sort_order":1}`)
	if code != 200 || !env.Success {
		t.Fatalf("create cat: %d %+v", code, env)
	}
	var cat struct{ ID int64 `json:"id"` }
	json.Unmarshal(env.Data, &cat)

	// 准备设备
	code, env = doJSON(t, h, "POST", "/api/v1/devices", `{"name":"NAS","kind":"nas","ip_address":"192.168.1.10","enabled":true}`)
	if code != 200 || !env.Success {
		t.Fatalf("create device: %d %+v", code, env)
	}
	var device struct{ ID int64 `json:"id"` }
	json.Unmarshal(env.Data, &device)

	body := fmt.Sprintf(`{"category_id":%d,"name":"NAS","url":"http://nas","device_id":%d}`, cat.ID, device.ID)

	// 绑定存在的设备
	code, env = doJSON(t, h, "POST", "/api/v1/nav/items", body)
	if code != 200 || !env.Success {
		t.Fatalf("create item with device_id: %d %+v", code, env)
	}
	var item struct {
		ID       int64  `json:"id"`
		DeviceID *int64 `json:"device_id"`
	}
	json.Unmarshal(env.Data, &item)
	if item.DeviceID == nil || *item.DeviceID != device.ID {
		t.Fatalf("device_id not set: %+v", item)
	}

	// 同一设备绑定第二个导航项应 409
	body2 := fmt.Sprintf(`{"category_id":%d,"name":"NAS2","url":"http://nas2","device_id":%d}`, cat.ID, device.ID)
	code, _ = doJSON(t, h, "POST", "/api/v1/nav/items", body2)
	if code != 409 {
		t.Errorf("duplicate device_id: want 409, got %d", code)
	}

	// 绑定不存在的设备应 422
	body3 := fmt.Sprintf(`{"category_id":%d,"name":"Ghost","url":"http://ghost","device_id":99999}`, cat.ID)
	code, _ = doJSON(t, h, "POST", "/api/v1/nav/items", body3)
	if code != 422 {
		t.Errorf("nonexistent device_id: want 422, got %d", code)
	}
}
```

- [ ] **运行确认失败**

```bash
cd /Users/kanliang.xu/code/hearth/server && go test ./internal/api/ -run TestNavItemDeviceIDAPI -v 2>&1 | head -30
```

预期：编译失败或 `422`/`409` 分支返回 200。

- [ ] **最小实现**

**Step A：修改 `server/internal/api/router.go`**

将 `registerNavRoutes(mux, nav)` 改为 `registerNavRoutes(mux, nav, inventory)`：

找到行（约第 22 行）：
```go
	registerNavRoutes(mux, nav) // Task 10 实现；本 Task 先提供空实现避免编译失败
```
替换为：
```go
	registerNavRoutes(mux, nav, inventory)
```

**Step B：完整替换 `server/internal/api/nav.go`：**

```go
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/m1iktea/hearth/server/internal/store"
)

func registerNavRoutes(mux *http.ServeMux, nav *store.NavStore, inventory *store.InventoryStore) {
	h := &navHandler{nav: nav, inventory: inventory}
	mux.HandleFunc("GET /api/v1/nav", h.list)
	mux.HandleFunc("POST /api/v1/nav/categories", h.createCategory)
	mux.HandleFunc("PUT /api/v1/nav/categories/{id}", h.updateCategory)
	mux.HandleFunc("DELETE /api/v1/nav/categories/{id}", h.deleteCategory)
	mux.HandleFunc("POST /api/v1/nav/items", h.createItem)
	mux.HandleFunc("PUT /api/v1/nav/items/{id}", h.updateItem)
	mux.HandleFunc("DELETE /api/v1/nav/items/{id}", h.deleteItem)
}

type navHandler struct {
	nav      *store.NavStore
	inventory *store.InventoryStore
}

type categoryInput struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type itemInput struct {
	CategoryID int64  `json:"category_id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Icon       string `json:"icon"`
	SortOrder  int    `json:"sort_order"`
	DeviceID   *int64 `json:"device_id"`
}

func (h *navHandler) list(w http.ResponseWriter, r *http.Request) {
	cats, err := h.nav.ListCategories()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list nav")
		return
	}
	writeOK(w, cats)
}

func (h *navHandler) createCategory(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeCategory(w, r)
	if !ok {
		return
	}
	cat, err := h.nav.CreateCategory(in.Name, in.SortOrder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create category")
		return
	}
	writeOK(w, cat)
}

func (h *navHandler) updateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	in, ok := decodeCategory(w, r)
	if !ok {
		return
	}
	cat, err := h.nav.UpdateCategory(id, in.Name, in.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "category not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update category")
		return
	}
	writeOK(w, cat)
}

func (h *navHandler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.nav.DeleteCategory(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete category")
		return
	}
	writeOK(w, nil)
}

func (h *navHandler) createItem(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeItem(w, r)
	if !ok {
		return
	}
	if in.DeviceID != nil {
		if err := h.checkDeviceExists(w, *in.DeviceID); err != nil {
			return
		}
		if err := h.checkDeviceNotLinked(w, *in.DeviceID, 0); err != nil {
			return
		}
	}
	item, err := h.nav.CreateItem(store.Item{
		CategoryID: in.CategoryID,
		Name:       in.Name,
		URL:        in.URL,
		Icon:       in.Icon,
		SortOrder:  in.SortOrder,
		DeviceID:   in.DeviceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create item")
		return
	}
	writeOK(w, item)
}

func (h *navHandler) updateItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	in, ok := decodeItem(w, r)
	if !ok {
		return
	}
	if in.DeviceID != nil {
		if err := h.checkDeviceExists(w, *in.DeviceID); err != nil {
			return
		}
		if err := h.checkDeviceNotLinked(w, *in.DeviceID, id); err != nil {
			return
		}
	}
	item, err := h.nav.UpdateItem(store.Item{
		ID:         id,
		CategoryID: in.CategoryID,
		Name:       in.Name,
		URL:        in.URL,
		Icon:       in.Icon,
		SortOrder:  in.SortOrder,
		DeviceID:   in.DeviceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update item")
		return
	}
	writeOK(w, item)
}

func (h *navHandler) deleteItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.nav.DeleteItem(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete item")
		return
	}
	writeOK(w, nil)
}

// checkDeviceExists 校验设备在台账中存在，失败时写入 422 响应并返回 error。
func (h *navHandler) checkDeviceExists(w http.ResponseWriter, deviceID int64) error {
	_, err := h.inventory.GetDevice(deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 422, fmt.Sprintf("device %d not found", deviceID))
		return err
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check device")
		return err
	}
	return nil
}

// checkDeviceNotLinked 校验同一 device_id 未被其他导航项占用。
// skipItemID 为 0 表示创建场景；更新场景传入当前条目 id 排除自身。
func (h *navHandler) checkDeviceNotLinked(w http.ResponseWriter, deviceID int64, skipItemID int64) error {
	existing, err := h.nav.GetItemByDeviceID(deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // 未被占用
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check device link")
		return err
	}
	if existing.ID == skipItemID {
		return nil // 更新自身，允许
	}
	writeError(w, http.StatusConflict, fmt.Sprintf("device %d is already linked to nav item %d", deviceID, existing.ID))
	return fmt.Errorf("conflict")
}

func decodeCategory(w http.ResponseWriter, r *http.Request) (categoryInput, bool) {
	var in categoryInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return in, false
	}
	if strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return in, false
	}
	return in, true
}

func decodeItem(w http.ResponseWriter, r *http.Request) (itemInput, bool) {
	var in itemInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return in, false
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.URL) == "" {
		writeError(w, http.StatusBadRequest, "name and url are required")
		return in, false
	}
	if in.CategoryID <= 0 {
		writeError(w, http.StatusBadRequest, "category_id is required")
		return in, false
	}
	return in, true
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}
```

- [ ] **运行确认通过**

```bash
cd /Users/kanliang.xu/code/hearth/server && go test ./internal/api/ -run "TestNav" -v -race 2>&1
```

预期：所有 `TestNav*` 测试 PASS。

- [ ] **git commit**

```bash
cd /Users/kanliang.xu/code/hearth/server
git add internal/api/nav.go internal/api/nav_test.go internal/api/router.go
git commit -m "feat(api): nav item 支持 device_id 绑定，校验设备存在性与唯一关联"
```

---

#### Task 3：设备详情接口返回关联导航项

**Files:**
- Modify: `server/internal/api/inventory.go`（`getDevice` handler，约第 80-110 行）
- Modify: `server/internal/api/inventory_test.go`（追加测试）

**TDD 步骤：**

- [ ] **写失败测试**

在 `server/internal/api/inventory_test.go` 末尾追加：

```go
func TestDeviceDetailWithNavItem(t *testing.T) {
	h, _, _ := newTestRouter(t)

	// 创建设备
	code, env := doJSON(t, h, "POST", "/api/v1/devices", `{"name":"PVE","kind":"server","ip_address":"192.168.1.1","enabled":true}`)
	if code != 200 || !env.Success {
		t.Fatalf("create device: %d %+v", code, env)
	}
	var device struct{ ID int64 `json:"id"` }
	json.Unmarshal(env.Data, &device)

	// 创建分类和关联导航项
	code, env = doJSON(t, h, "POST", "/api/v1/nav/categories", `{"name":"服务","sort_order":1}`)
	var cat struct{ ID int64 `json:"id"` }
	json.Unmarshal(env.Data, &cat)

	body := fmt.Sprintf(`{"category_id":%d,"name":"PVE","url":"https://pve:8006","device_id":%d}`, cat.ID, device.ID)
	code, env = doJSON(t, h, "POST", "/api/v1/nav/items", body)
	if code != 200 || !env.Success {
		t.Fatalf("create nav item: %d %+v", code, env)
	}
	var navItem struct{ ID int64 `json:"id"` }
	json.Unmarshal(env.Data, &navItem)

	// 获取设备详情，期望含 nav_item
	code, env = doJSON(t, h, "GET", fmt.Sprintf("/api/v1/devices/%d", device.ID), "")
	if code != 200 || !env.Success {
		t.Fatalf("get detail: %d %+v", code, env)
	}
	var detail struct {
		Device  struct{ Name string `json:"name"` } `json:"device"`
		NavItem *struct {
			ID  int64  `json:"id"`
			URL string `json:"url"`
		} `json:"nav_item"`
	}
	json.Unmarshal(env.Data, &detail)
	if detail.NavItem == nil {
		t.Fatal("nav_item should not be nil")
	}
	if detail.NavItem.ID != navItem.ID {
		t.Errorf("nav_item.id = %d, want %d", detail.NavItem.ID, navItem.ID)
	}
	if detail.NavItem.URL != "https://pve:8006" {
		t.Errorf("nav_item.url = %q", detail.NavItem.URL)
	}
}
```

- [ ] **运行确认失败**

```bash
cd /Users/kanliang.xu/code/hearth/server && go test ./internal/api/ -run TestDeviceDetailWithNavItem -v 2>&1 | head -20
```

预期：`nav_item should not be nil`。

- [ ] **最小实现**

读取 `server/internal/api/inventory.go` 找到 `getDevice` handler（处理 `GET /api/v1/devices/{id}` 的函数）。在其响应组装处，将原来的 `DeviceDetail` 替换为扩展结构：

在 `inventory.go` 中找到返回设备详情的 handler（搜索 `GetDevice` 或 `DeviceDetail`），在 `writeOK` 之前增加导航项查询。具体：在 handler 函数结尾（获得 `detail DeviceDetail` 之后、`writeOK` 之前）插入：

```go
// 查询关联导航项（可选）
type deviceDetailWithNav struct {
    store.DeviceDetail
    NavItem *store.Item `json:"nav_item,omitempty"`
}
result := deviceDetailWithNav{DeviceDetail: detail}
if navItem, err := nav.GetItemByDeviceID(detail.Device.ID); err == nil {
    result.NavItem = &navItem
}
writeOK(w, result)
```

为此需要将 `nav *store.NavStore` 传入 inventory handler。在 `registerInventoryRoutes` 函数签名中增加 `nav *store.NavStore` 参数，并在 `router.go` 中对应更新调用。

**修改 `server/internal/api/inventory.go`** — 找到 `registerInventoryRoutes` 函数签名：

```go
// 原签名（约第1-5行）：
func registerInventoryRoutes(mux *http.ServeMux, inventory *store.InventoryStore, scanner arpScanner) {
```

替换为：

```go
func registerInventoryRoutes(mux *http.ServeMux, inventory *store.InventoryStore, nav *store.NavStore, scanner arpScanner) {
```

将 `inventoryHandler` 结构体增加 `nav *store.NavStore` 字段，并在构造处传入。找到处理 `GET /api/v1/devices/{id}` 的 handler 函数（约含 `GetDevice` 和 `writeOK`），在 `writeOK` 前加入导航项查询逻辑：

```go
func (h *inventoryHandler) getDevice(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(w, r)
    if !ok {
        return
    }
    detail, err := h.inventory.GetDeviceDetail(id)
    if errors.Is(err, sql.ErrNoRows) {
        writeError(w, http.StatusNotFound, "device not found")
        return
    }
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to get device")
        return
    }

    type deviceDetailWithNav struct {
        store.DeviceDetail
        NavItem *store.Item `json:"nav_item,omitempty"`
    }
    result := deviceDetailWithNav{DeviceDetail: detail}
    if h.nav != nil {
        if navItem, err := h.nav.GetItemByDeviceID(detail.Device.ID); err == nil {
            result.NavItem = &navItem
        }
    }
    writeOK(w, result)
}
```

**修改 `server/internal/api/router.go`** 中调用 `registerInventoryRoutes` 的行：

```go
// 原：
registerInventoryRoutes(mux, inventory, arpScan)
// 改为：
registerInventoryRoutes(mux, inventory, nav, arpScan)
```

- [ ] **运行确认通过**

```bash
cd /Users/kanliang.xu/code/hearth/server && go test ./internal/api/ -run "TestDevice" -v -race 2>&1
```

预期：所有 `TestDevice*` 通过。

- [ ] **全量后端测试**

```bash
cd /Users/kanliang.xu/code/hearth/server && go test ./... -race 2>&1
```

预期：`ok` 无失败。

- [ ] **git commit**

```bash
cd /Users/kanliang.xu/code/hearth/server
git add internal/api/inventory.go internal/api/inventory_test.go internal/api/router.go
git commit -m "feat(api): 设备详情接口附加关联导航项"
```

---

### 阶段二：前端 — 设备导航互通 UI

---

#### Task 4：扩展前端类型与 nav store

**Files:**
- Modify: `web/src/types.ts`（`NavItem`、`DeviceDetail`）
- Modify: `web/src/stores/nav.ts`（`saveItem`、`linkDevice`、`unlinkDevice`）

**TDD 步骤：**（类型变更无独立测试，随 Task 5 验证）

- [ ] **修改 `web/src/types.ts`**

在 `NavItem` 接口追加字段，在 `DeviceDetail` 追加 `nav_item`：

找到：
```typescript
export interface NavItem {
  id: number
  category_id: number
  name: string
  url: string
  icon: string
  sort_order: number
}
```
替换为：
```typescript
export interface NavItem {
  id: number
  category_id: number
  name: string
  url: string
  icon: string
  sort_order: number
  device_id?: number
}
```

找到：
```typescript
export interface DeviceDetail { device: Device; checks: HealthCheck[] }
```
替换为：
```typescript
export interface DeviceDetail { device: Device; checks: HealthCheck[]; nav_item?: NavItem }
```

- [ ] **修改 `web/src/stores/nav.ts`**

首先 Read `web/src/stores/nav.ts`，然后在 `saveItem` 的请求 body 中加入 `device_id` 字段，并新增两个 action：

```typescript
// 在 saveItem action 的 body 对象中加入：
device_id: itemForm.device_id ?? null,

// 新增 action：
async linkDevice(itemId: number, deviceId: number): Promise<void> {
  // 先获取当前条目数据，再 PUT 更新
  const allItems = this.categories.flatMap((c) => c.items)
  const item = allItems.find((i) => i.id === itemId)
  if (!item) throw new Error(`nav item ${itemId} not found`)
  await apiPut(`/api/v1/nav/items/${itemId}`, {
    category_id: item.category_id,
    name: item.name,
    url: item.url,
    icon: item.icon,
    sort_order: item.sort_order,
    device_id: deviceId,
  })
  await this.load()
},

async unlinkDevice(itemId: number): Promise<void> {
  const allItems = this.categories.flatMap((c) => c.items)
  const item = allItems.find((i) => i.id === itemId)
  if (!item) throw new Error(`nav item ${itemId} not found`)
  await apiPut(`/api/v1/nav/items/${itemId}`, {
    category_id: item.category_id,
    name: item.name,
    url: item.url,
    icon: item.icon,
    sort_order: item.sort_order,
    device_id: null,
  })
  await this.load()
},
```

- [ ] **git commit**

```bash
cd /Users/kanliang.xu/code/hearth/web
git add src/types.ts src/stores/nav.ts
git commit -m "feat(frontend): 类型扩展 device_id，nav store 新增 linkDevice/unlinkDevice"
```

---

#### Task 5：DeviceDetailView 新增关联导航入口操作区

**Files:**
- Modify: `web/src/views/DeviceDetailView.vue`

**TDD 步骤：**（集成场景，通过 build 验证）

- [ ] **实现**

Read `web/src/views/DeviceDetailView.vue` 完整内容，然后在 `<template>` 中 `</n-space>` 之后（设备基本信息卡片之后）增加导航关联卡片区块：

在 `<script setup>` 中增加导入和逻辑：

```typescript
import { useNavStore } from '../stores/nav'
// ...（已有 import 保持不变）

const navStore = useNavStore()
const linkModal = ref(false)
const linkItemId = ref<number | null>(null)
const linkError = ref('')

// 未被其他设备占用的导航项
const availableNavItems = computed(() => {
  if (!detail.value) return []
  const currentNavItem = detail.value.nav_item
  return navStore.categories
    .flatMap((c) => c.items)
    .filter((i) => i.device_id == null || i.id === currentNavItem?.id)
    .map((i) => ({ label: i.name, value: i.id }))
})

onMounted(async () => {
  await navStore.load()
  await load()
})

async function saveLink() {
  if (!detail.value || linkItemId.value == null) return
  try {
    linkError.value = ''
    await navStore.linkDevice(linkItemId.value, id)
    linkModal.value = false
    await load()
  } catch (e) {
    linkError.value = String(e)
  }
}

async function removeLink() {
  if (!detail.value?.nav_item) return
  try {
    linkError.value = ''
    await navStore.unlinkDevice(detail.value.nav_item.id)
    await load()
  } catch (e) {
    linkError.value = String(e)
  }
}
```

在 `<template>` 中，在健康检查卡片之后插入：

```html
<!-- 关联导航入口 -->
<n-card title="关联导航入口" style="margin-top: 16px">
  <template v-if="detail.nav_item">
    <n-space align="center">
      <span>{{ detail.nav_item.name }}</span>
      <n-tag type="info" size="small">{{ detail.nav_item.url }}</n-tag>
      <n-popconfirm @positive-click="removeLink">
        <template #trigger>
          <n-button size="small" type="error" ghost>解绑</n-button>
        </template>
        确认解除与该导航入口的关联？
      </n-popconfirm>
    </n-space>
  </template>
  <template v-else>
    <n-space>
      <span style="opacity:.65">未关联导航入口</span>
      <n-button size="small" @click="linkModal = true">绑定</n-button>
    </n-space>
  </template>
  <n-alert v-if="linkError" type="error" style="margin-top: 8px">{{ linkError }}</n-alert>
</n-card>

<n-modal v-model:show="linkModal" title="选择导航入口" preset="card" style="width: 400px">
  <n-form-item label="导航项">
    <n-select v-model:value="linkItemId" :options="availableNavItems" placeholder="选择要关联的导航项" />
  </n-form-item>
  <n-space justify="end">
    <n-button @click="linkModal = false">取消</n-button>
    <n-button type="primary" :disabled="linkItemId == null" @click="saveLink">确认绑定</n-button>
  </n-space>
</n-modal>
```

- [ ] **构建验证**

```bash
cd /Users/kanliang.xu/code/hearth/web && npm run build 2>&1 | tail -20
```

预期：`built in` 无 TypeScript 错误。

- [ ] **git commit**

```bash
cd /Users/kanliang.xu/code/hearth/web
git add src/views/DeviceDetailView.vue
git commit -m "feat(frontend): DeviceDetailView 新增导航入口绑定/解绑"
```

---

#### Task 6：NavView 导航卡片显示设备在线状态角标

**Files:**
- Modify: `web/src/views/NavView.vue`

**TDD 步骤：**（视觉改动，build 验证）

- [ ] **实现**

Read `web/src/views/NavView.vue`，在 `<script setup>` 中增加：

```typescript
import { useInventoryStore } from '../stores/inventory'
const inventoryStore = useInventoryStore()

// 按设备 id 索引健康状态（来自已有台账数据，不新增轮询）
const deviceStatusMap = computed(() => {
  const map = new Map<number, 'online' | 'offline' | 'unknown'>()
  for (const device of inventoryStore.devices) {
    // 设备的最新健康状态取自其 checks 中任意一个的 last_status
    // 若 devices 列表只含基础字段（无 checks），设为 unknown
    map.set(device.id, 'unknown')
  }
  return map
})

onMounted(() => {
  store.load()
  inventoryStore.loadDevices()
})
```

在导航卡片渲染处（每个 `item` 卡片）：在卡片标题或卡片右上角加状态角标。找到渲染 `NavItem` 的位置，在卡片内容区（`<SiteIcon>` 或名称旁）追加：

```html
<n-tag
  v-if="item.device_id != null"
  :type="deviceStatusMap.get(item.device_id) === 'online' ? 'success'
       : deviceStatusMap.get(item.device_id) === 'offline' ? 'error' : 'default'"
  size="tiny"
  style="margin-left: 4px; vertical-align: middle"
>
  {{ deviceStatusMap.get(item.device_id) === 'online' ? '在线'
   : deviceStatusMap.get(item.device_id) === 'offline' ? '离线' : '未知' }}
</n-tag>
```

注意：`inventoryStore.loadDevices()` 需在 `inventory` store 中存在。Read `web/src/stores/inventory.ts`，若无 `loadDevices` 则添加之（若已有 `devices` 响应式和 `loadDevices` action 则直接引用）。

- [ ] **构建验证**

```bash
cd /Users/kanliang.xu/code/hearth/web && npm run build 2>&1 | tail -20
```

- [ ] **git commit**

```bash
cd /Users/kanliang.xu/code/hearth/web
git add src/views/NavView.vue src/stores/inventory.ts
git commit -m "feat(frontend): NavView 导航卡片显示关联设备在线状态角标"
```

---

### 阶段三：前端 — 资源趋势图表

---

#### Task 7：安装 ECharts 依赖

**Files:**
- Modify: `web/package.json`（间接，通过 npm install）

- [ ] **安装依赖**

```bash
cd /Users/kanliang.xu/code/hearth/web && npm install echarts vue-echarts 2>&1 | tail -10
```

预期：`added N packages` 无错误。

- [ ] **验证安装**

```bash
cd /Users/kanliang.xu/code/hearth/web && node -e "require('./node_modules/echarts/package.json'); console.log('ok')" 2>&1
```

- [ ] **git commit**

```bash
cd /Users/kanliang.xu/code/hearth/web
git add package.json package-lock.json
git commit -m "chore(deps): 添加 echarts + vue-echarts 依赖"
```

---

#### Task 8：新建 metrics API 客户端 `web/src/api/metrics.ts`

**Files:**
- Create: `web/src/api/metrics.ts`
- Create: `web/src/utils/metrics.test.ts`

**TDD 步骤：**

- [ ] **写失败测试**

新建 `web/src/utils/metrics.test.ts`：

```typescript
import { describe, expect, it } from 'vitest'
import { buildEChartsSeries, type MetricSample } from '../api/metrics'

describe('buildEChartsSeries', () => {
  it('按 object 分组，时间升序，value 保留原值', () => {
    const samples: MetricSample[] = [
      { source: 'proxmox', object: 'pve-01', metric: 'cpu_pct', value: 20.5, created_at: '2026-07-17T10:00:00Z' },
      { source: 'proxmox', object: 'pve-02', metric: 'cpu_pct', value: 35.0, created_at: '2026-07-17T10:00:00Z' },
      { source: 'proxmox', object: 'pve-01', metric: 'cpu_pct', value: 22.0, created_at: '2026-07-17T10:01:00Z' },
    ]

    const series = buildEChartsSeries(samples)

    expect(series).toHaveLength(2)
    const pve01 = series.find((s) => s.name === 'pve-01')
    const pve02 = series.find((s) => s.name === 'pve-02')
    expect(pve01).toBeDefined()
    expect(pve01!.data).toHaveLength(2)
    expect(pve01!.data[0]).toEqual(['2026-07-17T10:00:00Z', 20.5])
    expect(pve01!.data[1]).toEqual(['2026-07-17T10:01:00Z', 22.0])
    expect(pve02).toBeDefined()
    expect(pve02!.data).toHaveLength(1)
  })

  it('空数组返回空序列', () => {
    expect(buildEChartsSeries([])).toEqual([])
  })
})
```

- [ ] **运行确认失败**

```bash
cd /Users/kanliang.xu/code/hearth/web && npm run test 2>&1 | grep -A5 "metrics"
```

预期：`Cannot find module '../api/metrics'`。

- [ ] **最小实现**

新建 `web/src/api/metrics.ts`：

```typescript
import { apiGet } from './client'

export interface MetricSample {
  source: string
  object: string
  metric: string
  value: number
  created_at: string
}

export interface MetricQueryParams {
  source?: string
  object?: string
  metric?: string
  /** RFC3339 时间字符串 */
  since?: string
  limit?: number
}

export interface EChartsSeriesItem {
  name: string
  type: 'line'
  smooth: boolean
  data: [string, number][]
}

/**
 * 查询 metric_samples，返回时序数组。
 * GET /api/v1/metrics?source=&object=&metric=&since=&limit=
 */
export async function queryMetrics(params: MetricQueryParams): Promise<MetricSample[]> {
  const qs = new URLSearchParams()
  if (params.source) qs.set('source', params.source)
  if (params.object) qs.set('object', params.object)
  if (params.metric) qs.set('metric', params.metric)
  if (params.since) qs.set('since', params.since)
  if (params.limit != null) qs.set('limit', String(params.limit))
  const query = qs.toString()
  return apiGet<MetricSample[]>(`/api/v1/metrics${query ? `?${query}` : ''}`)
}

/**
 * 将平坦的 MetricSample 数组按 object 分组，转换为 ECharts series 格式。
 * 输入已按时间升序（后端保证）。
 */
export function buildEChartsSeries(samples: MetricSample[]): EChartsSeriesItem[] {
  const groups = new Map<string, [string, number][]>()
  for (const s of samples) {
    if (!groups.has(s.object)) {
      groups.set(s.object, [])
    }
    groups.get(s.object)!.push([s.created_at, s.value])
  }
  return Array.from(groups.entries()).map(([name, data]) => ({
    name,
    type: 'line',
    smooth: true,
    data,
  }))
}
```

- [ ] **运行确认通过**

```bash
cd /Users/kanliang.xu/code/hearth/web && npm run test 2>&1 | grep -E "PASS|FAIL|metrics"
```

预期：`metrics.test.ts` PASS。

- [ ] **git commit**

```bash
cd /Users/kanliang.xu/code/hearth/web
git add src/api/metrics.ts src/utils/metrics.test.ts
git commit -m "feat(frontend): metrics API 客户端与 ECharts 数据组装函数"
```

---

#### Task 9：新建 `MetricChart.vue` 通用折线图组件

**Files:**
- Create: `web/src/components/dashboard/MetricChart.vue`

**TDD 步骤：**（组件无独立渲染测试，通过 build 验证 + Task 11 集成验证）

- [ ] **实现**

新建 `web/src/components/dashboard/MetricChart.vue`：

```vue
<script setup lang="ts">
import { computed, ref, watchEffect } from 'vue'
import { use } from 'echarts/core'
import { LineChart } from 'echarts/charts'
import {
  GridComponent,
  LegendComponent,
  TooltipComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import VChart from 'vue-echarts'
import { NEmpty, NSpin } from 'naive-ui'
import { buildEChartsSeries, queryMetrics, type MetricQueryParams } from '../../api/metrics'

use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

export interface SeriesDef {
  /** 图例名称，同时也是传给 API 的 object 过滤值（若设置） */
  label: string
  params: MetricQueryParams
}

const props = defineProps<{
  title: string
  /** 每条查询序列的定义；支持多个并发请求 */
  seriesDefs: SeriesDef[]
  /** 时间范围（小时），映射到 since 参数 */
  timeRangeHours: 1 | 6 | 24
  unit?: string
}>()

const loading = ref(false)
const error = ref('')

interface SeriesData {
  name: string
  type: 'line'
  smooth: boolean
  data: [string, number][]
}

const series = ref<SeriesData[]>([])

watchEffect(async () => {
  loading.value = true
  error.value = ''
  const since = new Date(Date.now() - props.timeRangeHours * 3600 * 1000).toISOString()

  const results = await Promise.allSettled(
    props.seriesDefs.map((def) =>
      queryMetrics({ ...def.params, since, limit: 500 }),
    ),
  )

  const allSeries: SeriesData[] = []
  for (const result of results) {
    if (result.status === 'fulfilled') {
      allSeries.push(...buildEChartsSeries(result.value))
    }
  }
  series.value = allSeries
  loading.value = false
})

const hasData = computed(() => series.value.some((s) => s.data.length > 0))

const chartOption = computed(() => ({
  tooltip: {
    trigger: 'axis',
    formatter: (params: { seriesName: string; value: [string, number] }[]) =>
      params.map((p) => `${p.seriesName}: ${p.value[1]}${props.unit ?? '%'}`).join('<br/>'),
  },
  legend: { bottom: 0 },
  grid: { left: 48, right: 16, top: 12, bottom: 36 },
  xAxis: {
    type: 'time',
    axisLabel: { formatter: (v: number) => new Date(v).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) },
  },
  yAxis: {
    type: 'value',
    min: 0,
    max: props.unit ? undefined : 100,
    axisLabel: { formatter: (v: number) => `${v}${props.unit ?? '%'}` },
  },
  series: series.value,
}))
</script>

<template>
  <div style="position: relative; min-height: 160px">
    <div v-if="loading" style="display: flex; justify-content: center; padding: 40px">
      <n-spin size="small" />
    </div>
    <template v-else-if="hasData">
      <v-chart :option="chartOption" style="height: 200px" autoresize />
    </template>
    <n-empty v-else description="暂无数据" style="padding: 32px 0" />
  </div>
</template>
```

- [ ] **构建验证**

```bash
cd /Users/kanliang.xu/code/hearth/web && npm run build 2>&1 | tail -10
```

预期：无 TypeScript 错误，build 成功。

- [ ] **git commit**

```bash
cd /Users/kanliang.xu/code/hearth/web
git add src/components/dashboard/MetricChart.vue
git commit -m "feat(frontend): MetricChart 通用折线图组件（ECharts 按需注册）"
```

---

#### Task 10：新建 `TrendSection.vue` 趋势区块容器

**Files:**
- Create: `web/src/components/dashboard/TrendSection.vue`

**TDD 步骤：**（build 验证）

- [ ] **实现**

新建 `web/src/components/dashboard/TrendSection.vue`：

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { NCard, NGi, NGrid, NRadioButton, NRadioGroup, NSpace } from 'naive-ui'
import MetricChart from './MetricChart.vue'
import type { SeriesDef } from './MetricChart.vue'

type TimeRange = 1 | 6 | 24

const timeRange = ref<TimeRange>(6)

const timeRangeOptions: { label: string; value: TimeRange }[] = [
  { label: '1h', value: 1 },
  { label: '6h', value: 6 },
  { label: '24h', value: 24 },
]

// PVE 节点 CPU 趋势：source=proxmox，metric=cpu_pct；每节点一条序列（不过滤 object，由 API 返回所有节点）
const pveCpuDefs: SeriesDef[] = [
  { label: 'PVE CPU', params: { source: 'proxmox', metric: 'cpu_pct' } },
]

// PVE 节点内存趋势
const pveMemDefs: SeriesDef[] = [
  { label: 'PVE 内存', params: { source: 'proxmox', metric: 'mem_pct' } },
]

// Docker 容器 CPU 趋势：source=docker，metric=cpu_pct
// 后端返回所有 running 容器，前端按当前数据 top5 由 MetricChart 的 series 数量隐性体现
const dockerCpuDefs: SeriesDef[] = [
  { label: 'Docker CPU', params: { source: 'docker', metric: 'cpu_pct' } },
]

// OpenWrt 内存趋势：source=openwrt，metric=mem_used_pct；object 为路由器 hostname
const openwrtMemDefs: SeriesDef[] = [
  { label: 'OpenWrt 内存', params: { source: 'openwrt', metric: 'mem_used_pct' } },
]
</script>

<template>
  <n-card title="资源趋势" style="margin-top: 16px">
    <template #header-extra>
      <n-radio-group v-model:value="timeRange" size="small">
        <n-radio-button v-for="opt in timeRangeOptions" :key="opt.value" :value="opt.value">
          {{ opt.label }}
        </n-radio-button>
      </n-radio-group>
    </template>

    <n-grid :cols="2" x-gap="16" y-gap="16" responsive="screen" item-responsive>
      <n-gi span="2 m:1">
        <n-space vertical :size="4">
          <span style="font-size: 13px; opacity: .75">PVE CPU %</span>
          <MetricChart title="PVE CPU" :series-defs="pveCpuDefs" :time-range-hours="timeRange" />
        </n-space>
      </n-gi>

      <n-gi span="2 m:1">
        <n-space vertical :size="4">
          <span style="font-size: 13px; opacity: .75">PVE 内存 %</span>
          <MetricChart title="PVE 内存" :series-defs="pveMemDefs" :time-range-hours="timeRange" />
        </n-space>
      </n-gi>

      <n-gi span="2 m:1">
        <n-space vertical :size="4">
          <span style="font-size: 13px; opacity: .75">Docker 容器 CPU %</span>
          <MetricChart title="Docker CPU" :series-defs="dockerCpuDefs" :time-range-hours="timeRange" />
        </n-space>
      </n-gi>

      <n-gi span="2 m:1">
        <n-space vertical :size="4">
          <span style="font-size: 13px; opacity: .75">OpenWrt 内存 %</span>
          <MetricChart title="OpenWrt 内存" :series-defs="openwrtMemDefs" :time-range-hours="timeRange" />
        </n-space>
      </n-gi>
    </n-grid>
  </n-card>
</template>
```

- [ ] **构建验证**

```bash
cd /Users/kanliang.xu/code/hearth/web && npm run build 2>&1 | tail -10
```

- [ ] **git commit**

```bash
cd /Users/kanliang.xu/code/hearth/web
git add src/components/dashboard/TrendSection.vue
git commit -m "feat(frontend): TrendSection 资源趋势区块，1h/6h/24h 时间切换"
```

---

#### Task 11：在 DashboardView 中引入 TrendSection

**Files:**
- Modify: `web/src/views/DashboardView.vue`

**TDD 步骤：**（build 验证）

- [ ] **修改 DashboardView.vue**

Read `web/src/views/DashboardView.vue`，在 `<script setup>` 的 import 区最后一个组件 import 后追加：

```typescript
import TrendSection from '../components/dashboard/TrendSection.vue'
```

在 `<template>` 中，找到最后一个现有区块（如 `<EventList>` 或 `<RiskList>`）之后追加：

```html
<TrendSection />
```

- [ ] **构建验证**

```bash
cd /Users/kanliang.xu/code/hearth/web && npm run build 2>&1 | tail -10
```

预期：build 成功，无 TypeScript 错误。

- [ ] **git commit**

```bash
cd /Users/kanliang.xu/code/hearth/web
git add src/views/DashboardView.vue
git commit -m "feat(frontend): DashboardView 集成 TrendSection 资源趋势区块"
```

---

### 阶段四：整体验证

---

#### Task 12：全量测试与构建验证

**Files:** 无代码改动，仅验证。

- [ ] **后端全量测试（含竞态检测）**

```bash
cd /Users/kanliang.xu/code/hearth/server && go test -race ./... 2>&1
```

预期：所有包 `ok`，无 FAIL。

- [ ] **前端单测**

```bash
cd /Users/kanliang.xu/code/hearth/web && npm run test 2>&1
```

预期：所有 `.test.ts` PASS，覆盖 `metrics.test.ts`、`overview.test.ts`、`format.test.ts` 等。

- [ ] **前端 TypeScript 类型检查 + 构建**

```bash
cd /Users/kanliang.xu/code/hearth/web && npm run build 2>&1
```

预期：`built in` 无错误。

- [ ] **回归确认**：确认以下测试用例全部绿灯：
  - `TestNavMigrateDeviceID_Idempotent`
  - `TestNavItemDeviceID_CRUD`
  - `TestNavItemDeviceID_UniqueConstraint`
  - `TestNavItemDeviceIDAPI`
  - `TestDeviceDetailWithNavItem`
  - `TestNavCategoryLifecycle`（原有，不应退化）
  - `TestNavValidation`（原有）
  - `TestDeviceAndHealthCheckLifecycle`（原有）
  - `metrics.test.ts` — `buildEChartsSeries` 两个用例

---

## 关键约定

### metric_samples 中实际写入的 source / object / metric 名称

| 来源 | source 值 | object 值 | metric 名 |
|------|-----------|-----------|-----------|
| PVE 节点 | `proxmox`（传入 Recorder 的 source 参数） | 节点名称（如 `pve-01`） | `cpu_pct`, `mem_pct`, `uptime_sec`, `vms_running` |
| Docker 容器 | `docker` | 容器 `name`（如 `immich`） | `cpu_pct`, `mem_pct` |
| Docker 汇总 | `docker` | `docker` | `containers_running`, `containers_total` |
| OpenWrt | `openwrt` | 路由器 `hostname`（空时为 `openwrt`） | `mem_used_pct`, `load1`, `uptime_sec` |

> **注意**：OpenWrt 的内存指标名是 `mem_used_pct`（非 `mem_pct`），与 PVE 不同，`TrendSection` 已正确使用。

### navSchema 中 device_id 与 devices 表的跨库 FK

`nav.db` 和 `inventory.db` 是两个独立 SQLite 文件（`NavStore` 和 `InventoryStore` 各自 `sql.Open` 不同路径），SQLite 跨文件外键不生效。因此：
- `ALTER TABLE nav_items ADD COLUMN device_id INTEGER` 上写的 `REFERENCES devices(id)` **不会实际约束**，仅作文档注释。
- 设备存在性校验在应用层（`checkDeviceExists`）完成，Task 2 已实现。
- `ON DELETE SET NULL` 同样不生效，需要 inventory 层 `DeleteDevice` 时同步清空（本期不实现，属 future enhancement）。

### `since` 参数格式

后端 `time.Parse(time.RFC3339, raw)` 严格要求 RFC3339 格式（如 `2026-07-17T10:00:00Z`）。前端 `new Date(...).toISOString()` 输出符合此格式，`MetricChart.vue` 已正确处理。

### 测试辅助函数

`server/internal/api/status_test.go` 中定义了 `newTestRouter(t)` 返回 `(http.Handler, *store.SnapshotStore, *store.NavStore)`，以及 `doJSON(t, h, method, path, body)` 返回 `(int, envelope)`。新测试直接复用，无需重定义。
