// Package sensors 通过 SSH 采集 PVE 宿主机温度（lm-sensors），补齐 PVE API
// 不暴露的硬件指标，样本写入黑匣子 metric_samples 供故障回看。
package sensors

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/m1iktea/hearth/server/internal/store"
)

// sampleSink 是探针对持久层的最小依赖（由 store.InventoryStore 满足）。
type sampleSink interface {
	InsertSamples(samples []store.MetricSample) error
}

type Config struct {
	Addr     string // host 或 host:port，缺省端口补 22
	User     string
	Password string
	KeyFile  string
}

type Prober struct {
	addr      string
	sshConfig *ssh.ClientConfig
	sink      sampleSink
	interval  time.Duration
	logger    *slog.Logger
}

func NewProber(cfg Config, sink sampleSink, interval time.Duration, logger *slog.Logger) (*Prober, error) {
	var auth []ssh.AuthMethod
	if cfg.KeyFile != "" {
		raw, err := os.ReadFile(cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("read ssh key %s: %w", cfg.KeyFile, err)
		}
		signer, err := ssh.ParsePrivateKey(raw)
		if err != nil {
			return nil, fmt.Errorf("parse ssh key %s: %w", cfg.KeyFile, err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		auth = append(auth, ssh.Password(cfg.Password))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("pve ssh sensors: no auth method configured")
	}
	addr := cfg.Addr
	if !strings.Contains(addr, ":") {
		addr += ":22"
	}
	return &Prober{
		addr: addr,
		sshConfig: &ssh.ClientConfig{
			User: cfg.User,
			Auth: auth,
			// 家庭局域网内的 PVE 宿主机，与 PVE API 跳过 TLS 校验保持同等信任假设
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
			Timeout:         8 * time.Second,
		},
		sink:     sink,
		interval: interval,
		logger:   logger,
	}, nil
}

// Run 先立即探测一次，然后按 interval 循环，直到 ctx 取消。
func (p *Prober) Run(ctx context.Context) {
	p.probeOnce(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.probeOnce(ctx)
		}
	}
}

func (p *Prober) probeOnce(ctx context.Context) {
	out, err := p.runSensors(ctx)
	if err != nil {
		p.logger.Warn("sensors probe failed", "addr", p.addr, "error", err)
		return
	}
	temps, err := ParseSensorsJSON(out)
	if err != nil {
		p.logger.Warn("sensors parse failed", "addr", p.addr, "error", err)
		return
	}
	now := time.Now()
	samples := make([]store.MetricSample, 0, len(temps))
	for chip, value := range temps {
		samples = append(samples, store.MetricSample{
			Source: "proxmox", Object: chip, Metric: "temp_c", Value: value, CreatedAt: now,
		})
	}
	if err := p.sink.InsertSamples(samples); err != nil {
		p.logger.Warn("persist temperature samples failed", "error", err)
	}
}

func (p *Prober) runSensors(ctx context.Context) ([]byte, error) {
	dialer := net.Dialer{Timeout: p.sshConfig.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", p.addr)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, p.addr, p.sshConfig)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()
	// 命令挂起时由看门狗关闭连接兜底，避免探针 goroutine 卡死
	watchdog := time.AfterFunc(15*time.Second, func() { client.Close() })
	defer watchdog.Stop()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	defer session.Close()
	out, err := session.Output("sensors -j")
	if err != nil {
		return nil, fmt.Errorf("run sensors -j: %w", err)
	}
	return out, nil
}

// ParseSensorsJSON 解析 `sensors -j` 输出，返回每个芯片的最高温度（摄氏度）。
// 结构：{ "<chip>": { "<feature>": { "tempN_input": 45.0, ... }, ... }, ... }
func ParseSensorsJSON(raw []byte) (map[string]float64, error) {
	var chips map[string]map[string]any
	if err := json.Unmarshal(raw, &chips); err != nil {
		return nil, fmt.Errorf("decode sensors json: %w", err)
	}
	out := map[string]float64{}
	for chip, features := range chips {
		max, found := 0.0, false
		for _, feature := range features {
			values, ok := feature.(map[string]any)
			if !ok {
				continue // 如 "Adapter": "ISA adapter"
			}
			for key, v := range values {
				num, ok := v.(float64)
				if !ok || !strings.HasPrefix(key, "temp") || !strings.HasSuffix(key, "_input") {
					continue
				}
				if !found || num > max {
					max, found = num, true
				}
			}
		}
		if found {
			out[chip] = max
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no temperature inputs found (lm-sensors 未安装或无温度传感器)")
	}
	return out, nil
}

// SortedChips 便于日志与测试的稳定输出。
func SortedChips(temps map[string]float64) []string {
	chips := make([]string, 0, len(temps))
	for chip := range temps {
		chips = append(chips, chip)
	}
	sort.Strings(chips)
	return chips
}
