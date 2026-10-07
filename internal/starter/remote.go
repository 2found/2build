package starter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Client keeps endpoints injectable for release-service tests, not project config.
type Client struct {
	HTTP *http.Client
	API  string
	Raw  string
}

func NewClient() Client {
	return Client{HTTP: &http.Client{Timeout: 5 * time.Second}, API: "https://api.github.com", Raw: "https://raw.githubusercontent.com"}
}

type Release struct {
	Catalog  Catalog
	Revision string
}

var commitPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

func (c Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "2build-starters")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("starter release service returned HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 1<<20 {
		return nil, fmt.Errorf("starter release response exceeds 1 MiB")
	}
	return b, nil
}

// Fetch resolves a published stable release to an immutable commit before
// reading its catalog. Empty version selects GitHub's latest stable release.
func (c Client) Fetch(ctx context.Context, version string) (Release, error) {
	var result Release
	endpoint := c.API + "/repos/" + Source + "/releases/latest"
	if version != "" {
		if !ValidVersion(version) {
			return result, fmt.Errorf("release must be a stable X.Y.Z version")
		}
		endpoint = c.API + "/repos/" + Source + "/releases/tags/v" + version
	}
	b, err := c.get(ctx, endpoint)
	if err != nil {
		return result, err
	}
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(b, &release); err != nil {
		return result, err
	}
	resolvedVersion := strings.TrimPrefix(release.Tag, "v")
	if release.Draft || release.Prerelease || release.Tag != "v"+resolvedVersion || !ValidVersion(resolvedVersion) || (version != "" && version != resolvedVersion) {
		return result, fmt.Errorf("starter release is not the requested stable vX.Y.Z tag")
	}
	b, err = c.get(ctx, c.API+"/repos/"+Source+"/commits/"+release.Tag)
	if err != nil {
		return result, err
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(b, &commit); err != nil || !commitPattern.MatchString(commit.SHA) {
		return result, fmt.Errorf("starter release has an invalid commit")
	}
	b, err = c.get(ctx, c.Raw+"/"+Source+"/"+commit.SHA+"/catalog.json")
	if err != nil {
		return result, err
	}
	result.Catalog, err = ParseCatalog(b)
	if err != nil {
		return result, err
	}
	if result.Catalog.Version != resolvedVersion {
		return result, fmt.Errorf("starter release tag and catalog version disagree")
	}
	result.Revision = commit.SHA
	return result, nil
}

// Checkout verifies the downloaded tag still points to the probed commit.
func Checkout(ctx context.Context, release Release) (string, func(), error) {
	dir, err := os.MkdirTemp("", "bbs-starter-source-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	root := filepath.Join(dir, "source")
	command := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", "v"+release.Catalog.Version, "https://github.com/"+Source+".git", root)
	if output, err := command.CombinedOutput(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("download starter release: %w: %s", err, strings.TrimSpace(string(output)))
	}
	command = exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD")
	output, err := command.Output()
	if err != nil || strings.TrimSpace(string(output)) != release.Revision {
		cleanup()
		return "", nil, fmt.Errorf("starter release changed while downloading; retry the release check")
	}
	return root, cleanup, nil
}
