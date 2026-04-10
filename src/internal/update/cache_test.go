package update

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheSaveLoad(t *testing.T) {
	dir := t.TempDir()
	CachePath = func() string { return filepath.Join(dir, "update-check.json") }
	defer func() {
		CachePath = func() string { return "" }
	}()

	entry := CacheEntry{
		CheckedAt:      time.Now(),
		LatestVersion:  "v1.2.0",
		ReleaseURL:     "https://example.com",
		CurrentVersion: "v1.0.0",
	}
	if err := SaveCache(entry); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadCache()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LatestVersion != "v1.2.0" {
		t.Errorf("expected v1.2.0, got %s", loaded.LatestVersion)
	}
	if loaded.CurrentVersion != "v1.0.0" {
		t.Errorf("expected v1.0.0, got %s", loaded.CurrentVersion)
	}
}

func TestLoadCache_Missing(t *testing.T) {
	CachePath = func() string { return filepath.Join(t.TempDir(), "nonexistent.json") }
	defer func() {
		CachePath = func() string { return "" }
	}()

	entry, err := LoadCache()
	if err != nil || entry != nil {
		t.Errorf("expected nil, nil for missing file, got %v, %v", entry, err)
	}
}

func TestCheckLatestCached_HitsCache(t *testing.T) {
	dir := t.TempDir()
	CachePath = func() string { return filepath.Join(dir, "update-check.json") }

	apiCalled := false
	origCheckFn := checkFn
	checkFn = func(ctx context.Context) (*Release, error) {
		apiCalled = true
		return &Release{Version: "v2.0.0", URL: "https://example.com"}, nil
	}
	defer func() {
		checkFn = origCheckFn
		CachePath = func() string { return "" }
	}()

	// First call — should hit API
	rel, err := CheckLatestCached(context.Background(), "v1.0.0", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !apiCalled {
		t.Error("expected API call on first check")
	}
	if rel == nil || rel.Version != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %v", rel)
	}

	// Second call — should use cache
	apiCalled = false
	rel, err = CheckLatestCached(context.Background(), "v1.0.0", 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if apiCalled {
		t.Error("expected cache hit, not API call")
	}
	if rel == nil || rel.Version != "v2.0.0" {
		t.Errorf("expected v2.0.0 from cache, got %v", rel)
	}
}

func TestCheckLatestCached_InvalidatesOnVersionChange(t *testing.T) {
	dir := t.TempDir()
	CachePath = func() string { return filepath.Join(dir, "update-check.json") }

	callCount := 0
	origCheckFn := checkFn
	checkFn = func(ctx context.Context) (*Release, error) {
		callCount++
		return &Release{Version: "v2.0.0", URL: "https://example.com"}, nil
	}
	defer func() {
		checkFn = origCheckFn
		CachePath = func() string { return "" }
	}()

	// Cache with v1.0.0
	CheckLatestCached(context.Background(), "v1.0.0", 24*time.Hour)
	if callCount != 1 {
		t.Fatalf("expected 1 API call, got %d", callCount)
	}

	// Check with v1.1.0 — cache should be invalidated
	CheckLatestCached(context.Background(), "v1.1.0", 24*time.Hour)
	if callCount != 2 {
		t.Errorf("expected 2 API calls (cache invalidated), got %d", callCount)
	}
}
