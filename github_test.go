package bcr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGitHubRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/modules/rules_go/metadata.json" {
			w.Write([]byte(`{
				"versions": ["0.53.0"],
				"homepage": "https://github.com/bazel-contrib/rules_go",
				"repository": ["github:bazel-contrib/rules_go"]
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	repo, err := c.GitHubRepo(context.Background(), "rules_go")
	if err != nil {
		t.Fatal(err)
	}
	if repo != "bazel-contrib/rules_go" {
		t.Errorf("repo = %q, want bazel-contrib/rules_go", repo)
	}
}

func TestGitHubRepo_NoRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"versions": ["1.0.0"], "homepage": "https://example.com"}`))
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	_, err := c.GitHubRepo(context.Background(), "test")
	if err != ErrNoGitHubRepo {
		t.Errorf("expected ErrNoGitHubRepo, got %v", err)
	}
}

func TestFetchGitHubDocs(t *testing.T) {
	// Mock GitHub API + raw file serving
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/modules/rules_go/metadata.json":
			w.Write([]byte(`{
				"versions": ["0.53.0"],
				"repository": ["github:test-org/rules_go"]
			}`))

		case "/api.github.com/repos/test-org/rules_go/contents/docs":
			files := []githubFile{
				{Name: "rules.md", Path: "docs/rules.md", Type: "file",
					DownloadURL: "http://" + r.Host + "/raw/rules.md", Size: 100},
				{Name: "README.md", Path: "docs/README.md", Type: "file",
					DownloadURL: "http://" + r.Host + "/raw/README.md", Size: 50},
			}
			json.NewEncoder(w).Encode(files)

		case "/raw/rules.md":
			w.Write([]byte(`<a id="my_rule"></a>

## my_rule

<pre>my_rule()</pre>

A test rule.
`))

		case "/raw/README.md":
			w.Write([]byte("# Getting Started\n\nThis is a guide.\n"))

		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Override GitHub API URL by intercepting the client's HTTP calls
	c := New(
		WithBaseURL(srv.URL),
		WithHTTPClient(&http.Client{
			Transport: &rewriteTransport{
				base:     http.DefaultTransport,
				rewriteHost: srv.Listener.Addr().String(),
			},
		}),
	)

	dir, err := c.FetchGitHubDocs(context.Background(), "rules_go", "0.53.0")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Check files were downloaded
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	t.Logf("Downloaded files: %v", names)

	if len(entries) == 0 {
		t.Error("expected at least 1 downloaded file")
	}

	// Verify content of stardoc file
	data, err := os.ReadFile(filepath.Join(dir, "rules.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("rules.md is empty")
	}
}

func TestIsStardocFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"rules.md", true},
		{"providers.md", true},
		{"analysis_test_doc.md", true},
		{"bzlmod-api.md", true},
		{"README.md", false},
		{"CHANGELOG.md", false},
		{"CONTRIBUTING.md", false},
		{"guide.md", true},  // accept unknown .md in docs/
		{"rules.txt", false},
		{"BUILD.bazel", false},
	}
	for _, tt := range tests {
		if got := isStardocFile(tt.name); got != tt.want {
			t.Errorf("isStardocFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// rewriteTransport rewrites GitHub API URLs to point to the test server.
type rewriteTransport struct {
	base        http.RoundTripper
	rewriteHost string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "api.github.com" {
		req = req.Clone(req.Context())
		req.URL.Scheme = "http"
		req.URL.Host = t.rewriteHost
		req.URL.Path = "/api.github.com" + req.URL.Path
	}
	if req.URL.Host == "raw.githubusercontent.com" {
		req = req.Clone(req.Context())
		req.URL.Scheme = "http"
		req.URL.Host = t.rewriteHost
		req.URL.Path = "/raw" + req.URL.Path
	}
	return t.base.RoundTrip(req)
}
