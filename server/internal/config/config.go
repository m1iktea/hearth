package config

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

// Config 全部来自环境变量，凭据不入库不进代码。
type Config struct {
	PVEURL         string
	PVETokenID     string
	PVETokenSecret string

	// PVE 宿主机温度采集（SSH + lm-sensors），可选。
	PVESSHHost     string
	PVESSHUser     string
	PVESSHPassword string
	PVESSHKeyFile  string

	DockerHost string

	OpenWrtURL      string
	OpenWrtUsername string
	OpenWrtPassword string

	PollInterval        time.Duration
	HealthInterval      time.Duration
	ScanNetworks        []string
	ARPDiscoveryEnabled bool
	DataDir             string
	ListenAddr          string

	// 黑匣子留存策略：事件与指标的保留期、指标落盘采样间隔。
	EventRetention       time.Duration
	MetricRetention      time.Duration
	MetricSampleInterval time.Duration
}

// PVEEnabled / OpenWrtEnabled 由 URL 是否配置决定；Docker 恒启用（有默认 socket）。
func (c *Config) PVEEnabled() bool        { return c.PVEURL != "" }
func (c *Config) OpenWrtEnabled() bool    { return c.OpenWrtURL != "" }
func (c *Config) PVESensorsEnabled() bool { return c.PVESSHHost != "" }

func Load(getenv func(string) string) (*Config, error) {
	cfg := &Config{
		PVEURL:          getenv("PVE_URL"),
		PVETokenID:      getenv("PVE_TOKEN_ID"),
		PVETokenSecret:  getenv("PVE_TOKEN_SECRET"),
		DockerHost:      getenv("DOCKER_HOST"),
		PVESSHHost:      getenv("PVE_SSH_HOST"),
		PVESSHUser:      getenv("PVE_SSH_USER"),
		PVESSHPassword:  getenv("PVE_SSH_PASSWORD"),
		PVESSHKeyFile:   getenv("PVE_SSH_KEY_FILE"),
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
	if raw := strings.TrimSpace(getenv("HEARTH_SCAN_NETWORKS")); raw != "" {
		for _, value := range strings.Split(raw, ",") {
			cidr := strings.TrimSpace(value)
			if _, err := netip.ParsePrefix(cidr); err != nil {
				return nil, fmt.Errorf("invalid HEARTH_SCAN_NETWORKS CIDR %q: %w", cidr, err)
			}
			cfg.ScanNetworks = append(cfg.ScanNetworks, cidr)
		}
	}
	if raw := strings.TrimSpace(getenv("HEARTH_ARP_DISCOVERY_ENABLED")); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid HEARTH_ARP_DISCOVERY_ENABLED %q: %w", raw, err)
		}
		cfg.ARPDiscoveryEnabled = enabled
	}

	interval := getenv("HEARTH_POLL_INTERVAL")
	if interval == "" {
		cfg.PollInterval = 10 * time.Second
	} else {
		d, err := time.ParseDuration(interval)
		if err != nil {
			return nil, fmt.Errorf("invalid HEARTH_POLL_INTERVAL %q: %w", interval, err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("HEARTH_POLL_INTERVAL must be positive, got %q", interval)
		}
		cfg.PollInterval = d
	}

	healthInterval := getenv("HEARTH_HEALTH_INTERVAL")
	if healthInterval == "" {
		cfg.HealthInterval = 30 * time.Second
	} else {
		d, err := time.ParseDuration(healthInterval)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("HEARTH_HEALTH_INTERVAL must be a positive duration, got %q", healthInterval)
		}
		cfg.HealthInterval = d
	}

	eventRetention, err := loadRetentionDays(getenv, "HEARTH_EVENT_RETENTION_DAYS", 90)
	if err != nil {
		return nil, err
	}
	cfg.EventRetention = eventRetention
	metricRetention, err := loadRetentionDays(getenv, "HEARTH_METRIC_RETENTION_DAYS", 30)
	if err != nil {
		return nil, err
	}
	cfg.MetricRetention = metricRetention
	sampleInterval := getenv("HEARTH_METRIC_SAMPLE_INTERVAL")
	if sampleInterval == "" {
		cfg.MetricSampleInterval = time.Minute
	} else {
		d, err := time.ParseDuration(sampleInterval)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("HEARTH_METRIC_SAMPLE_INTERVAL must be a positive duration, got %q", sampleInterval)
		}
		cfg.MetricSampleInterval = d
	}

	if cfg.PVEEnabled() && (cfg.PVETokenID == "" || cfg.PVETokenSecret == "") {
		return nil, errors.New("PVE_URL is set but PVE_TOKEN_ID/PVE_TOKEN_SECRET missing")
	}
	if cfg.OpenWrtEnabled() && (cfg.OpenWrtUsername == "" || cfg.OpenWrtPassword == "") {
		return nil, errors.New("OPENWRT_URL is set but OPENWRT_USERNAME/OPENWRT_PASSWORD missing")
	}
	if cfg.PVESensorsEnabled() && (cfg.PVESSHUser == "" || (cfg.PVESSHPassword == "" && cfg.PVESSHKeyFile == "")) {
		return nil, errors.New("PVE_SSH_HOST is set but PVE_SSH_USER and PVE_SSH_PASSWORD/PVE_SSH_KEY_FILE missing")
	}
	return cfg, nil
}

// loadRetentionDays 解析以“天”为单位的保留期环境变量。
func loadRetentionDays(getenv func(string) string, key string, defaultDays int) (time.Duration, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return time.Duration(defaultDays) * 24 * time.Hour, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer (days), got %q", key, raw)
	}
	return time.Duration(days) * 24 * time.Hour, nil
}
