// Package discovery implements local-LAN discovery. ARP is intentionally used
// instead of DHCP data: it works even when OpenWrt is not the DHCP server.
package discovery

import (
	"context"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
)

type Device struct {
	IPAddress  string `json:"ip_address"`
	MACAddress string `json:"mac_address"`
	Vendor     string `json:"vendor"`
}

type ARPScanner struct {
	defaultNetworks []string
	run             func(context.Context, string, ...string) ([]byte, error)
}

func NewARPScanner(networks []string) *ARPScanner {
	return &ARPScanner{
		defaultNetworks: networks,
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).Output()
		},
	}
}

// Scan discovers hosts on the same Layer-2 network. With no configured CIDR it
// delegates interface and subnet selection to arp-scan's --localnet.
func (s *ARPScanner) Scan(ctx context.Context) ([]Device, error) {
	networks := s.defaultNetworks
	if len(networks) == 0 {
		items, err := s.runScan(ctx, "--localnet")
		return uniqueByMAC(items), err
	}
	all := []Device{}
	seen := map[string]bool{}
	for _, network := range networks {
		items, err := s.runScan(ctx, network)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", network, err)
		}
		for _, item := range items {
			if !seen[item.MACAddress] {
				all = append(all, item)
				seen[item.MACAddress] = true
			}
		}
	}
	return uniqueByMAC(all), nil
}

func uniqueByMAC(items []Device) []Device {
	seen := map[string]bool{}
	unique := make([]Device, 0, len(items))
	for _, item := range items {
		if item.MACAddress == "" || seen[item.MACAddress] {
			continue
		}
		seen[item.MACAddress] = true
		unique = append(unique, item)
	}
	return unique
}

func (s *ARPScanner) runScan(ctx context.Context, network string) ([]Device, error) {
	args := []string{"--plain", "--numeric", "--retry=1", "--timeout=150", network}
	output, err := s.run(ctx, "arp-scan", args...)
	if err != nil {
		return nil, fmt.Errorf("arp-scan failed (ensure host network and NET_RAW capability): %w", err)
	}
	return parseOutput(string(output)), nil
}

func parseOutput(output string) []Device {
	items := []Device{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip, err := netip.ParseAddr(fields[0])
		if err != nil || !ip.IsValid() {
			continue
		}
		mac := strings.ToLower(fields[1])
		parts := strings.Split(mac, ":")
		if len(parts) != 6 {
			continue
		}
		valid := true
		for _, p := range parts {
			if len(p) != 2 {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		items = append(items, Device{IPAddress: ip.String(), MACAddress: mac, Vendor: strings.Join(fields[2:], " ")})
	}
	return items
}
