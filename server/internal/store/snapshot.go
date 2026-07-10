package store

import (
	"sort"
	"sync"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
)

// SnapshotStore 保存各数据源最新快照，并发安全。
type SnapshotStore struct {
	mu    sync.RWMutex
	snaps map[string]collector.Snapshot
}

func NewSnapshotStore() *SnapshotStore {
	return &SnapshotStore{snaps: make(map[string]collector.Snapshot)}
}

func (s *SnapshotStore) SetOK(source string, data any, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snaps[source] = collector.Snapshot{
		Source: source, Status: collector.StatusOnline, CollectedAt: at, Data: data,
	}
}

// SetError 标记离线但保留上次成功的 Data，便于页面展示最后已知状态。
func (s *SnapshotStore) SetError(source string, err error, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.snaps[source]
	s.snaps[source] = collector.Snapshot{
		Source: source, Status: collector.StatusOffline, CollectedAt: at,
		LastError: err.Error(), Data: prev.Data,
	}
}

func (s *SnapshotStore) Get(source string) (collector.Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.snaps[source]
	return snap, ok
}

func (s *SnapshotStore) All() []collector.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]collector.Snapshot, 0, len(s.snaps))
	for _, v := range s.snaps {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}
