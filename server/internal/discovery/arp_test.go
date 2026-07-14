package discovery

import "testing"

func TestParseOutput(t *testing.T) {
	output := "Interface: eth0\n192.168.1.1\tAA:BB:CC:DD:EE:FF\tExample Networks\n192.168.1.20  00:11:22:33:44:55  NAS Vendor\n2 packets received\n"
	items := parseOutput(output)
	if len(items) != 2 {
		t.Fatalf("items = %#v", items)
	}
	if items[0].IPAddress != "192.168.1.1" || items[0].MACAddress != "aa:bb:cc:dd:ee:ff" || items[0].Vendor != "Example Networks" {
		t.Errorf("first = %#v", items[0])
	}
	if items[1].Vendor != "NAS Vendor" {
		t.Errorf("second = %#v", items[1])
	}
}

func TestUniqueByMAC(t *testing.T) {
	items := uniqueByMAC([]Device{{IPAddress: "192.168.1.1", MACAddress: "aa:bb:cc:dd:ee:ff"}, {IPAddress: "192.168.1.2", MACAddress: "aa:bb:cc:dd:ee:ff"}, {IPAddress: "192.168.1.3", MACAddress: "00:11:22:33:44:55"}})
	if len(items) != 2 || items[0].IPAddress != "192.168.1.1" {
		t.Fatalf("items = %#v", items)
	}
}
