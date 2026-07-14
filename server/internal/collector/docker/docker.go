package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/m1iktea/hearth/server/internal/collector"
)

var _ collector.Collector = (*Client)(nil)

// cpuBaseline 保存上一轮采集的 CPU 累计值，用于差分计算。
type cpuBaseline struct {
	totalUsage  uint64
	systemUsage uint64
}

// Client 通过 Docker Engine API 读取容器列表。
type Client struct {
	baseURL  string
	http     *http.Client
	mu       sync.Mutex
	baseline map[string]cpuBaseline // containerID -> 上轮累计值
}

type Data struct {
	Containers []Container `json:"containers"`
}

type Container struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Image    string   `json:"image"`
	State    string   `json:"state"`   // running | exited | ...
	Status   string   `json:"status"`  // "Up 2 hours"
	CpuPct   *float64 `json:"cpu_pct"` // nil 表示无基线（第一轮或重启后）
	MemUsed  int64    `json:"mem_used"`
	MemLimit int64    `json:"mem_limit"`
}

func New(host string) (*Client, error) {
	c := &Client{
		http:     &http.Client{Timeout: 8 * time.Second},
		baseline: make(map[string]cpuBaseline),
	}
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

// fetchStats 拉取单容器的 stats 并返回 CPU% 和内存用量。
// cpuPct 为 nil 表示无基线（首轮或差分值异常）。
func (c *Client) fetchStats(ctx context.Context, id string) (cpuPct *float64, memUsed, memLimit int64, err error) {
	url := fmt.Sprintf("%s/containers/%s/stats?stream=false&one-shot=true", c.baseURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, 0, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, 0, fmt.Errorf("stats api returned %s", resp.Status)
	}

	var s struct {
		CPUStats struct {
			CPUUsage struct {
				TotalUsage uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			SystemCPUUsage uint64 `json:"system_cpu_usage"`
			OnlineCPUs     int    `json:"online_cpus"`
		} `json:"cpu_stats"`
		MemoryStats struct {
			Usage uint64            `json:"usage"`
			Limit uint64            `json:"limit"`
			Stats map[string]uint64 `json:"stats"`
		} `json:"memory_stats"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, 0, 0, fmt.Errorf("decode stats: %w", err)
	}

	// --- CPU 差分计算 ---
	totalUsage := s.CPUStats.CPUUsage.TotalUsage
	systemUsage := s.CPUStats.SystemCPUUsage
	onlineCPUs := s.CPUStats.OnlineCPUs
	if onlineCPUs == 0 {
		onlineCPUs = 1
	}

	c.mu.Lock()
	prev, hasPrev := c.baseline[id]
	c.baseline[id] = cpuBaseline{totalUsage: totalUsage, systemUsage: systemUsage}
	c.mu.Unlock()

	if hasPrev {
		cpuDelta := int64(totalUsage) - int64(prev.totalUsage)
		sysDelta := int64(systemUsage) - int64(prev.systemUsage)
		if sysDelta > 0 && cpuDelta >= 0 {
			v := float64(cpuDelta) / float64(sysDelta) * float64(onlineCPUs) * 100
			cpuPct = &v
		}
		// sysDelta <= 0 或 cpuDelta < 0 → cpuPct 保持 nil
	}
	// 无 hasPrev → cpuPct 保持 nil

	// --- 内存口径（cgroup v2 优先）---
	used := s.MemoryStats.Usage
	if inactiveFile, ok := s.MemoryStats.Stats["inactive_file"]; ok {
		if inactiveFile < used {
			used -= inactiveFile
		} else {
			used = 0
		}
	} else if cache, ok := s.MemoryStats.Stats["cache"]; ok {
		if cache < used {
			used -= cache
		} else {
			used = 0
		}
	}

	return cpuPct, int64(used), int64(s.MemoryStats.Limit), nil
}

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

	containers := make([]Container, 0, len(raw))
	for _, r := range raw {
		name := ""
		if len(r.Names) > 0 {
			name = strings.TrimPrefix(r.Names[0], "/")
		}
		containers = append(containers, Container{
			ID: r.ID, Name: name, Image: r.Image, State: r.State, Status: r.Status,
		})
	}

	// 对 running 容器并发拉取 stats，限并发 8。
	const maxConcurrent = 8
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	// statsMap 保存各容器的采集结果。
	type statsResult struct {
		cpuPct   *float64
		memUsed  int64
		memLimit int64
	}
	statsMap := make(map[string]statsResult, len(containers))

	for _, ct := range containers {
		if ct.State != "running" {
			continue
		}
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cpuPct, memUsed, memLimit, ferr := c.fetchStats(ctx, id)
			if ferr != nil {
				log.Printf("docker: fetchStats %s: %v", id, ferr)
				// 失败时字段保持零值
				cpuPct = nil
				memUsed = 0
				memLimit = 0
			}
			mu.Lock()
			statsMap[id] = statsResult{cpuPct: cpuPct, memUsed: memUsed, memLimit: memLimit}
			mu.Unlock()
		}(ct.ID)
	}
	wg.Wait()

	// 将 stats 写回容器列表。
	for i := range containers {
		if sr, ok := statsMap[containers[i].ID]; ok {
			containers[i].CpuPct = sr.cpuPct
			containers[i].MemUsed = sr.memUsed
			containers[i].MemLimit = sr.memLimit
		}
	}

	return Data{Containers: containers}, nil
}
