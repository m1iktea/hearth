package store

import (
	"testing"
	"time"
)

func newTestCheck(t *testing.T, s *InventoryStore, target string) CheckWithDevice {
	t.Helper()
	d, err := s.CreateDevice(Device{Name: "NAS", Kind: "nas", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.CreateCheck(HealthCheck{DeviceID: d.ID, Name: "ping", Type: "ping", Target: target, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return CheckWithDevice{HealthCheck: c, DeviceName: d.Name, DeviceIP: "10.0.0.9"}
}

// 只记状态迁移：首次确认状态写入；连续同状态不重复写；翻转写入并记录 down 原因。
func TestRecordHealthTransitionOnlyOnChange(t *testing.T) {
	s := openTestInventory(t)
	c := newTestCheck(t, s, "10.0.0.9")
	base := time.Now().UTC().Add(-time.Hour)

	wrote, err := s.RecordHealthTransition(c, "online", "正常", 12, base)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("首次确认状态应写入迁移事件")
	}

	wrote, err = s.RecordHealthTransition(c, "online", "正常", 15, base.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if wrote {
		t.Fatal("连续同状态不应重复写入")
	}

	wrote, err = s.RecordHealthTransition(c, "offline", "connection refused", 0, base.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("状态翻转应写入迁移事件")
	}

	got, err := s.QueryHealthTransitions(c.ID, base.Add(-time.Minute), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("迁移事件数应为 2，得到 %d", len(got))
	}
	if got[0].Status != "online" || got[1].Status != "offline" {
		t.Fatalf("升序/状态错误: %+v", got)
	}
	if got[1].Reason != "connection refused" {
		t.Fatalf("down 原因未记录: %q", got[1].Reason)
	}
	if got[1].CheckType != "ping" || got[1].Target != "10.0.0.9" {
		t.Fatalf("target/check_type 未携带: %+v", got[1])
	}
	if got[0].LatencyMS != 12 {
		t.Fatalf("延迟未记录: %d", got[0].LatencyMS)
	}
}

// 目标未设置 target 时回落到设备 IP。
func TestRecordHealthTransitionFallsBackToDeviceIP(t *testing.T) {
	s := openTestInventory(t)
	c := newTestCheck(t, s, "") // 空 target
	if _, err := s.RecordHealthTransition(c, "offline", "timeout", 0, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	got, err := s.QueryHealthTransitions(c.ID, time.Now().UTC().Add(-time.Hour), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Target != "10.0.0.9" {
		t.Fatalf("应回落到设备 IP: %+v", got)
	}
}

// since 过滤：返回窗口内的迁移 + 窗口前最近一条锚点（决定窗口起点状态），升序。
func TestQueryHealthTransitionsSinceFilter(t *testing.T) {
	s := openTestInventory(t)
	c := newTestCheck(t, s, "10.0.0.9")
	now := time.Now().UTC()
	if _, err := s.RecordHealthTransition(c, "online", "", 1, now.Add(-3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordHealthTransition(c, "offline", "down", 0, now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordHealthTransition(c, "online", "", 1, now.Add(-30*time.Minute)); err != nil {
		t.Fatal(err)
	}

	// 窗口 = 最近 1h：窗口内只有 -30min 的 online；但需带上窗口前最近的 offline(-2h) 作锚点。
	got, err := s.QueryHealthTransitions(c.ID, now.Add(-time.Hour), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("应返回锚点 + 窗口内迁移共 2 条，得到 %d: %+v", len(got), got)
	}
	if got[0].Status != "offline" || got[1].Status != "online" {
		t.Fatalf("锚点应在最前且为 offline，升序错误: %+v", got)
	}
}

// 长期稳定在线（最近一次迁移远早于窗口）时，锚点保证时间线不会整段显示为无数据。
func TestQueryHealthTransitionsAnchorWhenStable(t *testing.T) {
	s := openTestInventory(t)
	c := newTestCheck(t, s, "10.0.0.9")
	now := time.Now().UTC()
	// 唯一一次迁移发生在 3h 前，之后一直在线，窗口内无任何迁移。
	if _, err := s.RecordHealthTransition(c, "online", "", 1, now.Add(-3*time.Hour)); err != nil {
		t.Fatal(err)
	}

	got, err := s.QueryHealthTransitions(c.ID, now.Add(-time.Hour), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Status != "online" {
		t.Fatalf("应返回锚点 online 一条以还原窗口初始状态: %+v", got)
	}
}

// 保留清理：过期迁移事件随 PruneBefore 一并删除。
func TestPruneRemovesHealthTransitions(t *testing.T) {
	s := openTestInventory(t)
	c := newTestCheck(t, s, "10.0.0.9")
	now := time.Now().UTC()
	if _, err := s.RecordHealthTransition(c, "offline", "down", 0, now.Add(-40*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordHealthTransition(c, "online", "", 1, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PruneBefore(now.Add(-30*24*time.Hour), now.Add(-30*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := s.QueryHealthTransitions(c.ID, now.Add(-100*24*time.Hour), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Status != "online" {
		t.Fatalf("过期迁移事件应被清理: %+v", got)
	}
}
