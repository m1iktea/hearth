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
