package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/m1iktea/hearth/server/internal/discovery"
	"github.com/m1iktea/hearth/server/internal/store"
)

func newTestRouter(t *testing.T) (http.Handler, *store.SnapshotStore, *store.NavStore) {
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
	return NewRouter(snaps, nav, inventory, discovery.NewARPScanner(nil), dist, slog.Default()), snaps, nav
}

type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

func doJSON(t *testing.T, h http.Handler, method, path, body string) (int, envelope) {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var env envelope
	json.Unmarshal(rec.Body.Bytes(), &env)
	return rec.Code, env
}

func TestHealthz(t *testing.T) {
	h, _, _ := newTestRouter(t)
	code, env := doJSON(t, h, "GET", "/api/v1/healthz", "")
	if code != 200 || !env.Success {
		t.Fatalf("code=%d env=%+v", code, env)
	}
}

func TestStatusAll(t *testing.T) {
	h, snaps, _ := newTestRouter(t)
	snaps.SetOK("proxmox", map[string]string{"k": "v"}, time.Now())

	code, env := doJSON(t, h, "GET", "/api/v1/status", "")
	if code != 200 || !env.Success {
		t.Fatalf("code=%d env=%+v", code, env)
	}
	var list []map[string]any
	json.Unmarshal(env.Data, &list)
	if len(list) != 1 || list[0]["source"] != "proxmox" {
		t.Errorf("data = %s", env.Data)
	}
}

func TestStatusBySource(t *testing.T) {
	h, snaps, _ := newTestRouter(t)
	snaps.SetOK("docker", nil, time.Now())

	code, _ := doJSON(t, h, "GET", "/api/v1/status/docker", "")
	if code != 200 {
		t.Errorf("existing source: code=%d", code)
	}
	code, env := doJSON(t, h, "GET", "/api/v1/status/nope", "")
	if code != 404 || env.Success {
		t.Errorf("missing source: code=%d env=%+v", code, env)
	}
}

func TestSPAFallback(t *testing.T) {
	h, _, _ := newTestRouter(t)
	req := httptest.NewRequest("GET", "/nav", nil) // 前端路由路径
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "hearth") {
		t.Errorf("code=%d body=%q", rec.Code, rec.Body.String())
	}
}
