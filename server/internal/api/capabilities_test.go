package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/m1iktea/hearth/server/internal/discovery"
	"github.com/m1iktea/hearth/server/internal/store"
)

// newTestRouterWithScanner 允许注入 nil scanner，覆盖 ARP 未启用的部署形态。
func newTestRouterWithScanner(t *testing.T, scanner *discovery.ARPScanner) http.Handler {
	t.Helper()
	snaps := store.NewSnapshotStore()
	nav, err := store.OpenNav(t.TempDir() + "/nav.db")
	if err != nil {
		t.Fatalf("OpenNav: %v", err)
	}
	t.Cleanup(func() { nav.Close() })
	inventory, err := store.OpenInventory(t.TempDir() + "/inventory.db")
	if err != nil {
		t.Fatalf("OpenInventory: %v", err)
	}
	t.Cleanup(func() { inventory.Close() })
	dist := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>hearth</html>")}}
	return NewRouter(snaps, nav, inventory, scanner, dist, slog.Default())
}

func TestCapabilities(t *testing.T) {
	tests := []struct {
		name    string
		scanner *discovery.ARPScanner
		want    bool
	}{
		{name: "arp disabled", scanner: nil, want: false},
		{name: "arp enabled", scanner: discovery.NewARPScanner(nil), want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestRouterWithScanner(t, tt.scanner)
			code, env := doJSON(t, h, http.MethodGet, "/api/v1/capabilities", "")
			if code != http.StatusOK || !env.Success {
				t.Fatalf("code=%d success=%v error=%q", code, env.Success, env.Error)
			}
			var caps map[string]bool
			if err := json.Unmarshal(env.Data, &caps); err != nil {
				t.Fatalf("decode capabilities: %v", err)
			}
			if caps["arp_discovery"] != tt.want {
				t.Errorf("arp_discovery = %v, want %v", caps["arp_discovery"], tt.want)
			}
		})
	}
}

// 回归：ARP 未启用时 main 传入的是 *ARPScanner 的 nil。此前 typed-nil 塞进
// 接口后绕过 handler 的保护，在 nil 接收者上 panic 导致空响应体。
func TestScanARPDisabledReturnsServiceUnavailable(t *testing.T) {
	h := newTestRouterWithScanner(t, nil)
	code, env := doJSON(t, h, http.MethodPost, "/api/v1/discovery/arp", "{}")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", code)
	}
	if env.Success || env.Error == "" {
		t.Fatalf("want json error envelope, got success=%v error=%q", env.Success, env.Error)
	}
}
