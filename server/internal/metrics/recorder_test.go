package metrics

import (
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
	"github.com/m1iktea/hearth/server/internal/collector/proxmox"
	"github.com/m1iktea/hearth/server/internal/store"
)

type fakeStore struct {
	mu           sync.Mutex
	samples      []store.MetricSample
	snapshots    []collector.Snapshot
	systemEvents []string
	uptimes      map[string]float64
}

func (f *fakeStore) InsertSamples(samples []store.MetricSample) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.samples = append(f.samples, samples...)
	return nil
}
func (f *fakeStore) UpsertSnapshot(snap collector.Snapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snapshots = append(f.snapshots, snap)
	return nil
}
func (f *fakeStore) InsertSystemEvent(source, object, eventType, _, _, _ string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.systemEvents = append(f.systemEvents, source+"/"+object+"/"+eventType)
	return nil
}
func (f *fakeStore) LastUptimes() (map[string]float64, error) {
	if f.uptimes == nil {
		return nil, errors.New("no baseline")
	}
	return f.uptimes, nil
}

func pveData(uptime int64) proxmox.Data {
	return proxmox.Data{Nodes: []proxmox.Node{{
		Name: "pve-01", Status: "online", CPU: 0.235, Mem: 890, MaxMem: 1000, Uptime: uptime,
	}}}
}

func newTestRecorder(db Store, interval time.Duration) *Recorder {
	return NewRecorder(store.NewSnapshotStore(), db, interval, slog.New(slog.NewTextHandler(testWriter{}, nil)))
}

type testWriter struct{}

func (testWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestRecorderThrottlesByInterval(t *testing.T) {
	db := &fakeStore{uptimes: map[string]float64{}}
	r := newTestRecorder(db, time.Minute)
	base := time.Now()

	r.SetOK("proxmox", pveData(1000), base)
	r.SetOK("proxmox", pveData(1010), base.Add(10*time.Second))
	r.SetOK("proxmox", pveData(1070), base.Add(70*time.Second))

	// 首次 + 超过采样间隔的第三次落盘；第二次被节流
	if len(db.snapshots) != 2 {
		t.Fatalf("snapshots persisted = %d", len(db.snapshots))
	}
	var uptimes []float64
	for _, m := range db.samples {
		if m.Metric == "uptime_sec" {
			uptimes = append(uptimes, m.Value)
		}
	}
	if len(uptimes) != 2 || uptimes[0] != 1000 || uptimes[1] != 1070 {
		t.Fatalf("uptime samples = %v", uptimes)
	}
}

func TestRecorderPersistsOnStatusFlip(t *testing.T) {
	db := &fakeStore{uptimes: map[string]float64{}}
	r := newTestRecorder(db, time.Minute)
	base := time.Now()

	r.SetOK("proxmox", pveData(1000), base)
	r.SetError("proxmox", errors.New("timeout"), base.Add(10*time.Second))

	if len(db.snapshots) != 2 {
		t.Fatalf("snapshots persisted = %d", len(db.snapshots))
	}
	last := db.snapshots[len(db.snapshots)-1]
	if last.Status != collector.StatusOffline || last.LastError != "timeout" {
		t.Fatalf("offline snapshot = %+v", last)
	}
}

func TestRecorderDetectsRebootAcrossRestart(t *testing.T) {
	// 基线来自持久层：模拟 Hearth 自身重启后仍能发现节点重启
	db := &fakeStore{uptimes: map[string]float64{"proxmox/pve-01": 500_000}}
	r := newTestRecorder(db, time.Minute)

	r.SetOK("proxmox", pveData(120), time.Now())

	if len(db.systemEvents) != 1 || db.systemEvents[0] != "proxmox/pve-01/reboot" {
		t.Fatalf("system events = %v", db.systemEvents)
	}
}

func TestRecorderNoRebootWhenUptimeGrows(t *testing.T) {
	db := &fakeStore{uptimes: map[string]float64{"proxmox/pve-01": 100}}
	r := newTestRecorder(db, time.Minute)

	r.SetOK("proxmox", pveData(200), time.Now())

	if len(db.systemEvents) != 0 {
		t.Fatalf("unexpected reboot events: %v", db.systemEvents)
	}
}
