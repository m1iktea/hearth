// Package health 定期执行设备健康检查。它只做探测和状态切换，不包含任何设备控制操作。
package health

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/m1iktea/hearth/server/internal/store"
)

type checkStore interface {
	ListEnabledChecks() ([]store.CheckWithDevice, error)
	RecordProbe(store.CheckWithDevice, string, string, int64, time.Time) error
}

type Runner struct {
	store    checkStore
	interval time.Duration
	logger   *slog.Logger
	client   *http.Client
}

func NewRunner(s checkStore, interval time.Duration, logger *slog.Logger) *Runner {
	return &Runner{store: s, interval: interval, logger: logger, client: &http.Client{Timeout: 4 * time.Second}}
}

func (r *Runner) Run(ctx context.Context) {
	r.RunOnce(ctx)
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.RunOnce(ctx)
		}
	}
}

func (r *Runner) RunOnce(ctx context.Context) {
	checks, err := r.store.ListEnabledChecks()
	if err != nil {
		r.logger.Warn("list health checks failed", "error", err)
		return
	}
	var wg sync.WaitGroup
	for _, check := range checks {
		wg.Add(1)
		go func(c store.CheckWithDevice) {
			defer wg.Done()
			probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			status, msg, latency := r.probe(probeCtx, c)
			if err := r.store.RecordProbe(c, status, msg, latency, time.Now()); err != nil {
				r.logger.Warn("record health probe failed", "check", c.ID, "error", err)
			}
		}(check)
	}
	wg.Wait()
}

func target(c store.CheckWithDevice) string {
	if strings.TrimSpace(c.Target) != "" {
		return strings.TrimSpace(c.Target)
	}
	return strings.TrimSpace(c.DeviceIP)
}

func (r *Runner) probe(ctx context.Context, c store.CheckWithDevice) (string, string, int64) {
	start := time.Now()
	t := target(c)
	if t == "" {
		return "offline", "未设置检查目标或设备 IP", 0
	}
	var err error
	switch c.Type {
	case "ping":
		err = ping(ctx, t)
	case "tcp":
		err = tcp(ctx, t, c.Port)
	case "http":
		err = r.http(ctx, t, c.ExpectedStatus)
	default:
		err = fmt.Errorf("不支持的检查类型 %q", c.Type)
	}
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return "offline", err.Error(), latency
	}
	return "online", "正常", latency
}

// ping 使用系统 ping，以免服务自行持有 raw socket 权限。容器镜像会安装 iputils，
// compose 仅增加 NET_RAW 能力；TCP/HTTP 检查不需要这项能力。
func ping(ctx context.Context, host string) error {
	return exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", host).Run()
}
func tcp(ctx context.Context, host string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("TCP 端口必须在 1-65535")
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err == nil {
		conn.Close()
	}
	return err
}
func (r *Runner) http(ctx context.Context, raw string, expected int) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("HTTP 目标必须是完整 URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return err
	}
	res, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if expected > 0 && res.StatusCode != expected {
		return fmt.Errorf("HTTP 状态 %d，期望 %d", res.StatusCode, expected)
	}
	if expected == 0 && (res.StatusCode < 200 || res.StatusCode >= 400) {
		return fmt.Errorf("HTTP 状态 %d", res.StatusCode)
	}
	return nil
}
