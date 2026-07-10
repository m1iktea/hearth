package store

import (
	"errors"
	"testing"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
)

func TestSetOKAndGet(t *testing.T) {
	s := NewSnapshotStore()
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	s.SetOK("proxmox", map[string]int{"nodes": 1}, at)

	snap, ok := s.Get("proxmox")
	if !ok {
		t.Fatal("snapshot not found")
	}
	if snap.Status != collector.StatusOnline || !snap.CollectedAt.Equal(at) {
		t.Errorf("got %+v", snap)
	}
}

func TestSetErrorKeepsPreviousData(t *testing.T) {
	s := NewSnapshotStore()
	at := time.Now()
	s.SetOK("docker", "old-data", at)
	s.SetError("docker", errors.New("boom"), at.Add(time.Second))

	snap, _ := s.Get("docker")
	if snap.Status != collector.StatusOffline {
		t.Errorf("Status = %q", snap.Status)
	}
	if snap.LastError != "boom" {
		t.Errorf("LastError = %q", snap.LastError)
	}
	if snap.Data != "old-data" { // 保留上次成功数据
		t.Errorf("Data = %v", snap.Data)
	}
}

func TestGetMissing(t *testing.T) {
	s := NewSnapshotStore()
	if _, ok := s.Get("nope"); ok {
		t.Fatal("want ok=false")
	}
}

func TestAllSortedBySource(t *testing.T) {
	s := NewSnapshotStore()
	now := time.Now()
	s.SetOK("proxmox", nil, now)
	s.SetOK("docker", nil, now)
	all := s.All()
	if len(all) != 2 || all[0].Source != "docker" || all[1].Source != "proxmox" {
		t.Errorf("got %+v", all)
	}
}
