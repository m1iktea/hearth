// Package metrics 实现“黑匣子”：在采集链路上旁路记录关键指标与最新快照，
// 使 Hearth 自身随宿主机崩溃重启后，仍能回看故障前的资源走势和最后已知状态。
package metrics

import (
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
	dockercol "github.com/m1iktea/hearth/server/internal/collector/docker"
	"github.com/m1iktea/hearth/server/internal/collector/openwrt"
	"github.com/m1iktea/hearth/server/internal/collector/proxmox"
	"github.com/m1iktea/hearth/server/internal/store"
)

// Store 是 Recorder 对持久层的最小依赖（由 store.InventoryStore 满足）。
type Store interface {
	InsertSamples(samples []store.MetricSample) error
	UpsertSnapshot(snap collector.Snapshot) error
	InsertSystemEvent(source, object, eventType, severity, title, message string, at time.Time) error
	LastUptimes() (map[string]float64, error)
}

// Recorder 包装 SnapshotStore 作为采集调度器的落点：
// 实时状态照常进内存，同时按 sampleInterval 节流落盘。
type Recorder struct {
	snaps    *store.SnapshotStore
	db       Store
	logger   *slog.Logger
	interval time.Duration

	mu          sync.Mutex
	lastPersist map[string]time.Time
	lastStatus  map[string]string
	lastUptime  map[string]float64
}

func NewRecorder(snaps *store.SnapshotStore, db Store, sampleInterval time.Duration, logger *slog.Logger) *Recorder {
	r := &Recorder{
		snaps:       snaps,
		db:          db,
		logger:      logger,
		interval:    sampleInterval,
		lastPersist: map[string]time.Time{},
		lastStatus:  map[string]string{},
		lastUptime:  map[string]float64{},
	}
	// 用上次落盘的 uptime 做基线，跨进程重启仍能检测到设备重启。
	if uptimes, err := db.LastUptimes(); err != nil {
		logger.Warn("load uptime baseline failed", "error", err)
	} else {
		r.lastUptime = uptimes
	}
	return r
}

func (r *Recorder) SetOK(source string, data any, at time.Time) {
	r.snaps.SetOK(source, data, at)
	r.persist(source, data, at, collector.StatusOnline)
}

func (r *Recorder) SetError(source string, err error, at time.Time) {
	r.snaps.SetError(source, err, at)
	r.persist(source, nil, at, collector.StatusOffline)
}

// persist 决定本次采集是否落盘：到达采样间隔，或在线/离线状态翻转时立即写。
func (r *Recorder) persist(source string, data any, at time.Time, status string) {
	r.mu.Lock()
	statusChanged := r.lastStatus[source] != "" && r.lastStatus[source] != status
	due := at.Sub(r.lastPersist[source]) >= r.interval
	r.lastStatus[source] = status
	if !due && !statusChanged {
		r.mu.Unlock()
		return
	}
	r.lastPersist[source] = at

	var samples []store.MetricSample
	var reboots []rebootInfo
	if data != nil {
		samples = extract(source, data, at)
		reboots = r.detectReboots(samples)
	}
	r.mu.Unlock()

	if snap, ok := r.snaps.Get(source); ok {
		if err := r.db.UpsertSnapshot(snap); err != nil {
			r.logger.Warn("persist snapshot failed", "source", source, "error", err)
		}
	}
	if err := r.db.InsertSamples(samples); err != nil {
		r.logger.Warn("persist samples failed", "source", source, "error", err)
	}
	for _, reboot := range reboots {
		message := fmt.Sprintf("%s 检测到重启：运行时长从 %s 回落到 %s",
			reboot.object, formatDuration(reboot.previous), formatDuration(reboot.current))
		if err := r.db.InsertSystemEvent(source, reboot.object, "reboot", "warning", "节点重启", message, at); err != nil {
			r.logger.Warn("record reboot event failed", "source", source, "error", err)
		} else {
			r.logger.Info("node reboot detected", "source", source, "object", reboot.object)
		}
	}
}

type rebootInfo struct {
	object   string
	previous float64
	current  float64
}

// detectReboots 依据 uptime 回落判定重启。调用方需持有 r.mu。
func (r *Recorder) detectReboots(samples []store.MetricSample) []rebootInfo {
	var out []rebootInfo
	for _, m := range samples {
		if m.Metric != "uptime_sec" {
			continue
		}
		key := m.Source + "/" + m.Object
		if prev, ok := r.lastUptime[key]; ok && m.Value < prev {
			out = append(out, rebootInfo{object: m.Object, previous: prev, current: m.Value})
		}
		r.lastUptime[key] = m.Value
	}
	return out
}

// extract 从各源快照中抽取值得留存的硬件/运行指标。
func extract(source string, data any, at time.Time) []store.MetricSample {
	add := func(out []store.MetricSample, object, metric string, value float64) []store.MetricSample {
		return append(out, store.MetricSample{Source: source, Object: object, Metric: metric, Value: value, CreatedAt: at})
	}
	var out []store.MetricSample
	switch d := data.(type) {
	case proxmox.Data:
		for _, node := range d.Nodes {
			out = add(out, node.Name, "cpu_pct", round1(node.CPU*100))
			if node.MaxMem > 0 {
				out = add(out, node.Name, "mem_pct", round1(float64(node.Mem)/float64(node.MaxMem)*100))
			}
			out = add(out, node.Name, "uptime_sec", float64(node.Uptime))
			running := 0
			for _, vm := range node.VMs {
				if vm.Status == "running" {
					running++
				}
			}
			out = add(out, node.Name, "vms_running", float64(running))
		}
	case dockercol.Data:
		running := 0
		for _, c := range d.Containers {
			if c.State == "running" {
				running++
			}
		}
		out = add(out, "docker", "containers_running", float64(running))
		out = add(out, "docker", "containers_total", float64(len(d.Containers)))
		// 追加每个 running 容器的资源指标
		for _, c := range d.Containers {
			if c.State != "running" {
				continue
			}
			if c.CpuPct != nil {
				out = add(out, c.Name, "cpu_pct", round1(*c.CpuPct))
			}
			if c.MemLimit > 0 {
				memPct := float64(c.MemUsed) / float64(c.MemLimit) * 100
				out = add(out, c.Name, "mem_pct", round1(memPct))
			}
		}
	case openwrt.Data:
		object := d.Hostname
		if object == "" {
			object = "openwrt"
		}
		if d.Memory.Total > 0 {
			used := float64(d.Memory.Total-d.Memory.Available) / float64(d.Memory.Total) * 100
			out = add(out, object, "mem_used_pct", round1(used))
		}
		out = add(out, object, "load1", round2(d.Load[0]))
		out = add(out, object, "uptime_sec", float64(d.UptimeSec))
	}
	return out
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }

func formatDuration(seconds float64) string {
	d := time.Duration(seconds) * time.Second
	if d >= 24*time.Hour {
		return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
	}
	if d >= time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
