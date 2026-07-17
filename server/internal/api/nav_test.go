package api

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestNavCategoryLifecycle(t *testing.T) {
	h, _, _ := newTestRouter(t)

	// create
	code, env := doJSON(t, h, "POST", "/api/v1/nav/categories", `{"name":"服务","sort_order":1}`)
	if code != 200 || !env.Success {
		t.Fatalf("create: code=%d env=%+v", code, env)
	}
	var cat struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(env.Data, &cat)

	// create item under it
	body := fmt.Sprintf(`{"category_id":%d,"name":"PVE","url":"https://pve:8006","icon":"server","sort_order":1}`, cat.ID)
	code, env = doJSON(t, h, "POST", "/api/v1/nav/items", body)
	if code != 200 || !env.Success {
		t.Fatalf("create item: code=%d env=%+v", code, env)
	}
	var item struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(env.Data, &item)

	// list nested
	code, env = doJSON(t, h, "GET", "/api/v1/nav", "")
	var cats []struct {
		Name  string `json:"name"`
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	json.Unmarshal(env.Data, &cats)
	if code != 200 || len(cats) != 1 || len(cats[0].Items) != 1 || cats[0].Items[0].Name != "PVE" {
		t.Fatalf("list: code=%d data=%s", code, env.Data)
	}

	// update item
	body = fmt.Sprintf(`{"category_id":%d,"name":"Proxmox","url":"https://pve:8006","icon":"","sort_order":2}`, cat.ID)
	code, _ = doJSON(t, h, "PUT", fmt.Sprintf("/api/v1/nav/items/%d", item.ID), body)
	if code != 200 {
		t.Fatalf("update item: code=%d", code)
	}

	// delete
	if code, _ = doJSON(t, h, "DELETE", fmt.Sprintf("/api/v1/nav/items/%d", item.ID), ""); code != 200 {
		t.Fatalf("delete item: code=%d", code)
	}
	if code, _ = doJSON(t, h, "DELETE", fmt.Sprintf("/api/v1/nav/categories/%d", cat.ID), ""); code != 200 {
		t.Fatalf("delete category: code=%d", code)
	}
}

func TestNavValidation(t *testing.T) {
	h, _, _ := newTestRouter(t)

	if code, _ := doJSON(t, h, "POST", "/api/v1/nav/categories", `{"name":""}`); code != 400 {
		t.Errorf("empty category name: code=%d", code)
	}
	if code, _ := doJSON(t, h, "POST", "/api/v1/nav/items", `{"category_id":1,"name":"x","url":""}`); code != 400 {
		t.Errorf("empty item url: code=%d", code)
	}
	if code, _ := doJSON(t, h, "POST", "/api/v1/nav/categories", `not-json`); code != 400 {
		t.Errorf("bad json: code=%d", code)
	}
	if code, _ := doJSON(t, h, "PUT", "/api/v1/nav/items/99999", `{"category_id":1,"name":"x","url":"http://x"}`); code != 404 {
		t.Errorf("update missing item: code=%d", code)
	}
	if code, _ := doJSON(t, h, "PUT", "/api/v1/nav/items/abc", `{}`); code != 400 {
		t.Errorf("non-numeric id: code=%d", code)
	}
}

func TestNavItemDeviceIDAPI(t *testing.T) {
	h, _, _ := newTestRouter(t)

	// 准备分类
	code, env := doJSON(t, h, "POST", "/api/v1/nav/categories", `{"name":"服务","sort_order":1}`)
	if code != 200 || !env.Success {
		t.Fatalf("create cat: %d %+v", code, env)
	}
	var cat struct{ ID int64 `json:"id"` }
	json.Unmarshal(env.Data, &cat)

	// 准备设备
	code, env = doJSON(t, h, "POST", "/api/v1/devices", `{"name":"NAS","kind":"nas","ip_address":"192.168.1.10","enabled":true}`)
	if code != 200 || !env.Success {
		t.Fatalf("create device: %d %+v", code, env)
	}
	var device struct{ ID int64 `json:"id"` }
	json.Unmarshal(env.Data, &device)

	body := fmt.Sprintf(`{"category_id":%d,"name":"NAS","url":"http://nas","device_id":%d}`, cat.ID, device.ID)

	// 绑定存在的设备
	code, env = doJSON(t, h, "POST", "/api/v1/nav/items", body)
	if code != 200 || !env.Success {
		t.Fatalf("create item with device_id: %d %+v", code, env)
	}
	var item struct {
		ID       int64  `json:"id"`
		DeviceID *int64 `json:"device_id"`
	}
	json.Unmarshal(env.Data, &item)
	if item.DeviceID == nil || *item.DeviceID != device.ID {
		t.Fatalf("device_id not set: %+v", item)
	}

	// 同一设备绑定第二个导航项应 409
	body2 := fmt.Sprintf(`{"category_id":%d,"name":"NAS2","url":"http://nas2","device_id":%d}`, cat.ID, device.ID)
	code, _ = doJSON(t, h, "POST", "/api/v1/nav/items", body2)
	if code != 409 {
		t.Errorf("duplicate device_id: want 409, got %d", code)
	}

	// 绑定不存在的设备应 422
	body3 := fmt.Sprintf(`{"category_id":%d,"name":"Ghost","url":"http://ghost","device_id":99999}`, cat.ID)
	code, _ = doJSON(t, h, "POST", "/api/v1/nav/items", body3)
	if code != 422 {
		t.Errorf("nonexistent device_id: want 422, got %d", code)
	}

	// --- PUT (update) 路径用例 ---

	// 再建一个设备，用于"已被其他导航项占用"的场景
	code, env = doJSON(t, h, "POST", "/api/v1/devices", `{"name":"Router","kind":"router","ip_address":"192.168.1.1","enabled":true}`)
	if code != 200 || !env.Success {
		t.Fatalf("create second device: %d %+v", code, env)
	}
	var device2 struct{ ID int64 `json:"id"` }
	json.Unmarshal(env.Data, &device2)

	// 用 device2 创建第二个导航项，使 device2 被占用
	body4 := fmt.Sprintf(`{"category_id":%d,"name":"Router","url":"http://router","device_id":%d}`, cat.ID, device2.ID)
	code, env = doJSON(t, h, "POST", "/api/v1/nav/items", body4)
	if code != 200 || !env.Success {
		t.Fatalf("create second item: %d %+v", code, env)
	}
	var item2 struct{ ID int64 `json:"id"` }
	json.Unmarshal(env.Data, &item2)

	// PUT：把 item 的 device_id 改成已被 item2 占用的 device2 → 期望 409
	bodyConflict := fmt.Sprintf(`{"category_id":%d,"name":"NAS","url":"http://nas","device_id":%d}`, cat.ID, device2.ID)
	code, _ = doJSON(t, h, "PUT", fmt.Sprintf("/api/v1/nav/items/%d", item.ID), bodyConflict)
	if code != 409 {
		t.Errorf("update with occupied device_id: want 409, got %d", code)
	}

	// PUT：把 item 的 device_id 改成不存在的设备 → 期望 422
	bodyMissing := fmt.Sprintf(`{"category_id":%d,"name":"NAS","url":"http://nas","device_id":99999}`, cat.ID)
	code, _ = doJSON(t, h, "PUT", fmt.Sprintf("/api/v1/nav/items/%d", item.ID), bodyMissing)
	if code != 422 {
		t.Errorf("update with nonexistent device_id: want 422, got %d", code)
	}

	// PUT：把 item 的 device_id 保持指向它自己已关联的 device（自我更新）→ 期望 200
	bodySelf := fmt.Sprintf(`{"category_id":%d,"name":"NAS","url":"http://nas","device_id":%d}`, cat.ID, device.ID)
	code, _ = doJSON(t, h, "PUT", fmt.Sprintf("/api/v1/nav/items/%d", item.ID), bodySelf)
	if code != 200 {
		t.Errorf("update self device_id: want 200, got %d", code)
	}
}

func TestNavCategoryUpdate(t *testing.T) {
	h, _, _ := newTestRouter(t)

	code, env := doJSON(t, h, "POST", "/api/v1/nav/categories", `{"name":"旧名","sort_order":1}`)
	if code != 200 || !env.Success {
		t.Fatalf("create: code=%d env=%+v", code, env)
	}
	var cat struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &cat); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// update ok
	code, env = doJSON(t, h, "PUT", fmt.Sprintf("/api/v1/nav/categories/%d", cat.ID), `{"name":"新名","sort_order":2}`)
	if code != 200 || !env.Success {
		t.Fatalf("update: code=%d env=%+v", code, env)
	}
	var updated struct {
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
	}
	if err := json.Unmarshal(env.Data, &updated); err != nil {
		t.Fatalf("unmarshal updated: %v", err)
	}
	if updated.Name != "新名" || updated.SortOrder != 2 {
		t.Errorf("updated = %+v", updated)
	}

	// update missing -> 404
	if code, _ = doJSON(t, h, "PUT", "/api/v1/nav/categories/99999", `{"name":"x"}`); code != 404 {
		t.Errorf("update missing category: code=%d", code)
	}
	// update bad body -> 400
	if code, _ = doJSON(t, h, "PUT", fmt.Sprintf("/api/v1/nav/categories/%d", cat.ID), `{"name":""}`); code != 400 {
		t.Errorf("update empty name: code=%d", code)
	}
}
