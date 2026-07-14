package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
)

func openTestInventory(t *testing.T) *InventoryStore {
	t.Helper()
	s, err := OpenInventory(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSamplesInsertQueryAndPrune(t *testing.T) {
	s := openTestInventory(t)
	now := time.Now().UTC()
	old := now.Add(-40 * 24 * time.Hour)
	samples := []MetricSample{
		{Source: "proxmox", Object: "pve-01", Metric: "cpu_pct", Value: 23.5, CreatedAt: now},
		{Source: "proxmox", Object: "pve-01", Metric: "mem_pct", Value: 89, CreatedAt: now},
		{Source: "openwrt", Object: "router", Metric: "load1", Value: 0.35, CreatedAt: now},
		{Source: "proxmox", Object: "pve-01", Metric: "cpu_pct", Value: 10, CreatedAt: old},
	}
	if err := s.InsertSamples(samples); err != nil {
		t.Fatal(err)
	}

	got, err := s.QuerySamples("proxmox", "pve-01", "cpu_pct", now.Add(-time.Hour), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Value != 23.5 {
		t.Fatalf("QuerySamples = %+v", got)
	}

	all, err := s.QuerySamples("", "", "", old.Add(-time.Hour), 0)
	if err != nil || len(all) != 4 {
		t.Fatalf("all samples = %d, err = %v", len(all), err)
	}

	n, err := s.PruneBefore(now.Add(-90*24*time.Hour), now.Add(-30*24*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("PruneBefore n=%d err=%v", n, err)
	}
}

func TestLastUptimes(t *testing.T) {
	s := openTestInventory(t)
	base := time.Now().UTC().Add(-time.Hour)
	err := s.InsertSamples([]MetricSample{
		{Source: "proxmox", Object: "pve-01", Metric: "uptime_sec", Value: 1000, CreatedAt: base},
		{Source: "proxmox", Object: "pve-01", Metric: "uptime_sec", Value: 2000, CreatedAt: base.Add(time.Minute)},
		{Source: "openwrt", Object: "router", Metric: "uptime_sec", Value: 500, CreatedAt: base},
		{Source: "proxmox", Object: "pve-01", Metric: "cpu_pct", Value: 99, CreatedAt: base.Add(2 * time.Minute)},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.LastUptimes()
	if err != nil {
		t.Fatal(err)
	}
	if got["proxmox/pve-01"] != 2000 || got["openwrt/router"] != 500 {
		t.Fatalf("LastUptimes = %v", got)
	}
}

func TestSnapshotRoundTrip(t *testing.T) {
	s := openTestInventory(t)
	at := time.Now().UTC().Truncate(time.Second)
	snap := collector.Snapshot{
		Source: "docker", Status: "offline", CollectedAt: at,
		LastError: "connection refused",
		Data:      map[string]any{"containers": []any{map[string]any{"name": "immich"}}},
	}
	if err := s.UpsertSnapshot(snap); err != nil {
		t.Fatal(err)
	}
	// 覆盖写不产生第二行
	snap.Status = "online"
	snap.LastError = ""
	if err := s.UpsertSnapshot(snap); err != nil {
		t.Fatal(err)
	}
	got, err := s.LoadSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Source != "docker" || got[0].Status != "online" {
		t.Fatalf("LoadSnapshots = %+v", got)
	}
	if got[0].Data == nil {
		t.Fatal("snapshot data lost")
	}
}

func TestSystemEventsMergedIntoListEvents(t *testing.T) {
	s := openTestInventory(t)
	now := time.Now().UTC()
	if err := s.InsertSystemEvent("proxmox", "pve-01", "reboot", "warning", "节点重启", "检测到重启", now); err != nil {
		t.Fatal(err)
	}
	// 一条常规设备事件对比排序与字段
	d, err := s.CreateDevice(Device{Name: "NAS", Kind: "nas", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.CreateCheck(HealthCheck{DeviceID: d.ID, Name: "ping", Type: "ping", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordProbe(CheckWithDevice{HealthCheck: c, DeviceName: d.Name}, "offline", "timeout", 0, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	events, err := s.ListEvents(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d", len(events))
	}
	if events[0].Type != "offline" || events[1].Type != "reboot" {
		t.Fatalf("order = %s, %s", events[0].Type, events[1].Type)
	}
	if events[1].ID >= 0 {
		t.Fatalf("system event id should be negative, got %d", events[1].ID)
	}
	if events[1].DeviceName != "pve-01" {
		t.Fatalf("system event device_name = %q", events[1].DeviceName)
	}
}
