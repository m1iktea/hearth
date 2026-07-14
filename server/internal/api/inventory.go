package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/m1iktea/hearth/server/internal/discovery"
	"github.com/m1iktea/hearth/server/internal/store"
)

type arpScanner interface {
	Scan(context.Context) ([]discovery.Device, error)
}
type inventoryHandler struct {
	store      *store.InventoryStore
	arpScanner arpScanner
}

func registerInventoryRoutes(mux *http.ServeMux, s *store.InventoryStore, scanner arpScanner) {
	h := &inventoryHandler{store: s, arpScanner: scanner}
	mux.HandleFunc("GET /api/v1/devices", h.listDevices)
	mux.HandleFunc("POST /api/v1/devices", h.createDevice)
	mux.HandleFunc("GET /api/v1/devices/{id}", h.getDevice)
	mux.HandleFunc("PUT /api/v1/devices/{id}", h.updateDevice)
	mux.HandleFunc("DELETE /api/v1/devices/{id}", h.deleteDevice)
	mux.HandleFunc("POST /api/v1/devices/{id}/checks", h.createCheck)
	mux.HandleFunc("PUT /api/v1/devices/{id}/checks/{checkID}", h.updateCheck)
	mux.HandleFunc("DELETE /api/v1/devices/{id}/checks/{checkID}", h.deleteCheck)
	mux.HandleFunc("GET /api/v1/events", h.listEvents)
	mux.HandleFunc("GET /api/v1/health", h.listHealth)
	mux.HandleFunc("POST /api/v1/discovery/arp", h.scanARP)
}

type deviceInput struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Hostname   string `json:"hostname"`
	IPAddress  string `json:"ip_address"`
	MACAddress string `json:"mac_address"`
	Location   string `json:"location"`
	Notes      string `json:"notes"`
	URL        string `json:"url"`
	Enabled    *bool  `json:"enabled"`
}

func (h *inventoryHandler) listDevices(w http.ResponseWriter, r *http.Request) {
	v, e := h.store.ListDevices()
	if e != nil {
		writeError(w, 500, "failed to list devices")
		return
	}
	writeOK(w, v)
}
func (h *inventoryHandler) getDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	v, e := h.store.GetDeviceDetail(id)
	if errors.Is(e, sql.ErrNoRows) {
		writeError(w, 404, "device not found")
		return
	}
	if e != nil {
		writeError(w, 500, "failed to get device")
		return
	}
	writeOK(w, v)
}
func (h *inventoryHandler) createDevice(w http.ResponseWriter, r *http.Request) {
	d, ok := decodeDevice(w, r)
	if !ok {
		return
	}
	v, e := h.store.CreateDevice(d)
	if e != nil {
		writeError(w, 500, "failed to create device")
		return
	}
	writeOK(w, v)
}
func (h *inventoryHandler) updateDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	d, ok := decodeDevice(w, r)
	if !ok {
		return
	}
	d.ID = id
	v, e := h.store.UpdateDevice(d)
	if errors.Is(e, sql.ErrNoRows) {
		writeError(w, 404, "device not found")
		return
	}
	if e != nil {
		writeError(w, 500, "failed to update device")
		return
	}
	writeOK(w, v)
}
func (h *inventoryHandler) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if e := h.store.DeleteDevice(id); e != nil {
		writeError(w, 500, "failed to delete device")
		return
	}
	writeOK(w, nil)
}
func decodeDevice(w http.ResponseWriter, r *http.Request) (store.Device, bool) {
	var in deviceInput
	if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
		writeError(w, 400, "invalid json body")
		return store.Device{}, false
	}
	if strings.TrimSpace(in.Name) == "" {
		writeError(w, 400, "name is required")
		return store.Device{}, false
	}
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = "other"
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	return store.Device{Name: strings.TrimSpace(in.Name), Kind: kind, Hostname: strings.TrimSpace(in.Hostname), IPAddress: strings.TrimSpace(in.IPAddress), MACAddress: strings.TrimSpace(in.MACAddress), Location: strings.TrimSpace(in.Location), Notes: strings.TrimSpace(in.Notes), URL: strings.TrimSpace(in.URL), Enabled: enabled}, true
}

type checkInput struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Target         string `json:"target"`
	Port           int    `json:"port"`
	ExpectedStatus int    `json:"expected_status"`
	Enabled        *bool  `json:"enabled"`
}

func (h *inventoryHandler) createCheck(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := pathID(w, r)
	if !ok {
		return
	}
	c, ok := decodeCheck(w, r)
	if !ok {
		return
	}
	c.DeviceID = deviceID
	v, e := h.store.CreateCheck(c)
	if e != nil {
		writeError(w, 500, "failed to create health check")
		return
	}
	writeOK(w, v)
}
func checkID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, e := strconv.ParseInt(r.PathValue("checkID"), 10, 64)
	if e != nil || id < 1 {
		writeError(w, 400, "invalid check id")
		return 0, false
	}
	return id, true
}
func (h *inventoryHandler) updateCheck(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := pathID(w, r)
	if !ok {
		return
	}
	id, ok := checkID(w, r)
	if !ok {
		return
	}
	c, ok := decodeCheck(w, r)
	if !ok {
		return
	}
	c.ID = id
	c.DeviceID = deviceID
	v, e := h.store.UpdateCheck(c)
	if errors.Is(e, sql.ErrNoRows) {
		writeError(w, 404, "health check not found")
		return
	}
	if e != nil {
		writeError(w, 500, "failed to update health check")
		return
	}
	writeOK(w, v)
}
func (h *inventoryHandler) deleteCheck(w http.ResponseWriter, r *http.Request) {
	_, ok := pathID(w, r)
	if !ok {
		return
	}
	id, ok := checkID(w, r)
	if !ok {
		return
	}
	if e := h.store.DeleteCheck(id); e != nil {
		writeError(w, 500, "failed to delete health check")
		return
	}
	writeOK(w, nil)
}
func decodeCheck(w http.ResponseWriter, r *http.Request) (store.HealthCheck, bool) {
	var in checkInput
	if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
		writeError(w, 400, "invalid json body")
		return store.HealthCheck{}, false
	}
	typ := strings.ToLower(strings.TrimSpace(in.Type))
	if typ != "ping" && typ != "tcp" && typ != "http" {
		writeError(w, 400, "type must be ping, tcp, or http")
		return store.HealthCheck{}, false
	}
	if typ == "tcp" && (in.Port < 1 || in.Port > 65535) {
		writeError(w, 400, "tcp port must be 1-65535")
		return store.HealthCheck{}, false
	}
	if typ == "http" && strings.TrimSpace(in.Target) == "" {
		writeError(w, 400, "http target is required")
		return store.HealthCheck{}, false
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = strings.ToUpper(typ) + " 检查"
	}
	return store.HealthCheck{Name: name, Type: typ, Target: strings.TrimSpace(in.Target), Port: in.Port, ExpectedStatus: in.ExpectedStatus, Enabled: enabled}, true
}
func (h *inventoryHandler) listEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	v, e := h.store.ListEvents(limit)
	if e != nil {
		writeError(w, 500, "failed to list events")
		return
	}
	writeOK(w, v)
}
func (h *inventoryHandler) listHealth(w http.ResponseWriter, r *http.Request) {
	v, e := h.store.ListEnabledChecks()
	if e != nil {
		writeError(w, 500, "failed to list health checks")
		return
	}
	writeOK(w, v)
}
func (h *inventoryHandler) scanARP(w http.ResponseWriter, r *http.Request) {
	if h.arpScanner == nil {
		writeError(w, http.StatusServiceUnavailable, "当前 Docker 部署未启用 ARP 扫描；请使用主机网络模式的部署配置")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	found, err := h.arpScanner.Scan(ctx)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			writeError(w, http.StatusServiceUnavailable, "当前运行环境未安装 ARP 扫描工具；macOS 本地预览仅展示界面，请在飞牛 Docker 部署后执行扫描")
			return
		}
		writeError(w, http.StatusInternalServerError, "arp scan failed; verify host network, NET_RAW capability and arp-scan installation")
		return
	}
	type result struct {
		Device store.Device `json:"device"`
		IsNew  bool         `json:"is_new"`
		Vendor string       `json:"vendor"`
	}
	items := make([]result, 0, len(found))
	newCount := 0
	for _, item := range found {
		d, isNew, err := h.store.UpsertDiscoveredDevice(store.DiscoveredDevice{IPAddress: item.IPAddress, MACAddress: item.MACAddress, Vendor: item.Vendor})
		if err != nil {
			writeError(w, 500, "failed to save discovered device")
			return
		}
		if isNew {
			newCount++
		}
		items = append(items, result{Device: d, IsNew: isNew, Vendor: item.Vendor})
	}
	writeOK(w, map[string]any{"devices": items, "new_count": newCount, "updated_count": len(items) - newCount})
}
