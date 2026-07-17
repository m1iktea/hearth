package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func newTestNav(t *testing.T) *NavStore {
	t.Helper()
	n, err := OpenNav(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenNav: %v", err)
	}
	t.Cleanup(func() { n.Close() })
	return n
}

func TestCategoryCRUD(t *testing.T) {
	n := newTestNav(t)

	cat, err := n.CreateCategory("基础设施", 1)
	if err != nil || cat.ID == 0 || cat.Name != "基础设施" {
		t.Fatalf("CreateCategory: %+v, %v", cat, err)
	}

	cat2, err := n.UpdateCategory(cat.ID, "服务", 2)
	if err != nil || cat2.Name != "服务" || cat2.SortOrder != 2 {
		t.Fatalf("UpdateCategory: %+v, %v", cat2, err)
	}

	if err := n.DeleteCategory(cat.ID); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}
	cats, _ := n.ListCategories()
	if len(cats) != 0 {
		t.Errorf("want empty, got %+v", cats)
	}
}

func TestItemCRUDAndListNesting(t *testing.T) {
	n := newTestNav(t)
	cat, _ := n.CreateCategory("服务", 1)

	item, err := n.CreateItem(Item{CategoryID: cat.ID, Name: "PVE", URL: "https://pve:8006", Icon: "server", SortOrder: 1})
	if err != nil || item.ID == 0 {
		t.Fatalf("CreateItem: %+v, %v", item, err)
	}

	item.Name = "Proxmox"
	updated, err := n.UpdateItem(item)
	if err != nil || updated.Name != "Proxmox" {
		t.Fatalf("UpdateItem: %+v, %v", updated, err)
	}

	cats, err := n.ListCategories()
	if err != nil || len(cats) != 1 || len(cats[0].Items) != 1 {
		t.Fatalf("ListCategories: %+v, %v", cats, err)
	}
	if cats[0].Items[0].Name != "Proxmox" {
		t.Errorf("item = %+v", cats[0].Items[0])
	}

	if err := n.DeleteItem(item.ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
}

func TestDeleteCategoryCascadesItems(t *testing.T) {
	n := newTestNav(t)
	cat, _ := n.CreateCategory("临时", 1)
	n.CreateItem(Item{CategoryID: cat.ID, Name: "x", URL: "http://x"})

	if err := n.DeleteCategory(cat.ID); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}
	cats, _ := n.ListCategories()
	if len(cats) != 0 {
		t.Errorf("want cascade delete, got %+v", cats)
	}
}

// TestNavMigrateDeviceID_ExistingColumnNoIndex 验证：历史库有 device_id 列但无唯一索引时，
// MigrateDeviceID 能补建索引（重复 device_id 插入应报错）。
func TestNavMigrateDeviceID_ExistingColumnNoIndex(t *testing.T) {
	// 直接打开 SQLite，手工建含 device_id 列但无索引的表
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// 建表（含 device_id 列，故意不建索引，模拟历史库）
	_, err = db.Exec(`
		CREATE TABLE nav_categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			sort_order INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE nav_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category_id INTEGER NOT NULL REFERENCES nav_categories(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			icon TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0,
			device_id INTEGER
		);
	`)
	if err != nil {
		db.Close()
		t.Fatalf("create legacy schema: %v", err)
	}
	db.Close()

	// 通过 NavStore 打开同一个库，触发 MigrateDeviceID
	n, err := OpenNav(dbPath)
	if err != nil {
		t.Fatalf("OpenNav on legacy db: %v", err)
	}
	t.Cleanup(func() { n.Close() })

	// 验证唯一索引已补建：插入同一 device_id 的两条记录应触发约束错误
	cat, err := n.CreateCategory("测试", 0)
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	deviceID := int64(99)
	if _, err := n.CreateItem(Item{CategoryID: cat.ID, Name: "A", URL: "http://a", DeviceID: &deviceID}); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	_, err = n.CreateItem(Item{CategoryID: cat.ID, Name: "B", URL: "http://b", DeviceID: &deviceID})
	if err == nil {
		t.Fatal("expected unique constraint error on duplicate device_id, got nil")
	}
}

func TestNavMigrateDeviceID_Idempotent(t *testing.T) {
	n := newTestNav(t)
	// 第一次迁移
	if err := n.MigrateDeviceID(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	// 幂等：第二次不报错
	if err := n.MigrateDeviceID(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestNavItemDeviceID_CRUD(t *testing.T) {
	n := newTestNav(t)
	if err := n.MigrateDeviceID(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cat, err := n.CreateCategory("服务", 1)
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	deviceID := int64(42)

	// 创建带 device_id 的条目
	item, err := n.CreateItem(Item{
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
	cats, err := n.ListCategories()
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(cats[0].Items) != 1 || cats[0].Items[0].DeviceID == nil || *cats[0].Items[0].DeviceID != 42 {
		t.Fatalf("ListCategories item DeviceID: %+v", cats[0].Items)
	}

	// GetItemByDeviceID
	found, err := n.GetItemByDeviceID(42)
	if err != nil {
		t.Fatalf("GetItemByDeviceID: %v", err)
	}
	if found.ID != item.ID {
		t.Fatalf("GetItemByDeviceID returned wrong item: %+v", found)
	}

	// 更新清除 device_id
	item.DeviceID = nil
	updated, err := n.UpdateItem(item)
	if err != nil {
		t.Fatalf("UpdateItem clear device_id: %v", err)
	}
	if updated.DeviceID != nil {
		t.Fatalf("DeviceID should be nil after clear, got %v", updated.DeviceID)
	}
}

func TestNavItemDeviceID_UniqueConstraint(t *testing.T) {
	n := newTestNav(t)
	if err := n.MigrateDeviceID(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cat, _ := n.CreateCategory("cat", 0)
	deviceID := int64(7)
	_, err := n.CreateItem(Item{CategoryID: cat.ID, Name: "A", URL: "http://a", DeviceID: &deviceID})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	// 同一 device_id 再创建应报错
	_, err = n.CreateItem(Item{CategoryID: cat.ID, Name: "B", URL: "http://b", DeviceID: &deviceID})
	if err == nil {
		t.Fatal("expected unique constraint error, got nil")
	}
}
