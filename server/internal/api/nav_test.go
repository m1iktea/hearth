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
