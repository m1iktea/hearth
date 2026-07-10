package collector

import (
	"context"
	"time"
)

const (
	StatusOnline  = "online"
	StatusOffline = "offline"
)

// Snapshot 是某个数据源某一时刻的状态快照。
type Snapshot struct {
	Source      string    `json:"source"`
	Status      string    `json:"status"` // online | offline
	CollectedAt time.Time `json:"collected_at"`
	LastError   string    `json:"last_error,omitempty"`
	Data        any       `json:"data,omitempty"`
}

// Collector 采集一个数据源。Collect 返回源特定的 Data 负载。
type Collector interface {
	Name() string
	Collect(ctx context.Context) (any, error)
}
