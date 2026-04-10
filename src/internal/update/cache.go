package update

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/larkly/lazytalos/internal/shared"
)

// CacheEntry holds the result of a previous update check.
type CacheEntry struct {
	CheckedAt      time.Time `json:"checked_at"`
	LatestVersion  string    `json:"latest_version"`
	ReleaseURL     string    `json:"release_url"`
	CurrentVersion string    `json:"current_version"`
}

// CachePath returns the path to the update-check cache file.
// It is a variable so tests can override it.
var CachePath = func() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "lazytalos", "update-check.json")
}

// LoadCache reads the cached update-check result from disk.
func LoadCache() (*CacheEntry, error) {
	path := CachePath()
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// SaveCache writes the update-check result to disk.
func SaveCache(entry CacheEntry) error {
	path := CachePath()
	if path == "" {
		return errors.New("cannot determine cache directory")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// checkFn is the function used to query for the latest version.
// It is a variable so tests can override it.
var checkFn = func(ctx context.Context) (*Release, error) {
	return CheckLatest(ctx)
}

// CheckLatestCached wraps CheckLatest with a disk cache to avoid
// hitting the GitHub API on every launch. The API is only queried
// if the cache is missing, expired (older than ttl), or was written
// for a different binary version.
func CheckLatestCached(ctx context.Context, currentVersion string, ttl time.Duration) (*Release, error) {
	shared.Debugf("[update] CheckLatestCached: currentVersion=%s ttl=%s", currentVersion, ttl)
	cache, _ := LoadCache()
	if cache != nil && cache.CurrentVersion == currentVersion && time.Since(cache.CheckedAt) < ttl {
		shared.Debugf("[update] CheckLatestCached: cache hit (age=%s)", time.Since(cache.CheckedAt))
		if cache.LatestVersion == "" {
			return nil, nil
		}
		return &Release{Version: cache.LatestVersion, URL: cache.ReleaseURL}, nil
	}

	shared.Debugf("[update] CheckLatestCached: cache miss, querying API")
	rel, err := checkFn(ctx)
	if err != nil {
		return nil, err
	}

	entry := CacheEntry{
		CheckedAt:      time.Now(),
		CurrentVersion: currentVersion,
	}
	if rel != nil {
		entry.LatestVersion = rel.Version
		entry.ReleaseURL = rel.URL
	}
	_ = SaveCache(entry)

	return rel, nil
}
