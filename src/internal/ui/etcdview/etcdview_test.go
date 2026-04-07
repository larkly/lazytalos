package etcdview

import (
	"testing"
	"time"

	"charm.land/bubbletea/v2"
	"github.com/larkly/lazytalos/internal/etcd"
	"github.com/larkly/lazytalos/internal/shared"
)

func TestNew(t *testing.T) {
	m := New(nil, 30*time.Second)
	if m.subTab != subTabMembers {
		t.Errorf("expected initial subTab=%d, got %d", subTabMembers, m.subTab)
	}
	if !m.loading {
		t.Error("expected loading=true on New()")
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", m.cursor)
	}
}

func TestSetSize(t *testing.T) {
	m := New(nil, 30*time.Second)
	m.SetSize(120, 40)
	if m.width != 120 || m.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", m.width, m.height)
	}
}

func TestSubTabCycle(t *testing.T) {
	m := New(nil, 30*time.Second)
	m.loading = false

	// ] advances to Config.
	m, _ = m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	if m.subTab != subTabConfig {
		t.Errorf("expected subTab=%d after ], got %d", subTabConfig, m.subTab)
	}

	// ] wraps around back to Members.
	m, _ = m.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	if m.subTab != subTabMembers {
		t.Errorf("expected subTab=%d after second ], got %d", subTabMembers, m.subTab)
	}

	// [ goes back to Config.
	m, _ = m.Update(tea.KeyPressMsg{Code: '[', Text: "["})
	if m.subTab != subTabConfig {
		t.Errorf("expected subTab=%d after [, got %d", subTabConfig, m.subTab)
	}
}

func TestHints(t *testing.T) {
	m := New(nil, 30*time.Second)
	m.loading = false

	hints := m.Hints()
	if hints == "" {
		t.Error("expected non-empty hints for Members sub-tab")
	}

	// Switch to Config.
	m.subTab = subTabConfig
	hints = m.Hints()
	if hints == "" {
		t.Error("expected non-empty hints for Config sub-tab")
	}
}

func TestCtrlMEmitsMsg(t *testing.T) {
	m := New(nil, 30*time.Second)
	m.loading = false
	m.members = []etcd.Member{
		{
			NodeHostname: "tnn3-demo-cp-1",
			MemberID:     0xabc123def456789a,
			Hostname:     "tnn3-demo-cp-1",
			PeerAddrs:    []string{"10.244.120.1:2380"},
			ClientAddrs:  []string{"10.244.120.1:2379"},
		},
		{
			NodeHostname: "tnn3-demo-cp-2",
			MemberID:     0xdef789abc123456b,
			Hostname:     "tnn3-demo-cp-2",
			PeerAddrs:    []string{"10.244.120.2:2380"},
			ClientAddrs:  []string{"10.244.120.2:2379"},
		},
	}
	m.cursor = 0

	// Ctrl+M on cursor=0 in Members sub-tab should emit EtcdMemberRemoveRequestMsg.
	ctrlM := tea.KeyPressMsg{Code: 'm', Mod: tea.ModCtrl}
	_, cmd := m.Update(ctrlM)
	if cmd == nil {
		t.Fatal("expected a command after ctrl+m on a loaded member")
	}

	result := cmd()
	msg, ok := result.(shared.EtcdMemberRemoveRequestMsg)
	if !ok {
		t.Fatalf("expected EtcdMemberRemoveRequestMsg, got %T", result)
	}
	if msg.Node != "tnn3-demo-cp-1" {
		t.Errorf("expected Node=tnn3-demo-cp-1, got %q", msg.Node)
	}
	if msg.MemberID != 0xabc123def456789a {
		t.Errorf("expected MemberID=0xabc123def456789a, got 0x%x", msg.MemberID)
	}
}

func TestCtrlMIgnoredOnConfigSubTab(t *testing.T) {
	m := New(nil, 30*time.Second)
	m.loading = false
	m.subTab = subTabConfig
	m.members = []etcd.Member{
		{NodeHostname: "node-1", MemberID: 1, Hostname: "node-1"},
	}
	m.cursor = 0

	ctrlM := tea.KeyPressMsg{Code: 'm', Mod: tea.ModCtrl}
	_, cmd := m.Update(ctrlM)
	if cmd != nil {
		t.Error("expected no command when ctrl+m pressed on Config sub-tab")
	}
}

func TestCursorMovement(t *testing.T) {
	m := New(nil, 30*time.Second)
	m.loading = false
	m.members = []etcd.Member{
		{Hostname: "node-1", MemberID: 1},
		{Hostname: "node-2", MemberID: 2},
		{Hostname: "node-3", MemberID: 3},
	}
	m.cursor = 0

	// Down moves cursor.
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after Down, got %d", m.cursor)
	}

	// Down again.
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 after second Down, got %d", m.cursor)
	}

	// Down at end — should not exceed bounds.
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if m.cursor != 2 {
		t.Errorf("expected cursor stays at 2 at end, got %d", m.cursor)
	}

	// Up moves back.
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after Up, got %d", m.cursor)
	}
}
