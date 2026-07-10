package openwrt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
)

var _ collector.Collector = (*Client)(nil)

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
