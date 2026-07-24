package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"testing/fstest"
	"time"

	"github.com/m1iktea/hearth/server/internal/discovery"
	"github.com/m1iktea/hearth/server/internal/store"
)

func TestHealthTimelineEndpoint(t *testing.T) {
	inv, err := store.OpenInventory(t.TempDir() + "/inv.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { inv.Close() })
	nav, err := store.OpenNav(t.TempDir() + "/nav.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { nav.Close() })

	d, err := inv.CreateDevice(store.Device{Name: "NAS", Kind: "nas", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	c, err := inv.CreateCheck(store.HealthCheck{DeviceID: d.ID, Name: "ping", Type: "ping", Target: "10.0.0.9", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	cwd := store.CheckWithDevice{HealthCheck: c, DeviceName: d.Name}
	now := time.Now().UTC()
	for _, tr := range []struct {
		status, reason string
		at             time.Time
	}{
		{"online", "", now.Add(-3 * time.Hour)},
		{"offline", "connection refused", now.Add(-2 * time.Hour)},
		{"online", "", now.Add(-30 * time.Minute)},
	} {
		if _, err := inv.RecordHealthTransition(cwd, tr.status, tr.reason, 0, tr.at); err != nil {
			t.Fatal(err)
		}
	}

	dist := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>hearth</html>")}}
	h := NewRouter(store.NewSnapshotStore(), nav, inv, discovery.NewARPScanner(nil), dist, slog.Default())

	// 缺 check_id → 400
	if code, env := doJSON(t, h, "GET", "/api/v1/health/timeline", ""); code != 400 || env.Success {
		t.Fatalf("缺 check_id 应 400: code=%d env=%+v", code, env)
	}

	// 非法 check_id → 400
	if code, _ := doJSON(t, h, "GET", "/api/v1/health/timeline?check_id=abc", ""); code != 400 {
		t.Fatalf("非法 check_id 应 400: %d", code)
	}

	// 默认 24h 窗口：3 条，升序，携带原因
	code, env := doJSON(t, h, "GET", fmt.Sprintf("/api/v1/health/timeline?check_id=%d", c.ID), "")
	if code != 200 || !env.Success {
		t.Fatalf("code=%d env=%+v", code, env)
	}
	var got []store.HealthTransition
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("应返回 3 条迁移: %d", len(got))
	}
	if got[0].Status != "online" || got[1].Status != "offline" || got[2].Status != "online" {
		t.Fatalf("顺序错误: %+v", got)
	}
	if got[1].Reason != "connection refused" {
		t.Fatalf("原因缺失: %q", got[1].Reason)
	}

	// since 过滤：最近 1h → 窗口内 online(-30min) + 窗口前锚点 offline(-2h)，共 2 条升序
	code, env = doJSON(t, h, "GET", fmt.Sprintf("/api/v1/health/timeline?check_id=%d&since=%s", c.ID, now.Add(-time.Hour).Format(time.RFC3339)), "")
	if code != 200 {
		t.Fatalf("code=%d", code)
	}
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Status != "offline" || got[1].Status != "online" {
		t.Fatalf("since 过滤后应为锚点 offline + 窗口内 online: %+v", got)
	}

	// 非法 since → 400
	if code, _ := doJSON(t, h, "GET", fmt.Sprintf("/api/v1/health/timeline?check_id=%d&since=nope", c.ID), ""); code != 400 {
		t.Fatalf("非法 since 应 400: %d", code)
	}
}
