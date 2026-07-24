package health

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/m1iktea/hearth/server/internal/store"
)

// fakeStore 实现 checkStore，捕获迁移调用。目标为空时 probe 立即返回 offline，无需网络。
type fakeStore struct {
	mu          sync.Mutex
	checks      []store.CheckWithDevice
	transitions int
	lastStatus  string
	lastReason  string
}

func (f *fakeStore) ListEnabledChecks() ([]store.CheckWithDevice, error) { return f.checks, nil }

func (f *fakeStore) RecordProbe(store.CheckWithDevice, string, string, int64, time.Time) error {
	return nil
}

func (f *fakeStore) RecordHealthTransition(c store.CheckWithDevice, status, reason string, latency int64, at time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.transitions++
	f.lastStatus = status
	f.lastReason = reason
	return true, nil
}

func TestRunOnceRecordsTransition(t *testing.T) {
	fs := &fakeStore{checks: []store.CheckWithDevice{
		{HealthCheck: store.HealthCheck{ID: 1, Name: "x", Type: "ping", Target: ""}},
	}}
	r := NewRunner(fs, time.Minute, slog.Default())
	r.RunOnce(context.Background())

	if fs.transitions != 1 {
		t.Fatalf("RunOnce 应调用一次 RecordHealthTransition，实际 %d", fs.transitions)
	}
	if fs.lastStatus != "offline" {
		t.Fatalf("状态应为 offline，实际 %q", fs.lastStatus)
	}
	if fs.lastReason == "" {
		t.Fatal("offline 应携带失败原因")
	}
}
