package openwrt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newFakeUbus(t *testing.T, password string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ubus" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req struct {
			ID     int    `json:"id"`
			Params [4]any `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		object, _ := req.Params[1].(string)
		method, _ := req.Params[2].(string)

		reply := func(payload string) {
			w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[0,` + payload + `]}`))
		}
		switch {
		case object == "session" && method == "login":
			args, _ := req.Params[3].(map[string]any)
			if args["password"] != password {
				w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[6]}`)) // permission denied
				return
			}
			reply(`{"ubus_rpc_session":"sid-123"}`)
		case object == "system" && method == "board":
			if req.Params[0] != "sid-123" {
				w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[6]}`))
				return
			}
			reply(`{"hostname":"ImmortalWrt","model":"x86_64","release":{"distribution":"ImmortalWrt","version":"23.05"}}`)
		case object == "system" && method == "info":
			reply(`{"uptime":86400,"load":[65536,32768,16384],"memory":{"total":1000,"free":500,"available":600}}`)
		default:
			w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[2]}`))
		}
	}))
}

func TestCollect(t *testing.T) {
	srv := newFakeUbus(t, "pass")
	defer srv.Close()

	c := New(srv.URL, "root", "pass")
	got, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	data := got.(Data)
	if data.Hostname != "ImmortalWrt" || data.Release != "ImmortalWrt 23.05" {
		t.Errorf("board: %+v", data)
	}
	if data.UptimeSec != 86400 || data.Load[0] != 1.0 { // 65536/65536
		t.Errorf("info: %+v", data)
	}
	if data.Memory.Total != 1000 || data.Memory.Available != 600 {
		t.Errorf("memory: %+v", data.Memory)
	}
}

func TestCollectBadPassword(t *testing.T) {
	srv := newFakeUbus(t, "pass")
	defer srv.Close()

	c := New(srv.URL, "root", "wrong")
	if _, err := c.Collect(context.Background()); err == nil {
		t.Fatal("want login error")
	}
}

func TestName(t *testing.T) {
	if New("u", "a", "b").Name() != "openwrt" {
		t.Error("Name() != openwrt")
	}
}
