package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
)

var _ collector.Collector = (*Client)(nil)

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
