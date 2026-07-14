package docker

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCollectTCP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/json" || r.URL.Query().Get("all") != "1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`[{"Id":"abc123","Names":["/hearth"],"Image":"hearth:latest","State":"running","Status":"Up 2 hours"}]`))
	}))
	defer srv.Close()

	c, err := New("tcp://" + strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	data := got.(Data)
	if len(data.Containers) != 1 {
		t.Fatalf("containers = %+v", data.Containers)
	}
	ct := data.Containers[0]
	if ct.Name != "hearth" || ct.State != "running" { // Names 前导 / 已去除
		t.Errorf("got %+v", ct)
	}
}

func TestNewUnknownScheme(t *testing.T) {
	if _, err := New("ftp://x"); err == nil {
		t.Fatal("want error for unknown scheme")
	}
}

func TestName(t *testing.T) {
	c, _ := New("unix:///var/run/docker.sock")
	if c.Name() != "docker" {
		t.Errorf("Name() = %q", c.Name())
	}
}

// statsJSON 生成一个 stats 响应 JSON。
func statsJSON(totalUsage, systemUsage uint64, onlineCPUs int, memUsage, memLimit uint64, memStats map[string]uint64) string {
	statsEntries := ""
	for k, v := range memStats {
		if statsEntries != "" {
			statsEntries += ","
		}
		statsEntries += fmt.Sprintf(`%q:%d`, k, v)
	}
	return fmt.Sprintf(`{
		"cpu_stats": {
			"cpu_usage": {"total_usage": %d},
			"system_cpu_usage": %d,
			"online_cpus": %d
		},
		"memory_stats": {
			"usage": %d,
			"limit": %d,
			"stats": {%s}
		}
	}`, totalUsage, systemUsage, onlineCPUs, memUsage, memLimit, statsEntries)
}

// TestFetchStatsNormal 验证两轮采集后 CPU 差分能正确计算出非 nil 正数。
func TestFetchStatsNormal(t *testing.T) {
	// 第一轮 stats 数据
	round := 0
	statsRounds := []struct {
		total  uint64
		system uint64
	}{
		{total: 1_000_000, system: 100_000_000},
		{total: 2_000_000, system: 200_000_000},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/containers/json":
			w.Write([]byte(`[{"Id":"ctr1","Names":["/app"],"Image":"app:latest","State":"running","Status":"Up 1 hour"}]`))
		case strings.HasPrefix(r.URL.Path, "/containers/ctr1/stats"):
			idx := round
			if idx >= len(statsRounds) {
				idx = len(statsRounds) - 1
			}
			d := statsRounds[idx]
			w.Write([]byte(statsJSON(d.total, d.system, 2, 2048, 8192, map[string]uint64{"inactive_file": 512})))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, err := New("tcp://" + strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// 第一轮：无基线，CpuPct 应为 nil
	round = 0
	got1, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect round1: %v", err)
	}
	data1 := got1.(Data)
	if len(data1.Containers) != 1 {
		t.Fatalf("round1: expected 1 container")
	}
	if data1.Containers[0].CpuPct != nil {
		t.Errorf("round1: CpuPct should be nil (no baseline), got %v", data1.Containers[0].CpuPct)
	}

	// 第二轮：有基线，CpuPct 应为正数
	round = 1
	got2, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect round2: %v", err)
	}
	data2 := got2.(Data)
	if len(data2.Containers) != 1 {
		t.Fatalf("round2: expected 1 container")
	}
	ct := data2.Containers[0]
	if ct.CpuPct == nil {
		t.Fatalf("round2: CpuPct should be non-nil")
	}
	if *ct.CpuPct <= 0 {
		t.Errorf("round2: CpuPct should be positive, got %v", *ct.CpuPct)
	}
}

// TestFetchStatsFailDoesNotAffectOthers 验证一个容器 stats 返回 500 不影响另一个容器。
func TestFetchStatsFailDoesNotAffectOthers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/containers/json":
			w.Write([]byte(`[
				{"Id":"ok1","Names":["/ok"],"Image":"ok:latest","State":"running","Status":"Up 1 hour"},
				{"Id":"bad1","Names":["/bad"],"Image":"bad:latest","State":"running","Status":"Up 1 hour"}
			]`))
		case strings.HasPrefix(r.URL.Path, "/containers/ok1/stats"):
			w.Write([]byte(statsJSON(500_000, 50_000_000, 2, 1024, 4096, map[string]uint64{"inactive_file": 100})))
		case strings.HasPrefix(r.URL.Path, "/containers/bad1/stats"):
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, err := New("tcp://" + strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// Collect 不应返回 error
	got, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect should not return error even if one stats fails: %v", err)
	}
	data := got.(Data)
	if len(data.Containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(data.Containers))
	}

	// 两个容器第一轮 CpuPct 均应为 nil
	for _, ct := range data.Containers {
		if ct.CpuPct != nil {
			t.Errorf("container %s: round1 CpuPct should be nil, got %v", ct.ID, ct.CpuPct)
		}
	}
}

// TestMemoryV2AndV1 验证内存口径：cgroup v2 用 inactive_file，v1 回退到 cache。
func TestMemoryV2AndV1(t *testing.T) {
	tests := []struct {
		name     string
		memStats map[string]uint64
		usage    uint64
		wantUsed int64
	}{
		{
			name:     "cgroup v2 inactive_file",
			memStats: map[string]uint64{"inactive_file": 100},
			usage:    1100,
			wantUsed: 1000,
		},
		{
			name:     "cgroup v1 cache fallback",
			memStats: map[string]uint64{"cache": 200},
			usage:    1200,
			wantUsed: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/containers/json":
					w.Write([]byte(`[{"Id":"mem1","Names":["/mem"],"Image":"mem:latest","State":"running","Status":"Up 1 hour"}]`))
				case strings.HasPrefix(r.URL.Path, "/containers/mem1/stats"):
					w.Write([]byte(statsJSON(1_000_000, 100_000_000, 1, tt.usage, 65536, tt.memStats)))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			c, err := New("tcp://" + strings.TrimPrefix(srv.URL, "http://"))
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			got, err := c.Collect(context.Background())
			if err != nil {
				t.Fatalf("Collect: %v", err)
			}
			data := got.(Data)
			if len(data.Containers) != 1 {
				t.Fatalf("expected 1 container")
			}
			ct := data.Containers[0]
			if ct.MemUsed != tt.wantUsed {
				t.Errorf("MemUsed = %d, want %d", ct.MemUsed, tt.wantUsed)
			}
		})
	}
}
