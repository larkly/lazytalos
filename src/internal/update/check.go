package update

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

const githubAPIURL = "https://api.github.com/repos/larkly/lazytalos/releases/latest"

// Release holds information about a GitHub release.
type Release struct {
	Version string
	URL     string
}

// CheckLatest fetches the latest release from GitHub.
// On any network or decode error it returns (nil, nil) so callers can ignore
// failures gracefully; the context is honoured for cancellation.
func CheckLatest(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", githubAPIURL, nil)
	if err != nil {
		return nil, nil
	}
	req.Header.Set("User-Agent", "lazytalos")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil
	}
	var body struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, nil
	}
	return &Release{Version: body.TagName, URL: body.HTMLURL}, nil
}

// IsNewer reports whether latest is a higher semver than current.
// Returns false for "dev" or empty current versions.
func IsNewer(latest, current string) bool {
	if current == "dev" || current == "" {
		return false
	}
	lv := parseSemver(strings.TrimPrefix(latest, "v"))
	cv := parseSemver(strings.TrimPrefix(current, "v"))
	for i := range lv {
		if i >= len(cv) {
			return true
		}
		if lv[i] > cv[i] {
			return true
		}
		if lv[i] < cv[i] {
			return false
		}
	}
	return false
}

func parseSemver(s string) []int {
	parts := strings.Split(s, ".")
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, _ := strconv.Atoi(p)
		nums[i] = n
	}
	return nums
}
