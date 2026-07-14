package metrics

import (
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
	dockercol "github.com/m1iktea/hearth/server/internal/collector/docker"
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

func ptr64(v float64) *float64 { return &v }

func hasSample(samples []store.MetricSample, object, metric string) bool {
	for _, s := range samples {
		if s.Object == object && s.Metric == metric {
			return true
		}
	}
	return false
}

func TestExtractDockerContainerMetrics(t *testing.T) {
	now := time.Now()

	// running 容器：CpuPct 非 nil + MemLimit>0 → 两个样本都应产生
	t.Run("running container with cpu and mem", func(t *testing.T) {
		data := dockercol.Data{
			Containers: []dockercol.Container{
				{Name: "web", State: "running", CpuPct: ptr64(12.5), MemUsed: 256, MemLimit: 1024},
			},
		}
		samples := extract("docker", data, now)
		if !hasSample(samples, "web", "cpu_pct") {
			t.Error("expected cpu_pct sample for running container")
		}
		if !hasSample(samples, "web", "mem_pct") {
			t.Error("expected mem_pct sample for running container")
		}
	})

	// running 容器：CpuPct=nil → 不产生 cpu_pct
	t.Run("running container without cpu baseline", func(t *testing.T) {
		data := dockercol.Data{
			Containers: []dockercol.Container{
				{Name: "db", State: "running", CpuPct: nil, MemUsed: 100, MemLimit: 512},
			},
		}
		samples := extract("docker", data, now)
		if hasSample(samples, "db", "cpu_pct") {
			t.Error("should not produce cpu_pct when CpuPct is nil")
		}
		if !hasSample(samples, "db", "mem_pct") {
			t.Error("expected mem_pct sample even without cpu baseline")
		}
	})

	// exited 容器 → 不产生任何容器级样本
	t.Run("exited container produces no per-container samples", func(t *testing.T) {
		data := dockercol.Data{
			Containers: []dockercol.Container{
				{Name: "idle", State: "exited", CpuPct: ptr64(0), MemUsed: 0, MemLimit: 512},
			},
		}
		samples := extract("docker", data, now)
		if hasSample(samples, "idle", "cpu_pct") {
			t.Error("exited container should not produce cpu_pct")
		}
		if hasSample(samples, "idle", "mem_pct") {
			t.Error("exited container should not produce mem_pct")
		}
	})
}
