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
	if _, err := n.db.Exec(`ALTER TABLE nav_items ADD COLUMN device_id INTEGER`); err != nil {
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
