package sensors

import "testing"

const sampleOutput = `{
  "coretemp-isa-0000": {
    "Adapter": "ISA adapter",
    "Package id 0": { "temp1_input": 45.0, "temp1_max": 80.0, "temp1_crit": 100.0 },
    "Core 0": { "temp2_input": 43.0 },
    "Core 1": { "temp3_input": 47.5 }
  },
  "nvme-pci-0400": {
    "Adapter": "PCI adapter",
    "Composite": { "temp1_input": 38.9, "temp1_alarm": 0.0 }
  },
  "acpitz-acpi-0": {
    "Adapter": "ACPI interface"
  }
}`

func TestParseSensorsJSON(t *testing.T) {
	temps, err := ParseSensorsJSON([]byte(sampleOutput))
	if err != nil {
		t.Fatal(err)
	}
	if len(temps) != 2 {
		t.Fatalf("chips = %v", SortedChips(temps))
	}
	// 芯片取所有 temp*_input 的最大值；temp1_max/crit 阈值不算温度读数
	if temps["coretemp-isa-0000"] != 47.5 {
		t.Errorf("coretemp = %v", temps["coretemp-isa-0000"])
	}
	if temps["nvme-pci-0400"] != 38.9 {
		t.Errorf("nvme = %v", temps["nvme-pci-0400"])
	}
}

// 用户 PVE 宿主机（xkl）的真实 `sensors -j` 输出：验证 NVMe 的
// 65261.85 哨兵 max 值与 -273.15 min 值不会被误当成温度读数。
const realWorldOutput = `{"coretemp-isa-0000":{"Adapter":"ISA adapter","Package id 0":{"temp1_input":65.000000,"temp1_max":105.000000,"temp1_crit":105.000000,"temp1_crit_alarm":0.000000},"Core 0":{"temp2_input":65.000000,"temp2_max":105.000000,"temp2_crit":105.000000,"temp2_crit_alarm":0.000000},"Core 1":{"temp3_input":65.000000,"temp3_max":105.000000,"temp3_crit":105.000000,"temp3_crit_alarm":0.000000},"Core 2":{"temp4_input":65.000000,"temp4_max":105.000000,"temp4_crit":105.000000,"temp4_crit_alarm":0.000000},"Core 3":{"temp5_input":65.000000,"temp5_max":105.000000,"temp5_crit":105.000000,"temp5_crit_alarm":0.000000}},"acpitz-acpi-0":{"Adapter":"ACPI interface","temp1":{"temp1_input":27.800000}},"nvme-pci-0100":{"Adapter":"PCI adapter","Composite":{"temp1_input":46.850000,"temp1_max":81.850000,"temp1_min":-273.150000,"temp1_crit":84.850000,"temp1_alarm":0.000000},"Sensor 1":{"temp2_input":46.850000,"temp2_max":65261.850000,"temp2_min":-273.150000},"Sensor 2":{"temp3_input":48.850000,"temp3_max":65261.850000,"temp3_min":-273.150000}}}`

func TestParseSensorsJSONRealWorld(t *testing.T) {
	temps, err := ParseSensorsJSON([]byte(realWorldOutput))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{
		"coretemp-isa-0000": 65.0,
		"acpitz-acpi-0":     27.8,
		"nvme-pci-0100":     48.85,
	}
	if len(temps) != len(want) {
		t.Fatalf("chips = %v", SortedChips(temps))
	}
	for chip, value := range want {
		if temps[chip] != value {
			t.Errorf("%s = %v, want %v", chip, temps[chip], value)
		}
	}
}

func TestParseSensorsJSONNoTemps(t *testing.T) {
	if _, err := ParseSensorsJSON([]byte(`{"chip": {"Adapter": "x"}}`)); err == nil {
		t.Fatal("want error when no temperature inputs")
	}
}

func TestParseSensorsJSONInvalid(t *testing.T) {
	if _, err := ParseSensorsJSON([]byte(`not json`)); err == nil {
		t.Fatal("want error for invalid json")
	}
}
