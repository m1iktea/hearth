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
	if cfg.HealthInterval != 30*time.Second || len(cfg.ScanNetworks) != 0 || cfg.ARPDiscoveryEnabled {
		t.Errorf("HealthInterval=%v ScanNetworks=%v ARPDiscoveryEnabled=%v", cfg.HealthInterval, cfg.ScanNetworks, cfg.ARPDiscoveryEnabled)
	}
	if cfg.DataDir != "/data" || cfg.ListenAddr != ":8080" {
		t.Errorf("DataDir=%q ListenAddr=%q", cfg.DataDir, cfg.ListenAddr)
	}
}

func TestLoadARPDiscoveryEnabled(t *testing.T) {
	cfg, err := Load(getenvFrom(map[string]string{"HEARTH_ARP_DISCOVERY_ENABLED": "true"}))
	if err != nil || !cfg.ARPDiscoveryEnabled {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
	if _, err := Load(getenvFrom(map[string]string{"HEARTH_ARP_DISCOVERY_ENABLED": "invalid"})); err == nil {
		t.Error("want invalid boolean error")
	}
}

func TestLoadScanNetworks(t *testing.T) {
	cfg, err := Load(getenvFrom(map[string]string{"HEARTH_SCAN_NETWORKS": "192.168.1.0/24, 10.0.0.0/24"}))
	if err != nil || len(cfg.ScanNetworks) != 2 {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
	if _, err := Load(getenvFrom(map[string]string{"HEARTH_SCAN_NETWORKS": "not-a-cidr"})); err == nil {
		t.Error("want invalid CIDR error")
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

func TestLoadNonPositiveInterval(t *testing.T) {
	for _, v := range []string{"-5s", "0s"} {
		_, err := Load(getenvFrom(map[string]string{
			"PVE_URL": "u", "PVE_TOKEN_ID": "i", "PVE_TOKEN_SECRET": "s",
			"HEARTH_POLL_INTERVAL": v,
		}))
		if err == nil {
			t.Errorf("want error for interval %q", v)
		}
	}
}
