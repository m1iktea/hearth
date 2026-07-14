package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// InventoryStore 保存局域网资产、巡检配置和站内事件。它和导航共用同一 SQLite 文件，
// 但使用独立连接，避免把两类业务耦合在同一个 store 中。
type InventoryStore struct{ db *sql.DB }

type Device struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	Hostname   string    `json:"hostname"`
	IPAddress  string    `json:"ip_address"`
	MACAddress string    `json:"mac_address"`
	Location   string    `json:"location"`
	Notes      string    `json:"notes"`
	URL        string    `json:"url"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type HealthCheck struct {
	ID             int64      `json:"id"`
	DeviceID       int64      `json:"device_id"`
	Name           string     `json:"name"`
	Type           string     `json:"type"` // ping | tcp | http
	Target         string     `json:"target"`
	Port           int        `json:"port"`
	ExpectedStatus int        `json:"expected_status"`
	Enabled        bool       `json:"enabled"`
	LastStatus     string     `json:"last_status"`
	LastError      string     `json:"last_error"`
	LatencyMS      int64      `json:"latency_ms"`
	CheckedAt      *time.Time `json:"checked_at,omitempty"`
}

type DeviceDetail struct {
	Device Device        `json:"device"`
	Checks []HealthCheck `json:"checks"`
}

type Event struct {
	ID         int64      `json:"id"`
	DeviceID   int64      `json:"device_id"`
	DeviceName string     `json:"device_name"`
	CheckID    int64      `json:"check_id"`
	Type       string     `json:"type"`
	Severity   string     `json:"severity"`
	Title      string     `json:"title"`
	Message    string     `json:"message"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// DiscoveredDevice is an ARP observation. MAC address is the primary identity;
// IP is only a fallback because DHCP addresses can change.
type DiscoveredDevice struct {
	IPAddress  string
	MACAddress string
	Vendor     string
}

const inventorySchema = `
CREATE TABLE IF NOT EXISTS devices (
 id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, kind TEXT NOT NULL DEFAULT 'other',
 hostname TEXT NOT NULL DEFAULT '', ip_address TEXT NOT NULL DEFAULT '', mac_address TEXT NOT NULL DEFAULT '',
 location TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT '', url TEXT NOT NULL DEFAULT '',
 enabled INTEGER NOT NULL DEFAULT 1, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS health_checks (
 id INTEGER PRIMARY KEY AUTOINCREMENT, device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
 name TEXT NOT NULL, type TEXT NOT NULL, target TEXT NOT NULL DEFAULT '', port INTEGER NOT NULL DEFAULT 0,
 expected_status INTEGER NOT NULL DEFAULT 0, enabled INTEGER NOT NULL DEFAULT 1,
 last_status TEXT NOT NULL DEFAULT 'unknown', last_error TEXT NOT NULL DEFAULT '', latency_ms INTEGER NOT NULL DEFAULT 0,
 checked_at DATETIME
);
CREATE TABLE IF NOT EXISTS events (
 id INTEGER PRIMARY KEY AUTOINCREMENT, device_id INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
 check_id INTEGER NOT NULL REFERENCES health_checks(id) ON DELETE CASCADE, type TEXT NOT NULL, severity TEXT NOT NULL,
 title TEXT NOT NULL, message TEXT NOT NULL, created_at DATETIME NOT NULL, resolved_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_health_checks_device ON health_checks(device_id);
CREATE INDEX IF NOT EXISTS idx_events_created ON events(created_at DESC);
CREATE TABLE IF NOT EXISTS metric_samples (
 id INTEGER PRIMARY KEY AUTOINCREMENT, source TEXT NOT NULL, object TEXT NOT NULL,
 metric TEXT NOT NULL, value REAL NOT NULL, created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_metric_samples_lookup ON metric_samples(source, object, metric, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_metric_samples_created ON metric_samples(created_at);
CREATE TABLE IF NOT EXISTS snapshots (
 source TEXT PRIMARY KEY, status TEXT NOT NULL, collected_at DATETIME NOT NULL,
 last_error TEXT NOT NULL DEFAULT '', data TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS system_events (
 id INTEGER PRIMARY KEY AUTOINCREMENT, source TEXT NOT NULL, object TEXT NOT NULL,
 type TEXT NOT NULL, severity TEXT NOT NULL, title TEXT NOT NULL, message TEXT NOT NULL,
 created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_system_events_created ON system_events(created_at DESC);`

// sqliteDSN 统一两个 store 的连接参数：WAL 显著降低高频小事务的 fsync 开销，
// 也缓解 nav/inventory 两个连接池对同一文件的写锁竞争。
func sqliteDSN(path string) string {
	return path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
}

func OpenInventory(path string) (*InventoryStore, error) {
	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open inventory sqlite: %w", err)
	}
	if _, err := db.Exec(inventorySchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init inventory schema: %w", err)
	}
	return &InventoryStore{db: db}, nil
}
func (s *InventoryStore) Close() error { return s.db.Close() }

func scanDevice(row interface{ Scan(...any) error }) (Device, error) {
	var d Device
	var enabled int
	err := row.Scan(&d.ID, &d.Name, &d.Kind, &d.Hostname, &d.IPAddress, &d.MACAddress, &d.Location, &d.Notes, &d.URL, &enabled, &d.CreatedAt, &d.UpdatedAt)
	d.Enabled = enabled != 0
	return d, err
}

const deviceColumns = `id, name, kind, hostname, ip_address, mac_address, location, notes, url, enabled, created_at, updated_at`

func (s *InventoryStore) ListDevices() ([]Device, error) {
	rows, err := s.db.Query(`SELECT ` + deviceColumns + ` FROM devices ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Device{}
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}
func (s *InventoryStore) GetDevice(id int64) (Device, error) {
	return scanDevice(s.db.QueryRow(`SELECT `+deviceColumns+` FROM devices WHERE id = ?`, id))
}
func (s *InventoryStore) CreateDevice(d Device) (Device, error) {
	now := time.Now().UTC()
	r, err := s.db.Exec(`INSERT INTO devices (name,kind,hostname,ip_address,mac_address,location,notes,url,enabled,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, d.Name, d.Kind, d.Hostname, d.IPAddress, d.MACAddress, d.Location, d.Notes, d.URL, boolInt(d.Enabled), now, now)
	if err != nil {
		return Device{}, err
	}
	d.ID, _ = r.LastInsertId()
	d.CreatedAt = now
	d.UpdatedAt = now
	return d, nil
}
func (s *InventoryStore) UpdateDevice(d Device) (Device, error) {
	now := time.Now().UTC()
	r, err := s.db.Exec(`UPDATE devices SET name=?,kind=?,hostname=?,ip_address=?,mac_address=?,location=?,notes=?,url=?,enabled=?,updated_at=? WHERE id=?`, d.Name, d.Kind, d.Hostname, d.IPAddress, d.MACAddress, d.Location, d.Notes, d.URL, boolInt(d.Enabled), now, d.ID)
	if err != nil {
		return Device{}, err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return Device{}, sql.ErrNoRows
	}
	d.UpdatedAt = now
	return d, nil
}
func (s *InventoryStore) DeleteDevice(id int64) error {
	_, err := s.db.Exec(`DELETE FROM devices WHERE id=?`, id)
	return err
}

// UpsertDiscoveredDevice automatically brings an ARP result into the device
// center without overwriting a user-maintained name, location or notes.
func (s *InventoryStore) UpsertDiscoveredDevice(found DiscoveredDevice) (Device, bool, error) {
	var existing Device
	var err error
	if found.MACAddress != "" {
		existing, err = scanDevice(s.db.QueryRow(`SELECT `+deviceColumns+` FROM devices WHERE lower(mac_address)=lower(?)`, found.MACAddress))
	}
	if err == sql.ErrNoRows || (err == nil && found.MACAddress == "") {
		existing, err = scanDevice(s.db.QueryRow(`SELECT `+deviceColumns+` FROM devices WHERE ip_address=?`, found.IPAddress))
	}
	if err == nil {
		now := time.Now().UTC()
		_, err = s.db.Exec(`UPDATE devices SET ip_address=?, mac_address=CASE WHEN ? <> '' THEN ? ELSE mac_address END, updated_at=? WHERE id=?`, found.IPAddress, found.MACAddress, found.MACAddress, now, existing.ID)
		if err != nil {
			return Device{}, false, err
		}
		existing.IPAddress, existing.UpdatedAt = found.IPAddress, now
		if found.MACAddress != "" {
			existing.MACAddress = found.MACAddress
		}
		return existing, false, nil
	}
	if err != sql.ErrNoRows {
		return Device{}, false, err
	}
	name := "已发现设备 · " + found.IPAddress
	vendor := strings.ToLower(strings.TrimSpace(found.Vendor))
	if vendor != "" && !strings.Contains(vendor, "unknown") && !strings.Contains(vendor, "locally administered") {
		name = found.Vendor + " · " + found.IPAddress
	}
	d, err := s.CreateDevice(Device{Name: name, Kind: "discovered", IPAddress: found.IPAddress, MACAddress: found.MACAddress, Enabled: true})
	return d, true, err
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func scanCheck(row interface{ Scan(...any) error }) (HealthCheck, error) {
	var c HealthCheck
	var enabled int
	err := row.Scan(&c.ID, &c.DeviceID, &c.Name, &c.Type, &c.Target, &c.Port, &c.ExpectedStatus, &enabled, &c.LastStatus, &c.LastError, &c.LatencyMS, &c.CheckedAt)
	c.Enabled = enabled != 0
	return c, err
}

const checkColumns = `id, device_id, name, type, target, port, expected_status, enabled, last_status, last_error, latency_ms, checked_at`

func (s *InventoryStore) ListChecks(deviceID int64) ([]HealthCheck, error) {
	rows, err := s.db.Query(`SELECT `+checkColumns+` FROM health_checks WHERE device_id=? ORDER BY id`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HealthCheck{}
	for rows.Next() {
		c, e := scanCheck(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *InventoryStore) GetDeviceDetail(id int64) (DeviceDetail, error) {
	d, e := s.GetDevice(id)
	if e != nil {
		return DeviceDetail{}, e
	}
	c, e := s.ListChecks(id)
	return DeviceDetail{Device: d, Checks: c}, e
}
func (s *InventoryStore) CreateCheck(c HealthCheck) (HealthCheck, error) {
	r, e := s.db.Exec(`INSERT INTO health_checks (device_id,name,type,target,port,expected_status,enabled) VALUES (?,?,?,?,?,?,?)`, c.DeviceID, c.Name, c.Type, c.Target, c.Port, c.ExpectedStatus, boolInt(c.Enabled))
	if e != nil {
		return HealthCheck{}, e
	}
	c.ID, _ = r.LastInsertId()
	c.LastStatus = "unknown"
	return c, nil
}
func (s *InventoryStore) UpdateCheck(c HealthCheck) (HealthCheck, error) {
	r, e := s.db.Exec(`UPDATE health_checks SET name=?,type=?,target=?,port=?,expected_status=?,enabled=? WHERE id=? AND device_id=?`, c.Name, c.Type, c.Target, c.Port, c.ExpectedStatus, boolInt(c.Enabled), c.ID, c.DeviceID)
	if e != nil {
		return HealthCheck{}, e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return HealthCheck{}, sql.ErrNoRows
	}
	old, e := s.getCheck(c.ID)
	if e != nil {
		return HealthCheck{}, e
	}
	return old, nil
}
func (s *InventoryStore) getCheck(id int64) (HealthCheck, error) {
	return scanCheck(s.db.QueryRow(`SELECT `+checkColumns+` FROM health_checks WHERE id=?`, id))
}
func (s *InventoryStore) DeleteCheck(id int64) error {
	_, e := s.db.Exec(`DELETE FROM health_checks WHERE id=?`, id)
	return e
}

type CheckWithDevice struct {
	HealthCheck
	DeviceName string `json:"device_name"`
	DeviceIP   string `json:"device_ip"`
}

func (s *InventoryStore) ListEnabledChecks() ([]CheckWithDevice, error) {
	rows, e := s.db.Query(`SELECT h.id,h.device_id,h.name,h.type,h.target,h.port,h.expected_status,h.enabled,h.last_status,h.last_error,h.latency_ms,h.checked_at,d.name,d.ip_address FROM health_checks h JOIN devices d ON d.id=h.device_id WHERE h.enabled=1 AND d.enabled=1 ORDER BY h.id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []CheckWithDevice{}
	for rows.Next() {
		var c HealthCheck
		var en int
		var deviceName, deviceIP string
		e = rows.Scan(&c.ID, &c.DeviceID, &c.Name, &c.Type, &c.Target, &c.Port, &c.ExpectedStatus, &en, &c.LastStatus, &c.LastError, &c.LatencyMS, &c.CheckedAt, &deviceName, &deviceIP)
		if e != nil {
			return nil, e
		}
		c.Enabled = en != 0
		out = append(out, CheckWithDevice{HealthCheck: c, DeviceName: deviceName, DeviceIP: deviceIP})
	}
	return out, rows.Err()
}

// RecordProbe 写入最新状态；状态切换时才创建事件，避免重复刷屏。
func (s *InventoryStore) RecordProbe(c CheckWithDevice, status, message string, latency int64, at time.Time) error {
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var previous string
	if e = tx.QueryRow(`SELECT last_status FROM health_checks WHERE id=?`, c.ID).Scan(&previous); e != nil {
		return e
	}
	if _, e = tx.Exec(`UPDATE health_checks SET last_status=?,last_error=?,latency_ms=?,checked_at=? WHERE id=?`, status, message, latency, at.UTC(), c.ID); e != nil {
		return e
	}
	// 首次失败也值得记录；首次成功只是建立基线，不额外制造事件。
	if previous != status && (previous == "online" || previous == "offline" || (previous == "unknown" && status == "offline")) {
		title := "设备恢复"
		severity := "info"
		if status == "offline" {
			title = "设备离线"
			severity = "warning"
		} else {
			// 用恢复时间关闭该检查上一次未关闭的离线事件，保留恢复事件供审计。
			if _, e = tx.Exec(`UPDATE events SET resolved_at=? WHERE check_id=? AND type='offline' AND resolved_at IS NULL`, at.UTC(), c.ID); e != nil {
				return e
			}
		}
		_, e = tx.Exec(`INSERT INTO events (device_id,check_id,type,severity,title,message,created_at,resolved_at) VALUES (?,?,?,?,?,?,?,NULL)`, c.DeviceID, c.ID, status, severity, title, c.DeviceName+" · "+c.Name+": "+message, at.UTC())
		if e != nil {
			return e
		}
	}
	return tx.Commit()
}

// ListEvents 合并设备事件与系统事件（节点重启等）。系统事件 id 取负数，
// 避免与设备事件的自增 id 冲突（前端以 id 作为列表 key）。
func (s *InventoryStore) ListEvents(limit int) ([]Event, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, e := s.db.Query(`SELECT e.id,e.device_id,d.name,e.check_id,e.type,e.severity,e.title,e.message,e.created_at,e.resolved_at FROM events e JOIN devices d ON d.id=e.device_id
UNION ALL
SELECT -se.id,0,se.object,0,se.type,se.severity,se.title,se.message,se.created_at,NULL FROM system_events se
ORDER BY created_at DESC LIMIT ?`, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var v Event
		if e = rows.Scan(&v.ID, &v.DeviceID, &v.DeviceName, &v.CheckID, &v.Type, &v.Severity, &v.Title, &v.Message, &v.CreatedAt, &v.ResolvedAt); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
