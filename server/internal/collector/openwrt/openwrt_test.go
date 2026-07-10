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
		case object == "network.interface" && method == "dump":
			reply(`{"interface":[
				{"interface":"loopback","up":true,"uptime":86400,"l3_device":"lo","proto":"static","device":"lo","ipv4-address":[{"address":"127.0.0.1","mask":8}]},
				{"interface":"lan","up":true,"uptime":12345,"l3_device":"br-lan","proto":"static","device":"br-lan","ipv4-address":[{"address":"192.168.1.1","mask":24}]},
				{"interface":"wan","up":true,"uptime":12000,"l3_device":"eth1","proto":"dhcp","device":"eth1","ipv4-address":[{"address":"10.0.0.2","mask":24}]}
			]}`)
		case object == "network.device" && method == "status":
			reply(`{
				"lo":     {"up":true,"carrier":true,"statistics":{"rx_bytes":100,"tx_bytes":100}},
				"br-lan": {"up":true,"carrier":true,"statistics":{"rx_bytes":1000,"tx_bytes":2000}},
				"eth1":   {"up":true,"carrier":true,"statistics":{"rx_bytes":3000,"tx_bytes":4000}}
			}`)
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

	// interfaces: loopback skipped, expect lan + wan
	if len(data.Interfaces) != 2 {
		t.Fatalf("interfaces: want 2, got %d: %+v", len(data.Interfaces), data.Interfaces)
	}
	lan := data.Interfaces[0]
	if lan.Name != "lan" || !lan.Up || lan.Device != "br-lan" || lan.IPv4 != "192.168.1.1" {
		t.Errorf("lan interface: %+v", lan)
	}
	if lan.RxBytes != 1000 || lan.TxBytes != 2000 {
		t.Errorf("lan traffic: rx=%d tx=%d", lan.RxBytes, lan.TxBytes)
	}
	wan := data.Interfaces[1]
	if wan.Name != "wan" || wan.Device != "eth1" || wan.IPv4 != "10.0.0.2" {
		t.Errorf("wan interface: %+v", wan)
	}
	if wan.RxBytes != 3000 || wan.TxBytes != 4000 {
		t.Errorf("wan traffic: rx=%d tx=%d", wan.RxBytes, wan.TxBytes)
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
