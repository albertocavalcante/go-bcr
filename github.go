package bcr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoGitHubRepo is returned when a module's metadata has no GitHub repository.
var ErrNoGitHubRepo = errors.New("bcr: module has no GitHub repository in metadata")

// ErrNoGitHubDocs is returned when a module's GitHub repo has no docs/ directory.
var ErrNoGitHubDocs = errors.New("bcr: no stardoc files found in GitHub docs/")

// GitHubRepo extracts the GitHub "org/repo" from BCR metadata.
// The metadata.repository field contains entries like "github:bazel-contrib/rules_go".
func (c *Client) GitHubRepo(ctx context.Context, module string) (string, error) {
	meta, err := c.Metadata(ctx, module)
	if err != nil {
		return "", err
	}
	for _, r := range meta.Repository {
		if strings.HasPrefix(r, "github:") {
			return strings.TrimPrefix(r, "github:"), nil
		}
	}
	return "", ErrNoGitHubRepo
}

// FetchGitHubDocs downloads stardoc Markdown files from a module's GitHub repo.
//
// It resolves the GitHub repo from BCR metadata, then searches the docs/
// directory for stardoc-generated .md files. Downloaded files are written to
// a temporary directory. The caller must clean up the directory when done.
//
// This is the fallback path when a module doesn't publish docs_url.
func (c *Client) FetchGitHubDocs(ctx context.Context, module, version string) (string, error) {
	repo, err := c.GitHubRepo(ctx, module)
	if err != nil {
		return "", fmt.Errorf("bcr: FetchGitHubDocs %s: %w", module, err)
	}

	// Use GitHub API to list docs/ directory contents.
	files, err := c.listGitHubDocsFiles(ctx, repo)
	if err != nil {
		return "", fmt.Errorf("bcr: FetchGitHubDocs %s: %w", module, err)
	}

	if len(files) == 0 {
		return "", ErrNoGitHubDocs
	}

	dir, err := os.MkdirTemp("", "bcr-github-docs-*")
	if err != nil {
		return "", fmt.Errorf("bcr: FetchGitHubDocs: %w", err)
	}

	downloaded := 0
	for _, f := range files {
		data, err := c.fetchRaw(ctx, f.DownloadURL)
		if err != nil {
			continue // best-effort: skip files that fail
		}
		target := filepath.Join(dir, filepath.Base(f.Path))
		if err := os.WriteFile(target, data, 0o644); err != nil {
			continue
		}
		downloaded++
	}

	if downloaded == 0 {
		os.RemoveAll(dir)
		return "", ErrNoGitHubDocs
	}

	return dir, nil
}

// githubFile represents a file entry from the GitHub Contents API.
type githubFile struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	DownloadURL string `json:"download_url"`
	Size        int    `json:"size"`
}

// listGitHubDocsFiles lists stardoc Markdown files in a repo's docs/ directory.
// Recursively searches one level of subdirectories.
func (c *Client) listGitHubDocsFiles(ctx context.Context, repo string) ([]githubFile, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/docs", repo)

	entries, err := c.fetchGitHubDir(ctx, apiURL)
	if err != nil {
		return nil, err
	}

	var files []githubFile
	for _, e := range entries {
		if e.Type == "file" && isStardocFile(e.Name) {
			files = append(files, e)
		}
		// Search one level deep (e.g., docs/go/core/)
		if e.Type == "dir" {
			subURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repo, e.Path)
			subEntries, err := c.fetchGitHubDir(ctx, subURL)
			if err != nil {
				continue
			}
			for _, se := range subEntries {
				if se.Type == "file" && isStardocFile(se.Name) {
					files = append(files, se)
				}
				// One more level (docs/go/core/rules.md)
				if se.Type == "dir" {
					deepURL := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", repo, se.Path)
					deepEntries, err := c.fetchGitHubDir(ctx, deepURL)
					if err != nil {
						continue
					}
					for _, de := range deepEntries {
						if de.Type == "file" && isStardocFile(de.Name) {
							files = append(files, de)
						}
					}
				}
			}
		}
	}

	return files, nil
}

func (c *Client) fetchGitHubDir(ctx context.Context, apiURL string) ([]githubFile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // no docs/ directory
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API: HTTP %d for %s", resp.StatusCode, apiURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, err
	}

	var entries []githubFile
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parse GitHub response: %w", err)
	}
	return entries, nil
}

func (c *Client) fetchRaw(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5MB limit per file
}

// isStardocFile checks if a filename looks like stardoc-generated documentation.
func isStardocFile(name string) bool {
	if !strings.HasSuffix(name, ".md") {
		return false
	}
	lower := strings.ToLower(name)
	// Common stardoc patterns
	if strings.HasSuffix(lower, "_doc.md") {
		return true
	}
	if strings.HasSuffix(lower, "-api.md") || strings.HasSuffix(lower, "_api.md") {
		return true
	}
	if lower == "rules.md" || lower == "providers.md" || lower == "aspects.md" || lower == "functions.md" {
		return true
	}
	// Skip common non-stardoc files
	if lower == "readme.md" || lower == "changelog.md" || lower == "contributing.md" {
		return false
	}
	// Accept other .md files in docs/ directory as potential stardoc
	return true
}
