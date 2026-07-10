package proxmox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newFakePVE(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api2/json/nodes", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "PVEAPIToken=root@pam!hearth=secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"data":[{"node":"pve","status":"online","cpu":0.02,"mem":100,"maxmem":200,"uptime":86400}]}`))
	})
	mux.HandleFunc("GET /api2/json/nodes/pve/qemu", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"vmid":100,"name":"fnos","status":"running","cpu":0.05,"mem":50,"maxmem":80,"uptime":3600}]}`))
	})
	return httptest.NewServer(mux)
}

func TestCollect(t *testing.T) {
	srv := newFakePVE(t)
	defer srv.Close()

	c := New(srv.URL, "root@pam!hearth", "secret")
	got, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	data := got.(Data)
	if len(data.Nodes) != 1 || data.Nodes[0].Name != "pve" {
		t.Fatalf("nodes = %+v", data.Nodes)
	}
	n := data.Nodes[0]
	if len(n.VMs) != 1 || n.VMs[0].VMID != 100 || n.VMs[0].Status != "running" {
		t.Errorf("vms = %+v", n.VMs)
	}
}

func TestCollectAuthFailure(t *testing.T) {
	srv := newFakePVE(t)
	defer srv.Close()

	c := New(srv.URL, "root@pam!hearth", "wrong")
	if _, err := c.Collect(context.Background()); err == nil {
		t.Fatal("want error on 401")
	}
}

func TestName(t *testing.T) {
	if got := New("u", "i", "s").Name(); got != "proxmox" {
		t.Errorf("Name() = %q", got)
	}
}
