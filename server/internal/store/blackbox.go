package store

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
)

// MetricSample 是一条时序指标，用于故障后回看崩溃前的资源走势。
type MetricSample struct {
	Source    string    `json:"source"`
	Object    string    `json:"object"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

// InsertSamples 在单个事务中写入一批采样，减少 fsync 次数。
func (s *InventoryStore) InsertSamples(samples []MetricSample) error {
	if len(samples) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin insert samples: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO metric_samples (source,object,metric,value,created_at) VALUES (?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("prepare insert samples: %w", err)
	}
	defer stmt.Close()
	for _, m := range samples {
		if _, err := stmt.Exec(m.Source, m.Object, m.Metric, m.Value, m.CreatedAt.UTC()); err != nil {
			return fmt.Errorf("insert sample %s/%s/%s: %w", m.Source, m.Object, m.Metric, err)
		}
	}
	return tx.Commit()
}

// QuerySamples 按可选维度过滤查询，时间升序返回，便于直接画趋势。
func (s *InventoryStore) QuerySamples(source, object, metric string, since time.Time, limit int) ([]MetricSample, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}
	query := `SELECT source,object,metric,value,created_at FROM metric_samples WHERE created_at >= ?`
	args := []any{since.UTC()}
	if source != "" {
		query += ` AND source = ?`
		args = append(args, source)
	}
	if object != "" {
		query += ` AND object = ?`
		args = append(args, object)
	}
	if metric != "" {
		query += ` AND metric = ?`
		args = append(args, metric)
	}
	query += ` ORDER BY created_at ASC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MetricSample{}
	for rows.Next() {
		var m MetricSample
		if err := rows.Scan(&m.Source, &m.Object, &m.Metric, &m.Value, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// LastUptimes 返回每个 source/object 最近一次 uptime_sec 采样，
// 供进程重启后继续做“运行时长回落 = 设备重启”检测。
func (s *InventoryStore) LastUptimes() (map[string]float64, error) {
	rows, err := s.db.Query(`SELECT source, object, value, MAX(id) FROM metric_samples WHERE metric='uptime_sec' GROUP BY source, object`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var source, object string
		var value float64
		var maxID int64
		if err := rows.Scan(&source, &object, &value, &maxID); err != nil {
			return nil, err
		}
		out[source+"/"+object] = value
	}
	return out, rows.Err()
}

// UpsertSnapshot 持久化某个源的最新快照，进程重启后可恢复“最后已知状态”。
func (s *InventoryStore) UpsertSnapshot(snap collector.Snapshot) error {
	data := ""
	if snap.Data != nil {
		raw, err := json.Marshal(snap.Data)
		if err != nil {
			return fmt.Errorf("marshal snapshot %s: %w", snap.Source, err)
		}
		data = string(raw)
	}
	_, err := s.db.Exec(`INSERT INTO snapshots (source,status,collected_at,last_error,data) VALUES (?,?,?,?,?)
ON CONFLICT(source) DO UPDATE SET status=excluded.status,collected_at=excluded.collected_at,last_error=excluded.last_error,data=excluded.data`,
		snap.Source, snap.Status, snap.CollectedAt.UTC(), snap.LastError, data)
	return err
}

// LoadSnapshots 读出上次持久化的各源快照；Data 反序列化为通用结构，仅用于 API 展示。
func (s *InventoryStore) LoadSnapshots() ([]collector.Snapshot, error) {
	rows, err := s.db.Query(`SELECT source,status,collected_at,last_error,data FROM snapshots`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []collector.Snapshot{}
	for rows.Next() {
		var snap collector.Snapshot
		var data string
		if err := rows.Scan(&snap.Source, &snap.Status, &snap.CollectedAt, &snap.LastError, &data); err != nil {
			return nil, err
		}
		if data != "" {
			var v any
			if err := json.Unmarshal([]byte(data), &v); err == nil {
				snap.Data = v
			}
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// InsertSystemEvent 记录不挂靠具体设备/检查的系统级事件（如节点重启）。
func (s *InventoryStore) InsertSystemEvent(source, object, eventType, severity, title, message string, at time.Time) error {
	_, err := s.db.Exec(`INSERT INTO system_events (source,object,type,severity,title,message,created_at) VALUES (?,?,?,?,?,?,?)`,
		source, object, eventType, severity, title, message, at.UTC())
	return err
}

// PruneBefore 按保留期删除过期事件与指标，返回删除总行数。
func (s *InventoryStore) PruneBefore(eventsBefore, metricsBefore time.Time) (int64, error) {
	var total int64
	for _, q := range []struct {
		sql    string
		before time.Time
	}{
		{`DELETE FROM events WHERE created_at < ?`, eventsBefore},
		{`DELETE FROM system_events WHERE created_at < ?`, eventsBefore},
		{`DELETE FROM metric_samples WHERE created_at < ?`, metricsBefore},
	} {
		r, err := s.db.Exec(q.sql, q.before.UTC())
		if err != nil {
			return total, fmt.Errorf("prune: %w", err)
		}
		n, _ := r.RowsAffected()
		total += n
	}
	return total, nil
}
