package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
)

// 编译期断言：*Client 满足 collector.Collector 接口。
var _ collector.Collector = (*Client)(nil)

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
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
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
