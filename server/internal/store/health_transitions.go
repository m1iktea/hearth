package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// HealthTransition 是一条健康检查状态迁移事件（黑匣子）。只在状态变化时写入，
// 用于事后还原目标的绿红可用率时间线，而非每次探测都记流水。
type HealthTransition struct {
	ID        int64     `json:"id"`
	CheckID   int64     `json:"check_id"`
	DeviceID  int64     `json:"device_id"`
	Target    string    `json:"target"`
	CheckType string    `json:"check_type"`
	Status    string    `json:"status"` // online | offline
	LatencyMS int64     `json:"latency_ms"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// RecordHealthTransition 仅在状态与该检查最近一次记录不同（或首次确认状态）时写入一条迁移事件，
// 返回是否实际写入。连续同状态直接跳过，避免撑爆存储。
func (s *InventoryStore) RecordHealthTransition(c CheckWithDevice, status, reason string, latency int64, at time.Time) (bool, error) {
	var prev string
	err := s.db.QueryRow(
		`SELECT status FROM health_transitions WHERE check_id=? ORDER BY created_at DESC, id DESC LIMIT 1`,
		c.ID,
	).Scan(&prev)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("query last health transition: %w", err)
	}
	if err == nil && prev == status {
		return false, nil // 连续同状态，不重复写
	}

	tgt := strings.TrimSpace(c.Target)
	if tgt == "" {
		tgt = strings.TrimSpace(c.DeviceIP)
	}
	if _, err := s.db.Exec(
		`INSERT INTO health_transitions (check_id,device_id,target,check_type,status,latency_ms,reason,created_at)
		 VALUES (?,?,?,?,?,?,?,?)`,
		c.ID, c.DeviceID, tgt, c.Type, status, latency, reason, at.UTC(),
	); err != nil {
		return false, fmt.Errorf("insert health transition: %w", err)
	}
	return true, nil
}

// QueryHealthTransitions 返回某检查用于还原时间线的迁移事件，时间升序。
// 除窗口内 [since, now] 的迁移外，还会附带窗口前最近一条迁移作为“锚点”——
// 它决定了窗口起点的初始状态，否则长期稳定的目标在窗口内无迁移时会整段显示为无数据。
func (s *InventoryStore) QueryHealthTransitions(checkID int64, since time.Time, limit int) ([]HealthTransition, error) {
	if limit <= 0 || limit > 10000 {
		limit = 1000
	}

	out := []HealthTransition{}

	// 锚点：窗口开始前最近的一条迁移（若有）。
	var anchor HealthTransition
	err := s.db.QueryRow(
		`SELECT id,check_id,device_id,target,check_type,status,latency_ms,reason,created_at
		 FROM health_transitions WHERE check_id=? AND created_at < ?
		 ORDER BY created_at DESC, id DESC LIMIT 1`,
		checkID, since.UTC(),
	).Scan(&anchor.ID, &anchor.CheckID, &anchor.DeviceID, &anchor.Target, &anchor.CheckType, &anchor.Status, &anchor.LatencyMS, &anchor.Reason, &anchor.CreatedAt)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("query health transition anchor: %w", err)
	}
	if err == nil {
		out = append(out, anchor)
	}

	rows, err := s.db.Query(
		`SELECT id,check_id,device_id,target,check_type,status,latency_ms,reason,created_at
		 FROM health_transitions WHERE check_id=? AND created_at >= ?
		 ORDER BY created_at ASC, id ASC LIMIT ?`,
		checkID, since.UTC(), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var h HealthTransition
		if err := rows.Scan(&h.ID, &h.CheckID, &h.DeviceID, &h.Target, &h.CheckType, &h.Status, &h.LatencyMS, &h.Reason, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
