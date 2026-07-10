# Hearth MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建 Hearth MVP——导航页 + 节点状态监控（Proxmox/Docker/ImmortalWrt）+ 统一仪表盘，单二进制 Go 后端内嵌 Vue 3 前端，Docker 部署到飞牛 NAS。

**Architecture:** 后端每 10s 后台轮询三个数据源写入内存快照，REST API 读快照与 SQLite 导航数据；前端轮询 API 渲染。前端构建产物 go:embed 进二进制。

**Tech Stack:** Go 1.23（stdlib net/http 路由 + modernc.org/sqlite）、Vue 3 + Vite + TS + Naive UI + Pinia、Docker 多阶段构建。

**Spec:** `docs/superpowers/specs/2026-07-10-hearth-mvp-design.md`

**约定：**
- Go module：`github.com/m1iktea/hearth/server`
- 所有 Go 命令在 `server/` 目录执行；npm 命令在 `web/` 目录执行
- 统一 API 响应包裹：`{"success":bool,"data":...,"error":...}`
- commit 不带 attribution（全局已禁用）

---

## File Structure

```
hearth/
├── server/
│   ├── go.mod / go.sum
│   ├── cmd/hearth/main.go              # 组装：config→collectors→scheduler→http
│   └── internal/
│       ├── config/config.go            # 环境变量加载与分组校验
│       ├── collector/collector.go      # Collector 接口 + Snapshot 类型
│       ├── collector/scheduler.go      # 定时并发采集，写 SnapshotStore
│       ├── collector/proxmox/proxmox.go
│       ├── collector/docker/docker.go
│       ├── collector/openwrt/openwrt.go
│       ├── store/snapshot.go           # 内存快照（RWMutex）
│       ├── store/nav.go                # SQLite 导航 CRUD
│       ├── api/respond.go              # 响应包裹 helpers
│       ├── api/router.go               # 路由 + middleware（logger + auth 插槽）
│       ├── api/status.go / api/nav.go  # handlers
│       ├── api/spa.go                  # SPA 静态托管（index.html fallback）
│       └── webdist/webdist.go          # go:embed dist
├── web/                                # Vue 3 + Vite + TS
│   └── src/{api,stores,utils,views,router}/
├── deploy/Dockerfile / docker-compose.yml / .env.example
├── README.md
└── docs/deploy.md
```

---

### Task 1: 后端骨架与 go.mod

**Files:**
- Create: `.gitignore`, `server/go.mod`

- [x] **Step 1: 创建 .gitignore**

```gitignore
# repo root .gitignore
.env
deploy/.env
deploy/data/
web/node_modules/
web/dist/
server/internal/webdist/dist/*
!server/internal/webdist/dist/index.html
*.db
.DS_Store
```

- [x] **Step 2: 初始化 go module**

Run: `cd server && go mod init github.com/m1iktea/hearth/server`
Expected: 生成 `server/go.mod`，`module github.com/m1iktea/hearth/server`，`go 1.23`（低于 1.22 则手动改为本机版本，需 ≥1.22 以使用方法路由）

- [x] **Step 3: Commit**

```bash
git add .gitignore server/go.mod
git commit -m "chore: init go module and gitignore"
```

---

### Task 2: config 包（环境变量加载与校验）

**Files:**
- Create: `server/internal/config/config.go`
- Test: `server/internal/config/config_test.go`

**规则：** 数据源按组启用——`PVE_URL` 非空则 `PVE_TOKEN_ID`/`PVE_TOKEN_SECRET` 必填；`OPENWRT_URL` 非空则用户名密码必填；`DOCKER_HOST` 有默认值恒启用。三源全空则报错。

- [x] **Step 1: 写失败测试**

```go
// server/internal/config/config_test.go
package config

import (
	"testing"
	"time"
)

func getenvFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(getenvFrom(map[string]string{
		"PVE_URL": "https://pve:8006", "PVE_TOKEN_ID": "root@pam!hearth", "PVE_TOKEN_SECRET": "s",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DockerHost != "unix:///var/run/docker.sock" {
		t.Errorf("DockerHost = %q", cfg.DockerHost)
	}
	if cfg.PollInterval != 10*time.Second {
		t.Errorf("PollInterval = %v", cfg.PollInterval)
	}
	if cfg.DataDir != "/data" || cfg.ListenAddr != ":8080" {
		t.Errorf("DataDir=%q ListenAddr=%q", cfg.DataDir, cfg.ListenAddr)
	}
}

func TestLoadPVEGroupIncomplete(t *testing.T) {
	_, err := Load(getenvFrom(map[string]string{"PVE_URL": "https://pve:8006"}))
	if err == nil {
		t.Fatal("want error when PVE_URL set without token")
	}
}

func TestLoadOpenWrtGroupIncomplete(t *testing.T) {
	_, err := Load(getenvFrom(map[string]string{
		"OPENWRT_URL": "http://router", "OPENWRT_USERNAME": "root",
	}))
	if err == nil {
		t.Fatal("want error when OPENWRT password missing")
	}
}

func TestLoadInvalidInterval(t *testing.T) {
	_, err := Load(getenvFrom(map[string]string{
		"PVE_URL": "u", "PVE_TOKEN_ID": "i", "PVE_TOKEN_SECRET": "s",
		"HEARTH_POLL_INTERVAL": "abc",
	}))
	if err == nil {
		t.Fatal("want error for invalid interval")
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd server && go test ./internal/config/`
Expected: FAIL（`Load` 未定义，编译错误）

- [x] **Step 3: 最小实现**

```go
// server/internal/config/config.go
package config

import (
	"errors"
	"fmt"
	"time"
)

// Config 全部来自环境变量，凭据不入库不进代码。
type Config struct {
	PVEURL         string
	PVETokenID     string
	PVETokenSecret string

	DockerHost string

	OpenWrtURL      string
	OpenWrtUsername string
	OpenWrtPassword string

	PollInterval time.Duration
	DataDir      string
	ListenAddr   string
}

// PVEEnabled / OpenWrtEnabled 由 URL 是否配置决定；Docker 恒启用（有默认 socket）。
func (c *Config) PVEEnabled() bool     { return c.PVEURL != "" }
func (c *Config) OpenWrtEnabled() bool { return c.OpenWrtURL != "" }

func Load(getenv func(string) string) (*Config, error) {
	cfg := &Config{
		PVEURL:          getenv("PVE_URL"),
		PVETokenID:      getenv("PVE_TOKEN_ID"),
		PVETokenSecret:  getenv("PVE_TOKEN_SECRET"),
		DockerHost:      getenv("DOCKER_HOST"),
		OpenWrtURL:      getenv("OPENWRT_URL"),
		OpenWrtUsername: getenv("OPENWRT_USERNAME"),
		OpenWrtPassword: getenv("OPENWRT_PASSWORD"),
		DataDir:         getenv("HEARTH_DATA_DIR"),
		ListenAddr:      getenv("HEARTH_LISTEN"),
	}
	if cfg.DockerHost == "" {
		cfg.DockerHost = "unix:///var/run/docker.sock"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "/data"
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}

	interval := getenv("HEARTH_POLL_INTERVAL")
	if interval == "" {
		cfg.PollInterval = 10 * time.Second
	} else {
		d, err := time.ParseDuration(interval)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("invalid HEARTH_POLL_INTERVAL %q: %w", interval, err)
		}
		cfg.PollInterval = d
	}

	if cfg.PVEEnabled() && (cfg.PVETokenID == "" || cfg.PVETokenSecret == "") {
		return nil, errors.New("PVE_URL is set but PVE_TOKEN_ID/PVE_TOKEN_SECRET missing")
	}
	if cfg.OpenWrtEnabled() && (cfg.OpenWrtUsername == "" || cfg.OpenWrtPassword == "") {
		return nil, errors.New("OPENWRT_URL is set but OPENWRT_USERNAME/OPENWRT_PASSWORD missing")
	}
	return cfg, nil
}
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd server && go test ./internal/config/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add server/internal/config/
git commit -m "feat: add config loading with grouped validation"
```

---

### Task 3: collector 类型 + SnapshotStore

**Files:**
- Create: `server/internal/collector/collector.go`, `server/internal/store/snapshot.go`
- Test: `server/internal/store/snapshot_test.go`

- [x] **Step 1: 定义 collector 类型（无逻辑，无需单测）**

```go
// server/internal/collector/collector.go
package collector

import (
	"context"
	"time"
)

const (
	StatusOnline  = "online"
	StatusOffline = "offline"
)

// Snapshot 是某个数据源某一时刻的状态快照。
type Snapshot struct {
	Source      string    `json:"source"`
	Status      string    `json:"status"` // online | offline
	CollectedAt time.Time `json:"collected_at"`
	LastError   string    `json:"last_error,omitempty"`
	Data        any       `json:"data,omitempty"`
}

// Collector 采集一个数据源。Collect 返回源特定的 Data 负载。
type Collector interface {
	Name() string
	Collect(ctx context.Context) (any, error)
}
```

- [x] **Step 2: 写 SnapshotStore 失败测试**

```go
// server/internal/store/snapshot_test.go
package store

import (
	"errors"
	"testing"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
)

func TestSetOKAndGet(t *testing.T) {
	s := NewSnapshotStore()
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	s.SetOK("proxmox", map[string]int{"nodes": 1}, at)

	snap, ok := s.Get("proxmox")
	if !ok {
		t.Fatal("snapshot not found")
	}
	if snap.Status != collector.StatusOnline || !snap.CollectedAt.Equal(at) {
		t.Errorf("got %+v", snap)
	}
}

func TestSetErrorKeepsPreviousData(t *testing.T) {
	s := NewSnapshotStore()
	at := time.Now()
	s.SetOK("docker", "old-data", at)
	s.SetError("docker", errors.New("boom"), at.Add(time.Second))

	snap, _ := s.Get("docker")
	if snap.Status != collector.StatusOffline {
		t.Errorf("Status = %q", snap.Status)
	}
	if snap.LastError != "boom" {
		t.Errorf("LastError = %q", snap.LastError)
	}
	if snap.Data != "old-data" { // 保留上次成功数据
		t.Errorf("Data = %v", snap.Data)
	}
}

func TestGetMissing(t *testing.T) {
	s := NewSnapshotStore()
	if _, ok := s.Get("nope"); ok {
		t.Fatal("want ok=false")
	}
}

func TestAllSortedBySource(t *testing.T) {
	s := NewSnapshotStore()
	now := time.Now()
	s.SetOK("proxmox", nil, now)
	s.SetOK("docker", nil, now)
	all := s.All()
	if len(all) != 2 || all[0].Source != "docker" || all[1].Source != "proxmox" {
		t.Errorf("got %+v", all)
	}
}
```

- [x] **Step 3: 运行测试确认失败**

Run: `cd server && go test ./internal/store/`
Expected: FAIL（`NewSnapshotStore` 未定义）

- [x] **Step 4: 最小实现**

```go
// server/internal/store/snapshot.go
package store

import (
	"sort"
	"sync"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
)

// SnapshotStore 保存各数据源最新快照，并发安全。
type SnapshotStore struct {
	mu    sync.RWMutex
	snaps map[string]collector.Snapshot
}

func NewSnapshotStore() *SnapshotStore {
	return &SnapshotStore{snaps: make(map[string]collector.Snapshot)}
}

func (s *SnapshotStore) SetOK(source string, data any, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snaps[source] = collector.Snapshot{
		Source: source, Status: collector.StatusOnline, CollectedAt: at, Data: data,
	}
}

// SetError 标记离线但保留上次成功的 Data，便于页面展示最后已知状态。
func (s *SnapshotStore) SetError(source string, err error, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.snaps[source]
	s.snaps[source] = collector.Snapshot{
		Source: source, Status: collector.StatusOffline, CollectedAt: at,
		LastError: err.Error(), Data: prev.Data,
	}
}

func (s *SnapshotStore) Get(source string) (collector.Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, ok := s.snaps[source]
	return snap, ok
}

func (s *SnapshotStore) All() []collector.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]collector.Snapshot, 0, len(s.snaps))
	for _, v := range s.snaps {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}
```

- [x] **Step 5: 运行测试确认通过**

Run: `cd server && go test ./internal/store/`
Expected: PASS

- [x] **Step 6: Commit**

```bash
git add server/internal/collector/collector.go server/internal/store/snapshot.go server/internal/store/snapshot_test.go
git commit -m "feat: add collector types and in-memory snapshot store"
```

---

### Task 4: Proxmox collector

**Files:**
- Create: `server/internal/collector/proxmox/proxmox.go`
- Test: `server/internal/collector/proxmox/proxmox_test.go`

**上游契约（PVE API）：**
- 认证头：`Authorization: PVEAPIToken=<TOKEN_ID>=<SECRET>`
- `GET /api2/json/nodes` → `{"data":[{"node":"pve","status":"online","cpu":0.02,"mem":8589934592,"maxmem":33554432000,"uptime":86400}]}`
- `GET /api2/json/nodes/{node}/qemu` → `{"data":[{"vmid":100,"name":"fnos","status":"running","cpu":0.05,"mem":4294967296,"maxmem":8589934592,"uptime":3600}]}`
- 家用 PVE 常为自签名证书：client 使用 `InsecureSkipVerify`

- [x] **Step 1: 写失败测试**

```go
// server/internal/collector/proxmox/proxmox_test.go
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
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd server && go test ./internal/collector/proxmox/`
Expected: FAIL（`New` 未定义）

- [x] **Step 3: 最小实现**

```go
// server/internal/collector/proxmox/proxmox.go
package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client 通过 API Token 读取 PVE 节点与 VM 状态。
type Client struct {
	baseURL string
	token   string // "PVEAPIToken=<id>=<secret>"
	http    *http.Client
}

type Data struct {
	Nodes []Node `json:"nodes"`
}

type Node struct {
	Name   string  `json:"name"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	Mem    uint64  `json:"mem"`
	MaxMem uint64  `json:"maxmem"`
	Uptime int64   `json:"uptime"`
	VMs    []VM    `json:"vms"`
}

type VM struct {
	VMID   int     `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	Mem    uint64  `json:"mem"`
	MaxMem uint64  `json:"maxmem"`
	Uptime int64   `json:"uptime"`
}

func New(baseURL, tokenID, tokenSecret string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   fmt.Sprintf("PVEAPIToken=%s=%s", tokenID, tokenSecret),
		http: &http.Client{
			Timeout: 8 * time.Second,
			Transport: &http.Transport{
				// 家用 PVE 默认自签名证书
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (c *Client) Name() string { return "proxmox" }

func (c *Client) Collect(ctx context.Context) (any, error) {
	var nodeList []struct {
		Node   string  `json:"node"`
		Status string  `json:"status"`
		CPU    float64 `json:"cpu"`
		Mem    uint64  `json:"mem"`
		MaxMem uint64  `json:"maxmem"`
		Uptime int64   `json:"uptime"`
	}
	if err := c.getJSON(ctx, "/api2/json/nodes", &nodeList); err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	data := Data{Nodes: make([]Node, 0, len(nodeList))}
	for _, n := range nodeList {
		node := Node{
			Name: n.Node, Status: n.Status, CPU: n.CPU,
			Mem: n.Mem, MaxMem: n.MaxMem, Uptime: n.Uptime,
		}
		var vms []VM
		if err := c.getJSON(ctx, "/api2/json/nodes/"+n.Node+"/qemu", &vms); err != nil {
			return nil, fmt.Errorf("list qemu on %s: %w", n.Node, err)
		}
		node.VMs = vms
		data.Nodes = append(data.Nodes, node)
	}
	return data, nil
}

// getJSON 解开 PVE 的 {"data": ...} 包裹。
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pve returned %s", resp.Status)
	}
	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return json.Unmarshal(wrapper.Data, out)
}
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd server && go test ./internal/collector/proxmox/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add server/internal/collector/proxmox/
git commit -m "feat: add proxmox collector"
```

---

### Task 5: Docker collector

**Files:**
- Create: `server/internal/collector/docker/docker.go`
- Test: `server/internal/collector/docker/docker_test.go`

**上游契约（Docker Engine API）：**
- `GET /containers/json?all=1` → `[{"Id":"abc...","Names":["/hearth"],"Image":"hearth:latest","State":"running","Status":"Up 2 hours"}]`
- host 支持 `unix:///path`（自定义 DialContext）与 `tcp://host:port` / `http(s)://`

- [x] **Step 1: 写失败测试**

```go
// server/internal/collector/docker/docker_test.go
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
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd server && go test ./internal/collector/docker/`
Expected: FAIL（`New` 未定义）

- [x] **Step 3: 最小实现**

```go
// server/internal/collector/docker/docker.go
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Client 通过 Docker Engine API 读取容器列表。
type Client struct {
	baseURL string
	http    *http.Client
}

type Data struct {
	Containers []Container `json:"containers"`
}

type Container struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`  // running | exited | ...
	Status string `json:"status"` // "Up 2 hours"
}

func New(host string) (*Client, error) {
	c := &Client{http: &http.Client{Timeout: 8 * time.Second}}
	switch {
	case strings.HasPrefix(host, "unix://"):
		socketPath := strings.TrimPrefix(host, "unix://")
		c.baseURL = "http://docker"
		c.http.Transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		}
	case strings.HasPrefix(host, "tcp://"):
		c.baseURL = "http://" + strings.TrimPrefix(host, "tcp://")
	case strings.HasPrefix(host, "http://"), strings.HasPrefix(host, "https://"):
		c.baseURL = host
	default:
		return nil, fmt.Errorf("unsupported DOCKER_HOST %q", host)
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")
	return c, nil
}

func (c *Client) Name() string { return "docker" }

func (c *Client) Collect(ctx context.Context) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/containers/json?all=1", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker api returned %s", resp.Status)
	}

	var raw []struct {
		ID     string   `json:"Id"`
		Names  []string `json:"Names"`
		Image  string   `json:"Image"`
		State  string   `json:"State"`
		Status string   `json:"Status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode containers: %w", err)
	}

	data := Data{Containers: make([]Container, 0, len(raw))}
	for _, r := range raw {
		name := ""
		if len(r.Names) > 0 {
			name = strings.TrimPrefix(r.Names[0], "/")
		}
		data.Containers = append(data.Containers, Container{
			ID: r.ID, Name: name, Image: r.Image, State: r.State, Status: r.Status,
		})
	}
	return data, nil
}
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd server && go test ./internal/collector/docker/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add server/internal/collector/docker/
git commit -m "feat: add docker collector"
```

---

### Task 6: OpenWrt (ImmortalWrt) collector

**Files:**
- Create: `server/internal/collector/openwrt/openwrt.go`
- Test: `server/internal/collector/openwrt/openwrt_test.go`

**上游契约（LuCI ubus JSON-RPC，`POST {url}/ubus`）：**
- 请求：`{"jsonrpc":"2.0","id":N,"method":"call","params":[<session>,<object>,<method>,<args>]}`
- 登录：session 为 32 个 `0`，object=`session`，method=`login`，args=`{"username":u,"password":p,"timeout":300}` → `{"result":[0,{"ubus_rpc_session":"<sid>",...}]}`
- `system board` → `{"result":[0,{"hostname":"ImmortalWrt","model":"x86_64","release":{"distribution":"ImmortalWrt","version":"23.05"}}]}`
- `system info` → `{"result":[0,{"uptime":86400,"load":[65536,32768,16384],"memory":{"total":1000,"free":500,"available":600}}]}`（load 需除以 65536）
- result[0] 非 0 表示 ubus 错误（6 = 权限拒绝）

- [x] **Step 1: 写失败测试**

```go
// server/internal/collector/openwrt/openwrt_test.go
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
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd server && go test ./internal/collector/openwrt/`
Expected: FAIL（`New` 未定义）

- [x] **Step 3: 最小实现**

```go
// server/internal/collector/openwrt/openwrt.go
package openwrt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const nullSession = "00000000000000000000000000000000"

// Client 通过 LuCI ubus JSON-RPC 读取路由器状态，路由器侧零改动。
type Client struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

type Data struct {
	Hostname  string     `json:"hostname"`
	Model     string     `json:"model"`
	Release   string     `json:"release"`
	UptimeSec int64      `json:"uptime_sec"`
	Load      [3]float64 `json:"load"`
	Memory    Memory     `json:"memory"`
}

type Memory struct {
	Total     uint64 `json:"total"`
	Free      uint64 `json:"free"`
	Available uint64 `json:"available"`
}

func New(baseURL, username, password string) *Client {
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		http:     &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *Client) Name() string { return "openwrt" }

func (c *Client) Collect(ctx context.Context) (any, error) {
	var login struct {
		Session string `json:"ubus_rpc_session"`
	}
	err := c.call(ctx, nullSession, "session", "login", map[string]any{
		"username": c.username, "password": c.password, "timeout": 300,
	}, &login)
	if err != nil {
		return nil, fmt.Errorf("ubus login: %w", err)
	}

	var board struct {
		Hostname string `json:"hostname"`
		Model    string `json:"model"`
		Release  struct {
			Distribution string `json:"distribution"`
			Version      string `json:"version"`
		} `json:"release"`
	}
	if err := c.call(ctx, login.Session, "system", "board", map[string]any{}, &board); err != nil {
		return nil, fmt.Errorf("system board: %w", err)
	}

	var info struct {
		Uptime int64    `json:"uptime"`
		Load   [3]int64 `json:"load"`
		Memory Memory   `json:"memory"`
	}
	if err := c.call(ctx, login.Session, "system", "info", map[string]any{}, &info); err != nil {
		return nil, fmt.Errorf("system info: %w", err)
	}

	data := Data{
		Hostname:  board.Hostname,
		Model:     board.Model,
		Release:   strings.TrimSpace(board.Release.Distribution + " " + board.Release.Version),
		UptimeSec: info.Uptime,
		Memory:    info.Memory,
	}
	for i, l := range info.Load {
		data.Load[i] = float64(l) / 65536.0 // ubus load 为定点数
	}
	return data, nil
}

// call 发起一次 ubus JSON-RPC 调用并解出 result[1] 到 out。
func (c *Client) call(ctx context.Context, session, object, method string, args map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "call",
		"params": [4]any{session, object, method, args},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ubus", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ubus returned %s", resp.Status)
	}

	var rpc struct {
		Result []json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		return fmt.Errorf("decode rpc response: %w", err)
	}
	if rpc.Error != nil {
		return fmt.Errorf("rpc error %d: %s", rpc.Error.Code, rpc.Error.Message)
	}
	if len(rpc.Result) == 0 {
		return fmt.Errorf("empty ubus result")
	}
	var code int
	if err := json.Unmarshal(rpc.Result[0], &code); err != nil {
		return fmt.Errorf("decode ubus status: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("ubus status %d", code)
	}
	if out != nil && len(rpc.Result) > 1 {
		return json.Unmarshal(rpc.Result[1], out)
	}
	return nil
}
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd server && go test ./internal/collector/openwrt/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add server/internal/collector/openwrt/
git commit -m "feat: add openwrt ubus collector"
```

---

### Task 7: Scheduler（定时并发采集）

**Files:**
- Create: `server/internal/collector/scheduler.go`
- Test: `server/internal/collector/scheduler_test.go`

- [x] **Step 1: 写失败测试**

```go
// server/internal/collector/scheduler_test.go
package collector_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
	"github.com/m1iktea/hearth/server/internal/store"
)

type fake struct {
	name string
	data any
	err  error
}

func (f *fake) Name() string                            { return f.name }
func (f *fake) Collect(_ context.Context) (any, error) { return f.data, f.err }

func TestCollectOnceWritesSnapshots(t *testing.T) {
	s := store.NewSnapshotStore()
	sched := collector.NewScheduler(
		[]collector.Collector{
			&fake{name: "ok-source", data: "payload"},
			&fake{name: "bad-source", err: errors.New("down")},
		},
		s, time.Minute, slog.Default(),
	)

	sched.CollectOnce(context.Background())

	ok, _ := s.Get("ok-source")
	if ok.Status != collector.StatusOnline || ok.Data != "payload" {
		t.Errorf("ok-source: %+v", ok)
	}
	bad, _ := s.Get("bad-source")
	if bad.Status != collector.StatusOffline || bad.LastError != "down" {
		t.Errorf("bad-source: %+v", bad)
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	s := store.NewSnapshotStore()
	sched := collector.NewScheduler(
		[]collector.Collector{&fake{name: "ok-source", data: 1}},
		s, 10*time.Millisecond, slog.Default(),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() { sched.Run(ctx); close(done) }()

	select {
	case <-done: // Run 应随 ctx 取消而返回
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
	if _, ok := s.Get("ok-source"); !ok {
		t.Error("expected at least one collection")
	}
}
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd server && go test ./internal/collector/`
Expected: FAIL（`NewScheduler` 未定义）

- [x] **Step 3: 最小实现**

```go
// server/internal/collector/scheduler.go
package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// snapshotSink 是 Scheduler 对存储的最小依赖（由 store.SnapshotStore 满足）。
type snapshotSink interface {
	SetOK(source string, data any, at time.Time)
	SetError(source string, err error, at time.Time)
}

// Scheduler 定时并发触发所有 collector，将结果写入快照存储。
type Scheduler struct {
	collectors []Collector
	sink       snapshotSink
	interval   time.Duration
	logger     *slog.Logger
}

func NewScheduler(collectors []Collector, sink snapshotSink, interval time.Duration, logger *slog.Logger) *Scheduler {
	return &Scheduler{collectors: collectors, sink: sink, interval: interval, logger: logger}
}

// Run 先立即采集一次，然后按 interval 循环，直到 ctx 取消。
func (s *Scheduler) Run(ctx context.Context) {
	s.CollectOnce(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CollectOnce(ctx)
		}
	}
}

// CollectOnce 并发采集所有源；单源超时上限为 interval，失败只影响该源。
func (s *Scheduler) CollectOnce(ctx context.Context) {
	var wg sync.WaitGroup
	for _, c := range s.collectors {
		wg.Add(1)
		go func(c Collector) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, s.interval)
			defer cancel()
			data, err := c.Collect(cctx)
			now := time.Now()
			if err != nil {
				s.logger.Warn("collect failed", "source", c.Name(), "error", err)
				s.sink.SetError(c.Name(), err, now)
				return
			}
			s.sink.SetOK(c.Name(), data, now)
		}(c)
	}
	wg.Wait()
}
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd server && go test ./internal/collector/...`
Expected: PASS（含之前的 proxmox/docker/openwrt 测试）

- [x] **Step 5: Commit**

```bash
git add server/internal/collector/scheduler.go server/internal/collector/scheduler_test.go
git commit -m "feat: add polling scheduler with per-source failure isolation"
```

---

### Task 8: NavStore（SQLite 导航 CRUD）

**Files:**
- Create: `server/internal/store/nav.go`
- Test: `server/internal/store/nav_test.go`

**依赖：** `go get modernc.org/sqlite`（纯 Go，无 CGO）

- [x] **Step 1: 安装依赖**

Run: `cd server && go get modernc.org/sqlite`
Expected: go.mod 新增 `modernc.org/sqlite` 及间接依赖

- [x] **Step 2: 写失败测试**

```go
// server/internal/store/nav_test.go
package store

import (
	"path/filepath"
	"testing"
)

func newTestNav(t *testing.T) *NavStore {
	t.Helper()
	n, err := OpenNav(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenNav: %v", err)
	}
	t.Cleanup(func() { n.Close() })
	return n
}

func TestCategoryCRUD(t *testing.T) {
	n := newTestNav(t)

	cat, err := n.CreateCategory("基础设施", 1)
	if err != nil || cat.ID == 0 || cat.Name != "基础设施" {
		t.Fatalf("CreateCategory: %+v, %v", cat, err)
	}

	cat2, err := n.UpdateCategory(cat.ID, "服务", 2)
	if err != nil || cat2.Name != "服务" || cat2.SortOrder != 2 {
		t.Fatalf("UpdateCategory: %+v, %v", cat2, err)
	}

	if err := n.DeleteCategory(cat.ID); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}
	cats, _ := n.ListCategories()
	if len(cats) != 0 {
		t.Errorf("want empty, got %+v", cats)
	}
}

func TestItemCRUDAndListNesting(t *testing.T) {
	n := newTestNav(t)
	cat, _ := n.CreateCategory("服务", 1)

	item, err := n.CreateItem(Item{CategoryID: cat.ID, Name: "PVE", URL: "https://pve:8006", Icon: "server", SortOrder: 1})
	if err != nil || item.ID == 0 {
		t.Fatalf("CreateItem: %+v, %v", item, err)
	}

	item.Name = "Proxmox"
	updated, err := n.UpdateItem(item)
	if err != nil || updated.Name != "Proxmox" {
		t.Fatalf("UpdateItem: %+v, %v", updated, err)
	}

	cats, err := n.ListCategories()
	if err != nil || len(cats) != 1 || len(cats[0].Items) != 1 {
		t.Fatalf("ListCategories: %+v, %v", cats, err)
	}
	if cats[0].Items[0].Name != "Proxmox" {
		t.Errorf("item = %+v", cats[0].Items[0])
	}

	if err := n.DeleteItem(item.ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
}

func TestDeleteCategoryCascadesItems(t *testing.T) {
	n := newTestNav(t)
	cat, _ := n.CreateCategory("临时", 1)
	n.CreateItem(Item{CategoryID: cat.ID, Name: "x", URL: "http://x"})

	if err := n.DeleteCategory(cat.ID); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}
	cats, _ := n.ListCategories()
	if len(cats) != 0 {
		t.Errorf("want cascade delete, got %+v", cats)
	}
}
```

- [x] **Step 3: 运行测试确认失败**

Run: `cd server && go test ./internal/store/ -run 'Category|Item'`
Expected: FAIL（`OpenNav` 未定义）

- [x] **Step 4: 最小实现**

```go
// server/internal/store/nav.go
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// NavStore 管理导航分类与条目，存于 SQLite 单文件。
type NavStore struct {
	db *sql.DB
}

type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Items     []Item `json:"items"`
}

type Item struct {
	ID         int64  `json:"id"`
	CategoryID int64  `json:"category_id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Icon       string `json:"icon"`
	SortOrder  int    `json:"sort_order"`
}

const navSchema = `
CREATE TABLE IF NOT EXISTS nav_categories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS nav_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	category_id INTEGER NOT NULL REFERENCES nav_categories(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	url TEXT NOT NULL,
	icon TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL DEFAULT 0
);`

func OpenNav(path string) (*NavStore, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(navSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &NavStore{db: db}, nil
}

func (n *NavStore) Close() error { return n.db.Close() }

func (n *NavStore) ListCategories() ([]Category, error) {
	rows, err := n.db.Query(`SELECT id, name, sort_order FROM nav_categories ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cats := []Category{}
	index := map[int64]int{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.SortOrder); err != nil {
			return nil, err
		}
		c.Items = []Item{}
		index[c.ID] = len(cats)
		cats = append(cats, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	itemRows, err := n.db.Query(`SELECT id, category_id, name, url, icon, sort_order FROM nav_items ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer itemRows.Close()
	for itemRows.Next() {
		var it Item
		if err := itemRows.Scan(&it.ID, &it.CategoryID, &it.Name, &it.URL, &it.Icon, &it.SortOrder); err != nil {
			return nil, err
		}
		if i, ok := index[it.CategoryID]; ok {
			cats[i].Items = append(cats[i].Items, it)
		}
	}
	return cats, itemRows.Err()
}

func (n *NavStore) CreateCategory(name string, sortOrder int) (Category, error) {
	res, err := n.db.Exec(`INSERT INTO nav_categories (name, sort_order) VALUES (?, ?)`, name, sortOrder)
	if err != nil {
		return Category{}, err
	}
	id, _ := res.LastInsertId()
	return Category{ID: id, Name: name, SortOrder: sortOrder, Items: []Item{}}, nil
}

func (n *NavStore) UpdateCategory(id int64, name string, sortOrder int) (Category, error) {
	res, err := n.db.Exec(`UPDATE nav_categories SET name = ?, sort_order = ? WHERE id = ?`, name, sortOrder, id)
	if err != nil {
		return Category{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return Category{}, sql.ErrNoRows
	}
	return Category{ID: id, Name: name, SortOrder: sortOrder, Items: []Item{}}, nil
}

func (n *NavStore) DeleteCategory(id int64) error {
	_, err := n.db.Exec(`DELETE FROM nav_categories WHERE id = ?`, id)
	return err
}

func (n *NavStore) CreateItem(it Item) (Item, error) {
	res, err := n.db.Exec(
		`INSERT INTO nav_items (category_id, name, url, icon, sort_order) VALUES (?, ?, ?, ?, ?)`,
		it.CategoryID, it.Name, it.URL, it.Icon, it.SortOrder,
	)
	if err != nil {
		return Item{}, err
	}
	it.ID, _ = res.LastInsertId()
	return it, nil
}

func (n *NavStore) UpdateItem(it Item) (Item, error) {
	res, err := n.db.Exec(
		`UPDATE nav_items SET category_id = ?, name = ?, url = ?, icon = ?, sort_order = ? WHERE id = ?`,
		it.CategoryID, it.Name, it.URL, it.Icon, it.SortOrder, it.ID,
	)
	if err != nil {
		return Item{}, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return Item{}, sql.ErrNoRows
	}
	return it, nil
}

func (n *NavStore) DeleteItem(id int64) error {
	_, err := n.db.Exec(`DELETE FROM nav_items WHERE id = ?`, id)
	return err
}
```

- [x] **Step 5: 运行测试确认通过**

Run: `cd server && go test ./internal/store/`
Expected: PASS（全部 snapshot + nav 测试）

- [x] **Step 6: Commit**

```bash
git add server/go.mod server/go.sum server/internal/store/nav.go server/internal/store/nav_test.go
git commit -m "feat: add sqlite nav store with category/item CRUD"
```

---

### Task 9: API — respond helpers + status endpoints

**Files:**
- Create: `server/internal/api/respond.go`, `server/internal/api/status.go`, `server/internal/api/router.go`
- Test: `server/internal/api/status_test.go`

- [x] **Step 1: 写失败测试**

```go
// server/internal/api/status_test.go
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

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
	dist := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<html>hearth</html>")}}
	return NewRouter(snaps, nav, dist, slog.Default()), snaps, nav
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
```

注意：测试文件需要 `import "strings"`。

- [x] **Step 2: 运行测试确认失败**

Run: `cd server && go test ./internal/api/`
Expected: FAIL（`NewRouter` 未定义）

- [x] **Step 3: 实现 respond + status + router + spa**

```go
// server/internal/api/respond.go
package api

import (
	"encoding/json"
	"net/http"
)

type response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data"`
	Error   string `json:"error,omitempty"`
}

func writeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, response{Success: true, Data: data})
}

// writeError 只输出给定消息，不透传内部错误细节（防泄露凭据等）。
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, response{Success: false, Error: msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

```go
// server/internal/api/status.go
package api

import (
	"net/http"

	"github.com/m1iktea/hearth/server/internal/store"
)

type statusHandler struct {
	snaps *store.SnapshotStore
}

func (h *statusHandler) all(w http.ResponseWriter, r *http.Request) {
	writeOK(w, h.snaps.All())
}

func (h *statusHandler) bySource(w http.ResponseWriter, r *http.Request) {
	source := r.PathValue("source")
	snap, ok := h.snaps.Get(source)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown source: "+source)
		return
	}
	writeOK(w, snap)
}
```

```go
// server/internal/api/spa.go
package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// spaHandler 托管前端构建产物；未命中的非 /api 路径回落到 index.html（SPA 路由）。
func spaHandler(dist fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if _, err := fs.Stat(dist, path); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		r.URL.Path = "/" // fallback to index.html
		fileServer.ServeHTTP(w, r)
	})
}
```

```go
// server/internal/api/router.go
package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/m1iktea/hearth/server/internal/store"
)

func NewRouter(snaps *store.SnapshotStore, nav *store.NavStore, dist fs.FS, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeOK(w, "ok")
	})

	sh := &statusHandler{snaps: snaps}
	mux.HandleFunc("GET /api/v1/status", sh.all)
	mux.HandleFunc("GET /api/v1/status/{source}", sh.bySource)

	registerNavRoutes(mux, nav) // Task 10 实现；本 Task 先提供空实现避免编译失败

	mux.Handle("/", spaHandler(dist))

	return withMiddleware(mux, logger)
}

// withMiddleware: 日志 + auth 插槽（MVP 为直通，后续在此接入认证）。
func withMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return requestLogger(authStub(next), logger)
}

func authStub(next http.Handler) http.Handler { return next }

func requestLogger(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Debug("http", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}
```

同时创建占位的 nav 路由文件（Task 10 填充 handler）：

```go
// server/internal/api/nav.go
package api

import (
	"net/http"

	"github.com/m1iktea/hearth/server/internal/store"
)

func registerNavRoutes(mux *http.ServeMux, nav *store.NavStore) {
	_ = nav // Task 10 实现
	_ = mux
}
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd server && go test ./internal/api/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add server/internal/api/
git commit -m "feat: add http router, status endpoints and spa hosting"
```

---

### Task 10: API — nav CRUD

**Files:**
- Modify: `server/internal/api/nav.go`（替换占位实现）
- Test: `server/internal/api/nav_test.go`

- [x] **Step 1: 写失败测试**

```go
// server/internal/api/nav_test.go
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
```

- [x] **Step 2: 运行测试确认失败**

Run: `cd server && go test ./internal/api/ -run Nav`
Expected: FAIL（404，路由未注册）

- [x] **Step 3: 实现 nav handlers（替换 nav.go 全文）**

```go
// server/internal/api/nav.go
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/m1iktea/hearth/server/internal/store"
)

func registerNavRoutes(mux *http.ServeMux, nav *store.NavStore) {
	h := &navHandler{nav: nav}
	mux.HandleFunc("GET /api/v1/nav", h.list)
	mux.HandleFunc("POST /api/v1/nav/categories", h.createCategory)
	mux.HandleFunc("PUT /api/v1/nav/categories/{id}", h.updateCategory)
	mux.HandleFunc("DELETE /api/v1/nav/categories/{id}", h.deleteCategory)
	mux.HandleFunc("POST /api/v1/nav/items", h.createItem)
	mux.HandleFunc("PUT /api/v1/nav/items/{id}", h.updateItem)
	mux.HandleFunc("DELETE /api/v1/nav/items/{id}", h.deleteItem)
}

type navHandler struct {
	nav *store.NavStore
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
	item, err := h.nav.CreateItem(store.Item{
		CategoryID: in.CategoryID, Name: in.Name, URL: in.URL, Icon: in.Icon, SortOrder: in.SortOrder,
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
	item, err := h.nav.UpdateItem(store.Item{
		ID: id, CategoryID: in.CategoryID, Name: in.Name, URL: in.URL, Icon: in.Icon, SortOrder: in.SortOrder,
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
```

- [x] **Step 4: 运行测试确认通过**

Run: `cd server && go test ./...`
Expected: 全部 PASS

- [x] **Step 5: Commit**

```bash
git add server/internal/api/nav.go server/internal/api/nav_test.go
git commit -m "feat: add nav crud endpoints with input validation"
```

---

### Task 11: webdist embed + main.go 组装

**Files:**
- Create: `server/internal/webdist/webdist.go`, `server/internal/webdist/dist/index.html`（占位，构建时被真实前端覆盖）, `server/cmd/hearth/main.go`

- [x] **Step 1: 创建 webdist 包与占位页**

```go
// server/internal/webdist/webdist.go
package webdist

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// Dist 返回前端构建产物；本地开发时为占位页，Docker 构建时为真实前端。
func Dist() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
```

```html
<!-- server/internal/webdist/dist/index.html -->
<!doctype html>
<html><body>Hearth backend is running. Frontend not embedded in this build.</body></html>
```

- [x] **Step 2: 实现 main.go**

```go
// server/cmd/hearth/main.go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/m1iktea/hearth/server/internal/api"
	"github.com/m1iktea/hearth/server/internal/collector"
	dockercol "github.com/m1iktea/hearth/server/internal/collector/docker"
	"github.com/m1iktea/hearth/server/internal/collector/openwrt"
	"github.com/m1iktea/hearth/server/internal/collector/proxmox"
	"github.com/m1iktea/hearth/server/internal/config"
	"github.com/m1iktea/hearth/server/internal/store"
	"github.com/m1iktea/hearth/server/internal/webdist"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	var collectors []collector.Collector
	if cfg.PVEEnabled() {
		collectors = append(collectors, proxmox.New(cfg.PVEURL, cfg.PVETokenID, cfg.PVETokenSecret))
	}
	dc, err := dockercol.New(cfg.DockerHost)
	if err != nil {
		return err
	}
	collectors = append(collectors, dc)
	if cfg.OpenWrtEnabled() {
		collectors = append(collectors, openwrt.New(cfg.OpenWrtURL, cfg.OpenWrtUsername, cfg.OpenWrtPassword))
	}

	snaps := store.NewSnapshotStore()
	nav, err := store.OpenNav(filepath.Join(cfg.DataDir, "hearth.db"))
	if err != nil {
		return err
	}
	defer nav.Close()

	dist, err := webdist.Dist()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sched := collector.NewScheduler(collectors, snaps, cfg.PollInterval, logger)
	go sched.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.NewRouter(snaps, nav, dist, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	logger.Info("hearth started", "addr", cfg.ListenAddr, "sources", len(collectors), "interval", cfg.PollInterval)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
```

- [x] **Step 3: 编译与全量测试**

Run: `cd server && go build ./... && go vet ./... && go test ./...`
Expected: 编译通过，全部测试 PASS

- [x] **Step 4: 冒烟验证（本机无真实 PVE 时源会显示 offline，属预期）**

Run: `cd server && HEARTH_DATA_DIR=/tmp/hearth-dev DOCKER_HOST=unix:///var/run/docker.sock go run ./cmd/hearth & sleep 2 && curl -s localhost:8080/api/v1/healthz && curl -s localhost:8080/api/v1/status && kill %1`

（若本会话 curl 被 context-mode 拦截，用 `ctx_execute` 执行等价验证。）
Expected: healthz 返回 `{"success":true,"data":"ok"}`；status 返回 docker 源快照（本机 docker 在跑则 online）

- [x] **Step 5: Commit**

```bash
git add server/internal/webdist/ server/cmd/
git commit -m "feat: wire main with scheduler, api server and embedded spa"
```

---

### Task 12: 前端脚手架

**Files:**
- Create: `web/`（Vite vue-ts 模板）+ 修改 `web/vite.config.ts`

- [x] **Step 1: 生成脚手架并安装依赖**

Run:
```bash
cd web 2>/dev/null || npm create vite@latest web -- --template vue-ts
cd web && npm install && npm install naive-ui vue-router@4 pinia && npm install -D vitest
```
Expected: `web/` 生成 vue-ts 模板，依赖安装成功

- [x] **Step 2: 配置 dev 代理与构建（替换 vite.config.ts 全文）**

```ts
// web/vite.config.ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
```

- [x] **Step 3: package.json 添加 test 脚本**

在 `web/package.json` 的 `scripts` 中加入：

```json
"test": "vitest run"
```

- [x] **Step 4: 验证构建**

Run: `cd web && npm run build`
Expected: 生成 `web/dist/`，无 TS 错误

- [x] **Step 5: Commit**

```bash
git add web/package.json web/package-lock.json web/vite.config.ts web/tsconfig*.json web/index.html web/src web/public web/.gitignore 2>/dev/null; git add -f web/.vscode 2>/dev/null; true
git commit -m "chore: scaffold vue3 + vite + ts frontend with dev proxy"
```

（注意：`web/node_modules` 与 `web/dist` 已被根 .gitignore 排除；只 add 上面列出的路径，不用 `git add -A`。）

---

### Task 13: 前端类型 + API client + utils + stores

**Files:**
- Create: `web/src/types.ts`, `web/src/api/client.ts`, `web/src/utils/format.ts`, `web/src/stores/status.ts`, `web/src/stores/nav.ts`
- Test: `web/src/utils/format.test.ts`

- [x] **Step 1: 类型定义（与后端 JSON 逐字段对应）**

```ts
// web/src/types.ts
export interface Snapshot {
  source: 'proxmox' | 'docker' | 'openwrt' | string
  status: 'online' | 'offline'
  collected_at: string
  last_error?: string
  data?: ProxmoxData | DockerData | OpenWrtData
}

export interface ProxmoxData {
  nodes: {
    name: string
    status: string
    cpu: number
    mem: number
    maxmem: number
    uptime: number
    vms: {
      vmid: number
      name: string
      status: string
      cpu: number
      mem: number
      maxmem: number
      uptime: number
    }[]
  }[]
}

export interface DockerData {
  containers: {
    id: string
    name: string
    image: string
    state: string
    status: string
  }[]
}

export interface OpenWrtData {
  hostname: string
  model: string
  release: string
  uptime_sec: number
  load: [number, number, number]
  memory: { total: number; free: number; available: number }
}

export interface NavItem {
  id: number
  category_id: number
  name: string
  url: string
  icon: string
  sort_order: number
}

export interface NavCategory {
  id: number
  name: string
  sort_order: number
  items: NavItem[]
}
```

- [x] **Step 2: API client**

```ts
// web/src/api/client.ts
interface ApiResponse<T> {
  success: boolean
  data: T
  error?: string
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })
  const env = (await res.json()) as ApiResponse<T>
  if (!res.ok || !env.success) {
    throw new Error(env.error ?? `HTTP ${res.status}`)
  }
  return env.data
}

export const apiGet = <T>(path: string) => request<T>('GET', path)
export const apiPost = <T>(path: string, body: unknown) => request<T>('POST', path, body)
export const apiPut = <T>(path: string, body: unknown) => request<T>('PUT', path, body)
export const apiDelete = <T>(path: string) => request<T>('DELETE', path)
```

- [x] **Step 3: 写 utils 失败测试**

```ts
// web/src/utils/format.test.ts
import { describe, expect, it } from 'vitest'
import { formatBytes, formatUptime, percent } from './format'

describe('formatBytes', () => {
  it('formats scales', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(1024)).toBe('1.0 KiB')
    expect(formatBytes(8589934592)).toBe('8.0 GiB')
  })
})

describe('formatUptime', () => {
  it('formats days/hours/minutes', () => {
    expect(formatUptime(59)).toBe('59s')
    expect(formatUptime(3660)).toBe('1h 1m')
    expect(formatUptime(90061)).toBe('1d 1h')
  })
})

describe('percent', () => {
  it('handles zero denominator', () => {
    expect(percent(1, 0)).toBe(0)
    expect(percent(1, 4)).toBe(25)
  })
})
```

- [x] **Step 4: 运行测试确认失败**

Run: `cd web && npm test`
Expected: FAIL（format.ts 不存在）

- [x] **Step 5: 实现 utils**

```ts
// web/src/utils/format.ts
export function formatBytes(n: number): string {
  if (n <= 0) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return i === 0 ? `${v} B` : `${v.toFixed(1)} ${units[i]}`
}

export function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

export function percent(used: number, total: number): number {
  if (total <= 0) return 0
  return Math.round((used / total) * 100)
}
```

- [x] **Step 6: 运行测试确认通过**

Run: `cd web && npm test`
Expected: PASS

- [x] **Step 7: Pinia stores**

```ts
// web/src/stores/status.ts
import { defineStore } from 'pinia'
import { apiGet } from '../api/client'
import type { Snapshot } from '../types'

export const useStatusStore = defineStore('status', {
  state: () => ({
    snapshots: [] as Snapshot[],
    loading: false,
    error: '' as string,
    timer: 0 as number,
  }),
  getters: {
    bySource: (state) => (source: string) =>
      state.snapshots.find((s) => s.source === source),
  },
  actions: {
    async fetchNow() {
      this.loading = true
      try {
        this.snapshots = await apiGet<Snapshot[]>('/api/v1/status')
        this.error = ''
      } catch (e) {
        this.error = e instanceof Error ? e.message : String(e)
      } finally {
        this.loading = false
      }
    },
    startPolling(intervalMs = 10_000) {
      this.stopPolling()
      this.fetchNow()
      this.timer = window.setInterval(() => this.fetchNow(), intervalMs)
    },
    stopPolling() {
      if (this.timer) {
        window.clearInterval(this.timer)
        this.timer = 0
      }
    },
  },
})
```

```ts
// web/src/stores/nav.ts
import { defineStore } from 'pinia'
import { apiDelete, apiGet, apiPost, apiPut } from '../api/client'
import type { NavCategory, NavItem } from '../types'

export const useNavStore = defineStore('nav', {
  state: () => ({
    categories: [] as NavCategory[],
    error: '' as string,
  }),
  actions: {
    async load() {
      try {
        this.categories = await apiGet<NavCategory[]>('/api/v1/nav')
        this.error = ''
      } catch (e) {
        this.error = e instanceof Error ? e.message : String(e)
      }
    },
    async createCategory(name: string, sortOrder: number) {
      await apiPost('/api/v1/nav/categories', { name, sort_order: sortOrder })
      await this.load()
    },
    async updateCategory(id: number, name: string, sortOrder: number) {
      await apiPut(`/api/v1/nav/categories/${id}`, { name, sort_order: sortOrder })
      await this.load()
    },
    async deleteCategory(id: number) {
      await apiDelete(`/api/v1/nav/categories/${id}`)
      await this.load()
    },
    async saveItem(item: Omit<NavItem, 'id'> & { id?: number }) {
      if (item.id) {
        await apiPut(`/api/v1/nav/items/${item.id}`, item)
      } else {
        await apiPost('/api/v1/nav/items', item)
      }
      await this.load()
    },
    async deleteItem(id: number) {
      await apiDelete(`/api/v1/nav/items/${id}`)
      await this.load()
    },
  },
})
```

- [x] **Step 8: 构建验证 + Commit**

Run: `cd web && npm run build && npm test`
Expected: 构建 PASS、测试 PASS

```bash
git add web/src/types.ts web/src/api/ web/src/utils/ web/src/stores/
git commit -m "feat: add frontend api client, stores and format utils"
```

---

### Task 14: 前端页面与路由

**Files:**
- Create: `web/src/router/index.ts`, `web/src/views/DashboardView.vue`, `web/src/views/NavView.vue`, `web/src/views/NodesView.vue`
- Modify: `web/src/main.ts`, `web/src/App.vue`
- Delete: `web/src/components/HelloWorld.vue`, `web/src/style.css` 引用按模板实际情况清理

- [x] **Step 1: router**

```ts
// web/src/router/index.ts
import { createRouter, createWebHistory } from 'vue-router'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'dashboard', component: () => import('../views/DashboardView.vue') },
    { path: '/nav', name: 'nav', component: () => import('../views/NavView.vue') },
    { path: '/nodes', name: 'nodes', component: () => import('../views/NodesView.vue') },
  ],
})
```

- [x] **Step 2: main.ts（替换全文）**

```ts
// web/src/main.ts
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { router } from './router'

createApp(App).use(createPinia()).use(router).mount('#app')
```

- [x] **Step 3: App.vue（替换全文，布局 + 菜单）**

```vue
<!-- web/src/App.vue -->
<script setup lang="ts">
import { h, computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import {
  NConfigProvider, NLayout, NLayoutSider, NLayoutContent, NMenu, darkTheme,
} from 'naive-ui'
import type { MenuOption } from 'naive-ui'

const route = useRoute()
const activeKey = computed(() => (route.name as string) ?? 'dashboard')

const menuOptions: MenuOption[] = [
  { label: () => h(RouterLink, { to: '/' }, { default: () => '仪表盘' }), key: 'dashboard' },
  { label: () => h(RouterLink, { to: '/nav' }, { default: () => '导航' }), key: 'nav' },
  { label: () => h(RouterLink, { to: '/nodes' }, { default: () => '节点详情' }), key: 'nodes' },
]
</script>

<template>
  <n-config-provider :theme="darkTheme" style="height: 100vh">
    <n-layout has-sider style="height: 100%">
      <n-layout-sider bordered :width="180">
        <div style="padding: 16px; font-size: 18px; font-weight: 600">Hearth</div>
        <n-menu :options="menuOptions" :value="activeKey" />
      </n-layout-sider>
      <n-layout-content content-style="padding: 24px">
        <router-view />
      </n-layout-content>
    </n-layout>
  </n-config-provider>
</template>
```

- [x] **Step 4: DashboardView（三源汇总卡片）**

```vue
<!-- web/src/views/DashboardView.vue -->
<script setup lang="ts">
import { onMounted, onUnmounted, computed } from 'vue'
import { NAlert, NCard, NGrid, NGi, NProgress, NTag, NStatistic } from 'naive-ui'
import { useStatusStore } from '../stores/status'
import { formatBytes, formatUptime, percent } from '../utils/format'
import type { DockerData, OpenWrtData, ProxmoxData } from '../types'

const store = useStatusStore()
onMounted(() => store.startPolling())
onUnmounted(() => store.stopPolling())

const pve = computed(() => store.bySource('proxmox'))
const docker = computed(() => store.bySource('docker'))
const openwrt = computed(() => store.bySource('openwrt'))

const pveData = computed(() => pve.value?.data as ProxmoxData | undefined)
const dockerData = computed(() => docker.value?.data as DockerData | undefined)
const wrtData = computed(() => openwrt.value?.data as OpenWrtData | undefined)

const runningContainers = computed(
  () => dockerData.value?.containers.filter((c) => c.state === 'running').length ?? 0,
)
</script>

<template>
  <n-alert v-if="store.error" type="error" style="margin-bottom: 16px">
    {{ store.error }}
  </n-alert>

  <n-grid :cols="3" :x-gap="16" :y-gap="16" responsive="screen" item-responsive>
    <n-gi span="3 m:1">
      <n-card title="Proxmox VE">
        <template #header-extra>
          <n-tag :type="pve?.status === 'online' ? 'success' : 'error'" size="small">
            {{ pve?.status ?? 'unknown' }}
          </n-tag>
        </template>
        <template v-if="pveData">
          <div v-for="node in pveData.nodes" :key="node.name">
            <n-statistic :label="`节点 ${node.name} · 运行 ${formatUptime(node.uptime)}`">
              {{ node.vms.filter((v) => v.status === 'running').length }}/{{ node.vms.length }} VM 运行中
            </n-statistic>
            <div style="margin-top: 8px">
              CPU {{ Math.round(node.cpu * 100) }}%
              <n-progress type="line" :percentage="Math.round(node.cpu * 100)" :show-indicator="false" />
            </div>
            <div style="margin-top: 8px">
              内存 {{ formatBytes(node.mem) }} / {{ formatBytes(node.maxmem) }}
              <n-progress type="line" :percentage="percent(node.mem, node.maxmem)" :show-indicator="false" />
            </div>
          </div>
        </template>
        <span v-else>{{ pve?.last_error ?? '等待数据…' }}</span>
      </n-card>
    </n-gi>

    <n-gi span="3 m:1">
      <n-card title="飞牛 Docker">
        <template #header-extra>
          <n-tag :type="docker?.status === 'online' ? 'success' : 'error'" size="small">
            {{ docker?.status ?? 'unknown' }}
          </n-tag>
        </template>
        <template v-if="dockerData">
          <n-statistic label="容器">
            {{ runningContainers }}/{{ dockerData.containers.length }} 运行中
          </n-statistic>
        </template>
        <span v-else>{{ docker?.last_error ?? '等待数据…' }}</span>
      </n-card>
    </n-gi>

    <n-gi span="3 m:1">
      <n-card title="ImmortalWrt">
        <template #header-extra>
          <n-tag :type="openwrt?.status === 'online' ? 'success' : 'error'" size="small">
            {{ openwrt?.status ?? 'unknown' }}
          </n-tag>
        </template>
        <template v-if="wrtData">
          <n-statistic :label="`${wrtData.hostname} · ${wrtData.release}`">
            运行 {{ formatUptime(wrtData.uptime_sec) }}
          </n-statistic>
          <div style="margin-top: 8px">负载 {{ wrtData.load.map((l) => l.toFixed(2)).join(' / ') }}</div>
          <div style="margin-top: 8px">
            内存可用 {{ formatBytes(wrtData.memory.available) }} / {{ formatBytes(wrtData.memory.total) }}
            <n-progress
              type="line"
              :percentage="percent(wrtData.memory.total - wrtData.memory.available, wrtData.memory.total)"
              :show-indicator="false"
            />
          </div>
        </template>
        <span v-else>{{ openwrt?.last_error ?? '等待数据…' }}</span>
      </n-card>
    </n-gi>
  </n-grid>
</template>
```

- [x] **Step 5: NavView（导航 + 管理）**

```vue
<!-- web/src/views/NavView.vue -->
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import {
  NAlert, NButton, NCard, NForm, NFormItem, NGrid, NGi, NInput, NInputNumber,
  NModal, NPopconfirm, NSelect, NSpace, NSwitch,
} from 'naive-ui'
import { useNavStore } from '../stores/nav'
import type { NavItem } from '../types'

const store = useNavStore()
onMounted(() => store.load())

const manageMode = ref(false)

// --- 分类编辑 ---
const catModal = ref(false)
const catForm = reactive({ id: 0, name: '', sort_order: 0 })
function openCatModal(id = 0, name = '', sortOrder = 0) {
  Object.assign(catForm, { id, name, sort_order: sortOrder })
  catModal.value = true
}
async function saveCat() {
  if (!catForm.name.trim()) return
  if (catForm.id) await store.updateCategory(catForm.id, catForm.name, catForm.sort_order)
  else await store.createCategory(catForm.name, catForm.sort_order)
  catModal.value = false
}

// --- 条目编辑 ---
const itemModal = ref(false)
const itemForm = reactive<Omit<NavItem, 'id'> & { id?: number }>({
  category_id: 0, name: '', url: '', icon: '', sort_order: 0,
})
function openItemModal(categoryId: number, item?: NavItem) {
  Object.assign(itemForm, item ?? {
    id: undefined, category_id: categoryId, name: '', url: '', icon: '', sort_order: 0,
  })
  itemModal.value = true
}
async function saveItem() {
  if (!itemForm.name.trim() || !itemForm.url.trim()) return
  await store.saveItem({ ...itemForm })
  itemModal.value = false
}
</script>

<template>
  <n-space justify="space-between" style="margin-bottom: 16px">
    <span style="font-size: 16px">管理模式 <n-switch v-model:value="manageMode" /></span>
    <n-button v-if="manageMode" type="primary" @click="openCatModal()">新增分类</n-button>
  </n-space>

  <n-alert v-if="store.error" type="error" style="margin-bottom: 16px">{{ store.error }}</n-alert>

  <div v-for="cat in store.categories" :key="cat.id" style="margin-bottom: 24px">
    <n-space align="center" style="margin-bottom: 8px">
      <h3 style="margin: 0">{{ cat.name }}</h3>
      <template v-if="manageMode">
        <n-button size="tiny" @click="openCatModal(cat.id, cat.name, cat.sort_order)">改名</n-button>
        <n-popconfirm @positive-click="store.deleteCategory(cat.id)">
          <template #trigger><n-button size="tiny" type="error">删除</n-button></template>
          删除分类会同时删除其下所有链接，确认？
        </n-popconfirm>
        <n-button size="tiny" type="primary" @click="openItemModal(cat.id)">加链接</n-button>
      </template>
    </n-space>
    <n-grid :cols="4" :x-gap="12" :y-gap="12" responsive="screen" item-responsive>
      <n-gi v-for="item in cat.items" :key="item.id" span="4 m:1">
        <n-card size="small" hoverable>
          <a :href="item.url" target="_blank" rel="noopener" style="text-decoration: none; color: inherit">
            <strong>{{ item.icon ? item.icon + ' ' : '' }}{{ item.name }}</strong>
            <div style="font-size: 12px; opacity: 0.6">{{ item.url }}</div>
          </a>
          <n-space v-if="manageMode" style="margin-top: 8px">
            <n-button size="tiny" @click="openItemModal(cat.id, item)">编辑</n-button>
            <n-popconfirm @positive-click="store.deleteItem(item.id)">
              <template #trigger><n-button size="tiny" type="error">删除</n-button></template>
              确认删除该链接？
            </n-popconfirm>
          </n-space>
        </n-card>
      </n-gi>
    </n-grid>
  </div>

  <n-modal v-model:show="catModal" preset="card" title="分类" style="width: 400px">
    <n-form>
      <n-form-item label="名称"><n-input v-model:value="catForm.name" /></n-form-item>
      <n-form-item label="排序"><n-input-number v-model:value="catForm.sort_order" /></n-form-item>
    </n-form>
    <n-button type="primary" block @click="saveCat">保存</n-button>
  </n-modal>

  <n-modal v-model:show="itemModal" preset="card" title="链接" style="width: 400px">
    <n-form>
      <n-form-item label="分类">
        <n-select
          v-model:value="itemForm.category_id"
          :options="store.categories.map((c) => ({ label: c.name, value: c.id }))"
        />
      </n-form-item>
      <n-form-item label="名称"><n-input v-model:value="itemForm.name" /></n-form-item>
      <n-form-item label="URL"><n-input v-model:value="itemForm.url" placeholder="https://..." /></n-form-item>
      <n-form-item label="图标（emoji）"><n-input v-model:value="itemForm.icon" placeholder="🖥️" /></n-form-item>
      <n-form-item label="排序"><n-input-number v-model:value="itemForm.sort_order" /></n-form-item>
    </n-form>
    <n-button type="primary" block @click="saveItem">保存</n-button>
  </n-modal>
</template>
```

- [x] **Step 6: NodesView（各源详情表格）**

```vue
<!-- web/src/views/NodesView.vue -->
<script setup lang="ts">
import { onMounted, onUnmounted, computed } from 'vue'
import { NCard, NSpace, NTable, NTag } from 'naive-ui'
import { useStatusStore } from '../stores/status'
import { formatBytes, formatUptime } from '../utils/format'
import type { DockerData, OpenWrtData, ProxmoxData } from '../types'

const store = useStatusStore()
onMounted(() => store.startPolling())
onUnmounted(() => store.stopPolling())

const pveData = computed(() => store.bySource('proxmox')?.data as ProxmoxData | undefined)
const dockerData = computed(() => store.bySource('docker')?.data as DockerData | undefined)
const wrtData = computed(() => store.bySource('openwrt')?.data as OpenWrtData | undefined)
</script>

<template>
  <n-space vertical :size="16">
    <n-card title="Proxmox VM">
      <n-table v-if="pveData" size="small">
        <thead>
          <tr><th>VMID</th><th>名称</th><th>状态</th><th>CPU</th><th>内存</th><th>运行时长</th></tr>
        </thead>
        <tbody>
          <template v-for="node in pveData.nodes" :key="node.name">
            <tr v-for="vm in node.vms" :key="vm.vmid">
              <td>{{ vm.vmid }}</td>
              <td>{{ vm.name }}</td>
              <td>
                <n-tag :type="vm.status === 'running' ? 'success' : 'default'" size="small">
                  {{ vm.status }}
                </n-tag>
              </td>
              <td>{{ Math.round(vm.cpu * 100) }}%</td>
              <td>{{ formatBytes(vm.mem) }} / {{ formatBytes(vm.maxmem) }}</td>
              <td>{{ vm.status === 'running' ? formatUptime(vm.uptime) : '-' }}</td>
            </tr>
          </template>
        </tbody>
      </n-table>
      <span v-else>数据源离线或等待数据…</span>
    </n-card>

    <n-card title="Docker 容器">
      <n-table v-if="dockerData" size="small">
        <thead>
          <tr><th>名称</th><th>镜像</th><th>状态</th><th>详情</th></tr>
        </thead>
        <tbody>
          <tr v-for="c in dockerData.containers" :key="c.id">
            <td>{{ c.name }}</td>
            <td>{{ c.image }}</td>
            <td>
              <n-tag :type="c.state === 'running' ? 'success' : 'warning'" size="small">{{ c.state }}</n-tag>
            </td>
            <td>{{ c.status }}</td>
          </tr>
        </tbody>
      </n-table>
      <span v-else>数据源离线或等待数据…</span>
    </n-card>

    <n-card title="ImmortalWrt">
      <n-table v-if="wrtData" size="small">
        <tbody>
          <tr><td>主机名</td><td>{{ wrtData.hostname }}</td></tr>
          <tr><td>型号</td><td>{{ wrtData.model }}</td></tr>
          <tr><td>系统</td><td>{{ wrtData.release }}</td></tr>
          <tr><td>运行时长</td><td>{{ formatUptime(wrtData.uptime_sec) }}</td></tr>
          <tr><td>负载</td><td>{{ wrtData.load.map((l) => l.toFixed(2)).join(' / ') }}</td></tr>
          <tr>
            <td>内存</td>
            <td>可用 {{ formatBytes(wrtData.memory.available) }} / 共 {{ formatBytes(wrtData.memory.total) }}</td>
          </tr>
        </tbody>
      </n-table>
      <span v-else>数据源离线或等待数据…</span>
    </n-card>
  </n-space>
</template>
```

- [x] **Step 7: 清理模板残留**

删除 `web/src/components/HelloWorld.vue`；`web/src/style.css` 精简为：

```css
body {
  margin: 0;
  font-family: -apple-system, 'Segoe UI', Roboto, 'PingFang SC', 'Microsoft YaHei', sans-serif;
}
```

若 main.ts 模板原本 import 了 `./style.css`，保留该 import（上面 Step 2 的 main.ts 没有引 style.css，则在 `index.html` 无需改动的情况下直接删除 style.css 亦可——两者选一，保证 `npm run build` 通过）。

- [x] **Step 8: 构建 + 测试验证**

Run: `cd web && npm run build && npm test`
Expected: 构建 PASS（无 TS 错误）、vitest PASS

- [x] **Step 9: 前后端联调冒烟**

Run（两个终端或后台）:
```bash
cd server && HEARTH_DATA_DIR=/tmp/hearth-dev go run ./cmd/hearth &
cd web && npm run dev
```
浏览器打开 `http://localhost:5173`：仪表盘显示 docker 卡片（本机 docker 在跑则 online；pve/openwrt 未配置不显示），导航页可增删分类与链接。验证后停止两个进程。

- [x] **Step 10: Commit**

```bash
git add web/src/
git commit -m "feat: add dashboard, nav and nodes pages"
```

---

### Task 15: Docker 构建与编排

**Files:**
- Create: `deploy/Dockerfile`, `deploy/docker-compose.yml`, `deploy/.env.example`

- [x] **Step 1: Dockerfile（构建上下文为仓库根目录）**

```dockerfile
# deploy/Dockerfile
# --- 前端构建 ---
FROM node:20-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# --- 后端构建（embed 前端产物） ---
FROM golang:1.23-alpine AS server
WORKDIR /app/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=web /app/web/dist ./internal/webdist/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /hearth ./cmd/hearth

# --- 运行镜像（非 root） ---
FROM alpine:3.20
RUN adduser -D -u 1000 hearth && mkdir -p /data && chown hearth:hearth /data
COPY --from=server /hearth /usr/local/bin/hearth
USER hearth
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["hearth"]
```

- [x] **Step 2: docker-compose.yml**

```yaml
# deploy/docker-compose.yml
services:
  hearth:
    image: hearth:latest
    build:
      context: ..
      dockerfile: deploy/Dockerfile
    container_name: hearth
    restart: unless-stopped
    ports:
      - "8080:8080"
    env_file: .env
    volumes:
      - ./data:/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
    # 非 root 用户读 docker.sock 需要加入 sock 所属组；
    # 在飞牛上执行 `stat -c %g /var/run/docker.sock` 获取 GID 填入 .env 的 DOCKER_GID
    group_add:
      - "${DOCKER_GID:-999}"
```

- [x] **Step 3: .env.example**

```bash
# deploy/.env.example — 复制为 .env 并填入实际值（.env 已被 gitignore）

# Proxmox（Datacenter → Permissions → API Tokens 创建，只读角色 PVEAuditor 即可）
PVE_URL=https://192.168.1.2:8006
PVE_TOKEN_ID=hearth@pam!hearth
PVE_TOKEN_SECRET=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# Docker（容器内通过挂载的 socket 访问，通常无需修改）
DOCKER_HOST=unix:///var/run/docker.sock
# 宿主机 docker.sock 的属组 GID：stat -c %g /var/run/docker.sock
DOCKER_GID=999

# ImmortalWrt（LuCI 登录账号；建议单独建一个只读账号）
OPENWRT_URL=http://192.168.1.1
OPENWRT_USERNAME=root
OPENWRT_PASSWORD=change-me

# 可选
HEARTH_POLL_INTERVAL=10s
HEARTH_LISTEN=:8080
HEARTH_DATA_DIR=/data
```

- [x] **Step 4: 本机构建验证**

Run: `docker build -f deploy/Dockerfile -t hearth:dev .`（在仓库根目录）
Expected: 构建成功。随后 `docker run --rm -e PVE_URL= -e HEARTH_DATA_DIR=/tmp hearth:dev` 应因数据目录只读或 docker 源探测失败以外的原因正常启动（healthz 可用）；快速验证后 Ctrl+C。

- [x] **Step 5: Commit**

```bash
git add deploy/Dockerfile deploy/docker-compose.yml deploy/.env.example
git commit -m "feat: add docker build and compose deployment"
```

---

### Task 16: 文档（README + 部署说明）

**Files:**
- Create: `README.md`, `docs/deploy.md`

- [x] **Step 1: README.md**

内容要点（用中文写，200 行内）：
- 项目简介：自研家庭中枢，MVP = 导航页 + PVE/Docker/ImmortalWrt 状态监控 + 统一仪表盘
- 架构图（文字版）：`collectors → snapshot store → REST API → Vue SPA（embed）`
- 技术栈与目录结构（引用本计划 File Structure 一节的树）
- 快速开始（本地开发）：
  - 后端：`cd server && HEARTH_DATA_DIR=/tmp/hearth-dev go run ./cmd/hearth`（配好环境变量）
  - 前端：`cd web && npm install && npm run dev`（dev proxy 指向 :8080）
  - 测试：`cd server && go test ./...`；`cd web && npm test`
- 部署：指向 `docs/deploy.md`
- Roadmap：设备控制 / 自动化 / 通知聚合 / 历史数据看板（对应 spec 第 9 节）

- [x] **Step 2: docs/deploy.md**

内容要点：
- 前置：飞牛 SSH 可用、docker compose 可用
- PVE API Token 创建步骤（只读 PVEAuditor 角色）与 ImmortalWrt 只读账号建议
- 首次部署：clone 仓库 → `cp deploy/.env.example deploy/.env` 并填写 → `stat -c %g /var/run/docker.sock` 填 DOCKER_GID → `cd deploy && docker compose up -d --build`
- 更新流程：`git pull && docker compose up -d --build`（对应 spec「push → 构建镜像 → 重启」）
- 数据持久化说明：`deploy/data/` 即 SQLite 所在，备份即拷贝该目录
- 常见问题：某源显示 offline 时看 `docker logs hearth` 中对应 source 的 warn 日志

- [x] **Step 3: Commit**

```bash
git add README.md docs/deploy.md
git commit -m "docs: add readme and deployment guide"
```

---

### Task 17: 收尾验证

- [x] **Step 1: 全量回归**

Run: `cd server && go vet ./... && go test ./...`
Expected: 全部 PASS

Run: `cd web && npm run build && npm test`
Expected: PASS

- [x] **Step 2: 覆盖率检查（核心逻辑 80%+）**

Run: `cd server && go test ./... -coverprofile=/tmp/cover.out && go tool cover -func=/tmp/cover.out | tail -1`
Expected: total 覆盖率 ≥ 80%（collector/store/api 为核心；main.go 组装代码不强求）。若不足，为缺口路径补测试。

- [x] **Step 3: push**

```bash
git push origin main
```

---

## Self-Review 记录

- **Spec coverage**：导航 CRUD（Task 8/10/14）、三源采集（Task 4/5/6）、仪表盘（Task 14）、快照与故障隔离（Task 3/7）、SQLite 持久化（Task 8）、embed 单二进制（Task 11）、Docker 部署与更新流程（Task 15/16）、auth 插槽（Task 9 `authStub`）、凭据环境变量（Task 2/15）——spec 第 1-10 节均有对应任务。
- **类型一致性**：`Collector.Collect(ctx) (any, error)` 贯穿 Task 3-7；`SnapshotStore.SetOK/SetError/Get/All` 贯穿 Task 3/7/9；`store.Item/Category` 字段与前端 `types.ts`、API JSON 三方对齐（snake_case）。
- **已知取舍**：PVE 自签名证书用 `InsecureSkipVerify`（家庭内网，spec 无 CA 要求）；前端组件测试仅覆盖纯逻辑 utils（UI 由构建 + 冒烟覆盖）；LXC 容器（`/lxc` 端点）MVP 不采集，属 spec 外。
```
