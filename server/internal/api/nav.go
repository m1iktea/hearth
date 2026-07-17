// server/internal/api/nav.go
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/m1iktea/hearth/server/internal/store"
)

func registerNavRoutes(mux *http.ServeMux, nav *store.NavStore, inventory *store.InventoryStore) {
	h := &navHandler{nav: nav, inventory: inventory}
	mux.HandleFunc("GET /api/v1/nav", h.list)
	mux.HandleFunc("POST /api/v1/nav/categories", h.createCategory)
	mux.HandleFunc("PUT /api/v1/nav/categories/{id}", h.updateCategory)
	mux.HandleFunc("DELETE /api/v1/nav/categories/{id}", h.deleteCategory)
	mux.HandleFunc("POST /api/v1/nav/items", h.createItem)
	mux.HandleFunc("PUT /api/v1/nav/items/{id}", h.updateItem)
	mux.HandleFunc("DELETE /api/v1/nav/items/{id}", h.deleteItem)
}

type navHandler struct {
	nav       *store.NavStore
	inventory *store.InventoryStore
}

type categoryInput struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type itemInput struct {
	CategoryID int64  `json:"category_id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Icon       string `json:"icon"`
	SortOrder  int    `json:"sort_order"`
	DeviceID   *int64 `json:"device_id"`
}

func (h *navHandler) list(w http.ResponseWriter, r *http.Request) {
	cats, err := h.nav.ListCategories()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list nav")
		return
	}
	writeOK(w, cats)
}

func (h *navHandler) createCategory(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeCategory(w, r)
	if !ok {
		return
	}
	cat, err := h.nav.CreateCategory(in.Name, in.SortOrder)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create category")
		return
	}
	writeOK(w, cat)
}

func (h *navHandler) updateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	in, ok := decodeCategory(w, r)
	if !ok {
		return
	}
	cat, err := h.nav.UpdateCategory(id, in.Name, in.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "category not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update category")
		return
	}
	writeOK(w, cat)
}

func (h *navHandler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.nav.DeleteCategory(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete category")
		return
	}
	writeOK(w, nil)
}

func (h *navHandler) createItem(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeItem(w, r)
	if !ok {
		return
	}
	if in.DeviceID != nil {
		if err := h.checkDeviceExists(w, *in.DeviceID); err != nil {
			return
		}
		if err := h.checkDeviceNotLinked(w, *in.DeviceID, 0); err != nil {
			return
		}
	}
	item, err := h.nav.CreateItem(store.Item{
		CategoryID: in.CategoryID,
		Name:       in.Name,
		URL:        in.URL,
		Icon:       in.Icon,
		SortOrder:  in.SortOrder,
		DeviceID:   in.DeviceID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create item")
		return
	}
	writeOK(w, item)
}

func (h *navHandler) updateItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	in, ok := decodeItem(w, r)
	if !ok {
		return
	}
	if in.DeviceID != nil {
		if err := h.checkDeviceExists(w, *in.DeviceID); err != nil {
			return
		}
		if err := h.checkDeviceNotLinked(w, *in.DeviceID, id); err != nil {
			return
		}
	}
	item, err := h.nav.UpdateItem(store.Item{
		ID:         id,
		CategoryID: in.CategoryID,
		Name:       in.Name,
		URL:        in.URL,
		Icon:       in.Icon,
		SortOrder:  in.SortOrder,
		DeviceID:   in.DeviceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update item")
		return
	}
	writeOK(w, item)
}

func (h *navHandler) deleteItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.nav.DeleteItem(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete item")
		return
	}
	writeOK(w, nil)
}

// checkDeviceExists 校验设备在台账中存在，失败时写入 422 响应并返回 error。
func (h *navHandler) checkDeviceExists(w http.ResponseWriter, deviceID int64) error {
	_, err := h.inventory.GetDevice(deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, 422, "device not found")
		return err
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check device")
		return err
	}
	return nil
}

// checkDeviceNotLinked 校验同一 device_id 未被其他导航项占用。
// skipItemID 为 0 表示创建场景；更新场景传入当前条目 id 排除自身。
func (h *navHandler) checkDeviceNotLinked(w http.ResponseWriter, deviceID int64, skipItemID int64) error {
	existing, err := h.nav.GetItemByDeviceID(deviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // 未被占用
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check device link")
		return err
	}
	if existing.ID == skipItemID {
		return nil // 更新自身，允许
	}
	writeError(w, http.StatusConflict, "device is already linked to another nav item")
	return fmt.Errorf("conflict")
}

// --- input helpers（边界校验：JSON 合法性 + 必填字段） ---

func decodeCategory(w http.ResponseWriter, r *http.Request) (categoryInput, bool) {
	var in categoryInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return in, false
	}
	if strings.TrimSpace(in.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return in, false
	}
	return in, true
}

func decodeItem(w http.ResponseWriter, r *http.Request) (itemInput, bool) {
	var in itemInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return in, false
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.URL) == "" {
		writeError(w, http.StatusBadRequest, "name and url are required")
		return in, false
	}
	if in.CategoryID <= 0 {
		writeError(w, http.StatusBadRequest, "category_id is required")
		return in, false
	}
	return in, true
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}
