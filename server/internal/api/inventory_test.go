package api

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestDeviceAndHealthCheckLifecycle(t *testing.T) {
	h, _, _ := newTestRouter(t)
	code, env := doJSON(t, h, "POST", "/api/v1/devices", `{"name":"NAS","kind":"nas","ip_address":"192.168.1.10","enabled":true}`)
	if code != 200 || !env.Success { t.Fatalf("create device: %d %+v", code, env) }
	var device struct{ ID int64 `json:"id"` }; json.Unmarshal(env.Data, &device)
	code, env = doJSON(t, h, "POST", fmt.Sprintf("/api/v1/devices/%d/checks", device.ID), `{"name":"web","type":"tcp","port":443,"enabled":true}`)
	if code != 200 || !env.Success { t.Fatalf("create check: %d %+v", code, env) }
	code, env = doJSON(t, h, "GET", fmt.Sprintf("/api/v1/devices/%d", device.ID), "")
	if code != 200 || !env.Success { t.Fatalf("get detail: %d %+v", code, env) }
	var detail struct { Device struct { Name string `json:"name"` } `json:"device"`; Checks []struct { Type string `json:"type"`; Port int `json:"port"` } `json:"checks"` }
	json.Unmarshal(env.Data, &detail)
	if detail.Device.Name != "NAS" || len(detail.Checks) != 1 || detail.Checks[0].Type != "tcp" || detail.Checks[0].Port != 443 { t.Fatalf("detail: %s", env.Data) }
	if code, _ = doJSON(t, h, "POST", fmt.Sprintf("/api/v1/devices/%d/checks", device.ID), `{"type":"tcp","port":0}`); code != 400 { t.Errorf("invalid tcp port: %d", code) }
}
