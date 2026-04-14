package app

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestApplyPerNode_AllSucceed(t *testing.T) {
	nodes := []string{"a", "b", "c"}
	succeeded, failures := applyPerNode(nodes, func(string) error { return nil })
	if !reflect.DeepEqual(succeeded, nodes) {
		t.Fatalf("expected all nodes to succeed in order, got %v", succeeded)
	}
	if failures != nil {
		t.Fatalf("expected nil failures map, got %v", failures)
	}
}

func TestApplyPerNode_AllFail(t *testing.T) {
	nodes := []string{"a", "b"}
	boom := errors.New("boom")
	succeeded, failures := applyPerNode(nodes, func(string) error { return boom })
	if succeeded != nil {
		t.Fatalf("expected no successes, got %v", succeeded)
	}
	if len(failures) != 2 {
		t.Fatalf("expected 2 failures, got %d: %v", len(failures), failures)
	}
	for _, n := range nodes {
		if failures[n] != boom {
			t.Errorf("expected failures[%q]=boom, got %v", n, failures[n])
		}
	}
}

// Partial failure is the key regression this test guards against: the
// original implementation aborted on the first error and never tried the
// remaining nodes.
func TestApplyPerNode_PartialFailure_ContinuesAfterError(t *testing.T) {
	nodes := []string{"a", "b", "c", "d"}
	boom := errors.New("boom")
	attempts := 0
	succeeded, failures := applyPerNode(nodes, func(node string) error {
		attempts++
		if node == "a" || node == "c" {
			return boom
		}
		return nil
	})
	if attempts != 4 {
		t.Fatalf("expected fn called 4 times (one per node), got %d", attempts)
	}
	expectOK := []string{"b", "d"}
	if !reflect.DeepEqual(succeeded, expectOK) {
		t.Fatalf("expected succeeded=%v, got %v", expectOK, succeeded)
	}
	if len(failures) != 2 || failures["a"] != boom || failures["c"] != boom {
		t.Fatalf("expected failures for a and c, got %v", failures)
	}
}

func TestApplyPerNode_EmptyInput(t *testing.T) {
	calls := 0
	succeeded, failures := applyPerNode(nil, func(string) error {
		calls++
		return nil
	})
	if calls != 0 {
		t.Fatalf("fn must not be called for empty nodes slice, got %d calls", calls)
	}
	if succeeded != nil || failures != nil {
		t.Fatalf("expected nil,nil for empty input, got %v,%v", succeeded, failures)
	}
}

func TestFormatPartialFailure_NilOnEmpty(t *testing.T) {
	if err := formatPartialFailure(nil, nil); err != nil {
		t.Fatalf("expected nil for empty failures, got %v", err)
	}
	if err := formatPartialFailure([]string{"a"}, nil); err != nil {
		t.Fatalf("expected nil when there are successes but no failures, got %v", err)
	}
}

func TestFormatPartialFailure_SortedDeterministic(t *testing.T) {
	failures := map[string]error{
		"node-c": errors.New("cc"),
		"node-a": errors.New("aa"),
		"node-b": errors.New("bb"),
	}
	err := formatPartialFailure(nil, failures)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	msg := err.Error()
	idxA := strings.Index(msg, "node-a")
	idxB := strings.Index(msg, "node-b")
	idxC := strings.Index(msg, "node-c")
	if idxA < 0 || idxB < 0 || idxC < 0 {
		t.Fatalf("all node names must appear, got: %q", msg)
	}
	if !(idxA < idxB && idxB < idxC) {
		t.Fatalf("expected sorted order a<b<c, got: %q", msg)
	}
}

func TestFormatPartialFailure_IncludesSuccessesWhenMixed(t *testing.T) {
	err := formatPartialFailure([]string{"ok1", "ok2"}, map[string]error{
		"bad1": errors.New("down"),
	})
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bad1") || !strings.Contains(msg, "down") {
		t.Errorf("expected failure detail in message, got %q", msg)
	}
	if !strings.Contains(msg, "ok1") || !strings.Contains(msg, "ok2") {
		t.Errorf("expected successes listed in message, got %q", msg)
	}
}
