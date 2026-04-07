package etcdview

import (
	"testing"

	"charm.land/bubbletea/v2"
)

func testMembers() []etcdMemberRow {
	return []etcdMemberRow{
		{MemberID: 0xabc123, MemberHex: "abc123", Hostname: "cp-1", PeerURLs: []string{"https://10.0.0.1:2380"}, ClientURLs: []string{"https://10.0.0.1:2379"}},
		{MemberID: 0xdef456, MemberHex: "def456", Hostname: "cp-2", PeerURLs: []string{"https://10.0.0.2:2380"}, ClientURLs: []string{"https://10.0.0.2:2379"}},
		{MemberID: 0x789abc, MemberHex: "789abc", Hostname: "cp-3", PeerURLs: []string{"https://10.0.0.3:2380"}, ClientURLs: []string{"https://10.0.0.3:2379"}, IsLearner: true},
	}
}

func TestNew(t *testing.T) {
	m := New(nil, 0)
	if !m.loading {
		t.Error("expected loading=true on new model")
	}
	if m.cursor != 0 {
		t.Error("expected cursor=0")
	}
}

func TestCursorNavigation(t *testing.T) {
	m := Model{members: testMembers(), width: 120, height: 24}

	// Down
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}

	// Down again
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.cursor)
	}

	// Down at bottom stays
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (at bottom)", m.cursor)
	}

	// Up
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
}

func TestRemoveMemberEmitsRequest(t *testing.T) {
	m := Model{members: testMembers(), width: 120, height: 24}
	m.cursor = 1 // select cp-2

	_, cmd := m.Update(tea.KeyPressMsg{Code: 109, Mod: tea.ModCtrl}) // ctrl+m
	if cmd == nil {
		t.Fatal("expected command for remove member")
	}
}

func TestView_Loading(t *testing.T) {
	m := Model{loading: true, width: 80, height: 24}
	v := m.View()
	if v == "" {
		t.Error("expected non-empty loading view")
	}
}

func TestView_WithMembers(t *testing.T) {
	m := Model{members: testMembers(), width: 120, height: 24}
	v := m.View()
	if v == "" {
		t.Error("expected non-empty view")
	}
	if !contains(v, "abc123") {
		t.Error("expected member ID in view")
	}
	if !contains(v, "cp-1") {
		t.Error("expected hostname in view")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
