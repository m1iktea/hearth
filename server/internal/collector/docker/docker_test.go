package docker

import (
	"context"
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
