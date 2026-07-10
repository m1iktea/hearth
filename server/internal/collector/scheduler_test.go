package collector_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
	"github.com/m1iktea/hearth/server/internal/store"
)

type fake struct {
	name string
	data any
	err  error
}

func (f *fake) Name() string                            { return f.name }
func (f *fake) Collect(_ context.Context) (any, error) { return f.data, f.err }

func TestCollectOnceWritesSnapshots(t *testing.T) {
	s := store.NewSnapshotStore()
	sched := collector.NewScheduler(
		[]collector.Collector{
			&fake{name: "ok-source", data: "payload"},
			&fake{name: "bad-source", err: errors.New("down")},
		},
		s, time.Minute, slog.Default(),
	)

	sched.CollectOnce(context.Background())

	ok, _ := s.Get("ok-source")
	if ok.Status != collector.StatusOnline || ok.Data != "payload" {
		t.Errorf("ok-source: %+v", ok)
	}
	bad, _ := s.Get("bad-source")
	if bad.Status != collector.StatusOffline || bad.LastError != "down" {
		t.Errorf("bad-source: %+v", bad)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	s := store.NewSnapshotStore()
	sched := collector.NewScheduler(
		[]collector.Collector{&fake{name: "ok-source", data: 1}},
		s, 10*time.Millisecond, slog.Default(),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() { sched.Run(ctx); close(done) }()

	select {
	case <-done: // Run 应随 ctx 取消而返回
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
	if _, ok := s.Get("ok-source"); !ok {
		t.Error("expected at least one collection")
	}
}
