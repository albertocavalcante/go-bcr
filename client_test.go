package bcr

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		c := New()
		if c.baseURL != DefaultBaseURL {
			t.Errorf("baseURL = %q, want %q", c.baseURL, DefaultBaseURL)
		}
		if c.cache != nil {
			t.Error("cache should be nil by default")
		}
	})

	t.Run("with options", func(t *testing.T) {
		customClient := &http.Client{Timeout: 5 * time.Second}
		c := New(
			WithBaseURL("https://example.com"),
			WithHTTPClient(customClient),
			WithUserAgent("test/1.0"),
			WithCacheDir(t.TempDir()),
		)
		if c.baseURL != "https://example.com" {
			t.Errorf("baseURL = %q, want %q", c.baseURL, "https://example.com")
		}
		if c.http != customClient {
			t.Error("http client not set correctly")
		}
		if c.userAgent != "test/1.0" {
			t.Errorf("userAgent = %q, want %q", c.userAgent, "test/1.0")
		}
		if c.cache == nil {
			t.Error("cache should not be nil")
		}
	})
}

func TestMetadata(t *testing.T) {
	meta := &Metadata{
		Versions:       []string{"1.0.0", "1.1.0", "2.0.0"},
		YankedVersions: map[string]string{"1.0.0": "security issue"},
		Maintainers: []Maintainer{
			{Name: "Alice", GitHub: "alice"},
		},
		Homepage:   "https://example.com",
		Repository: []string{"github:example/repo"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/modules/testmod/metadata.json" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(meta)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		got, err := c.Metadata(ctx, "testmod")
		if err != nil {
			t.Fatalf("Metadata() error = %v", err)
		}
		if len(got.Versions) != 3 {
			t.Errorf("got %d versions, want 3", len(got.Versions))
		}
		if got.Homepage != "https://example.com" {
			t.Errorf("Homepage = %q, want %q", got.Homepage, "https://example.com")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := c.Metadata(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}

		var nf *NotFoundError
		if !errors.As(err, &nf) {
			t.Errorf("error should be *NotFoundError")
		} else if nf.Module != "nonexistent" {
			t.Errorf("Module = %q, want %q", nf.Module, "nonexistent")
		}
	})
}

func TestSource(t *testing.T) {
	src := &Source{
		URL:         "https://example.com/archive.zip",
		Integrity:   "sha256-abc123",
		StripPrefix: "prefix",
		Patches: map[string]string{
			"fix.patch": "sha256-xyz789",
		},
		PatchStrip: 1,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/modules/testmod/1.0.0/source.json" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(src)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		got, err := c.Source(ctx, "testmod", "1.0.0")
		if err != nil {
			t.Fatalf("Source() error = %v", err)
		}
		if got.URL != src.URL {
			t.Errorf("URL = %q, want %q", got.URL, src.URL)
		}
		if got.PatchStrip != 1 {
			t.Errorf("PatchStrip = %d, want 1", got.PatchStrip)
		}
		if len(got.Patches) != 1 {
			t.Errorf("got %d patches, want 1", len(got.Patches))
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := c.Source(ctx, "testmod", "9.9.9")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestModuleFile(t *testing.T) {
	moduleContent := []byte(`module(name = "testmod", version = "1.0.0")`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/modules/testmod/1.0.0/MODULE.bazel" {
			w.Write(moduleContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	ctx := context.Background()

	got, err := c.ModuleFile(ctx, "testmod", "1.0.0")
	if err != nil {
		t.Fatalf("ModuleFile() error = %v", err)
	}
	if string(got) != string(moduleContent) {
		t.Errorf("content = %q, want %q", got, moduleContent)
	}
}

func TestLatest(t *testing.T) {
	meta := &Metadata{
		Versions:       []string{"1.0.0", "1.1.0", "2.0.0"},
		YankedVersions: map[string]string{"2.0.0": "broken"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/modules/testmod/metadata.json" {
			json.NewEncoder(w).Encode(meta)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	ctx := context.Background()

	got, err := c.Latest(ctx, "testmod")
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if got != "1.1.0" {
		t.Errorf("Latest() = %q, want %q (skipping yanked 2.0.0)", got, "1.1.0")
	}
}

func TestVersions(t *testing.T) {
	meta := &Metadata{
		Versions: []string{"1.0.0", "1.1.0", "2.0.0"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/modules/testmod/metadata.json" {
			json.NewEncoder(w).Encode(meta)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	ctx := context.Background()

	var versions []string
	for v, err := range c.Versions(ctx, "testmod") {
		if err != nil {
			t.Fatalf("Versions() yielded error: %v", err)
		}
		versions = append(versions, v)
	}

	if len(versions) != 3 {
		t.Errorf("got %d versions, want 3", len(versions))
	}
}

func TestExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/modules/exists/metadata.json" {
			json.NewEncoder(w).Encode(&Metadata{Versions: []string{"1.0.0"}})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	ctx := context.Background()

	t.Run("exists", func(t *testing.T) {
		ok, err := c.Exists(ctx, "exists")
		if err != nil {
			t.Fatalf("Exists() error = %v", err)
		}
		if !ok {
			t.Error("Exists() = false, want true")
		}
	})

	t.Run("not exists", func(t *testing.T) {
		ok, err := c.Exists(ctx, "notexists")
		if err != nil {
			t.Fatalf("Exists() error = %v", err)
		}
		if ok {
			t.Error("Exists() = true, want false")
		}
	})
}

func TestMetadataHelpers(t *testing.T) {
	meta := &Metadata{
		Versions: []string{"1.0.0", "1.1.0", "2.0.0"},
		YankedVersions: map[string]string{
			"2.0.0": "security vulnerability",
		},
	}

	t.Run("IsYanked", func(t *testing.T) {
		if !meta.IsYanked("2.0.0") {
			t.Error("IsYanked(2.0.0) = false, want true")
		}
		if meta.IsYanked("1.0.0") {
			t.Error("IsYanked(1.0.0) = true, want false")
		}
	})

	t.Run("YankReason", func(t *testing.T) {
		reason := meta.YankReason("2.0.0")
		if reason != "security vulnerability" {
			t.Errorf("YankReason() = %q, want %q", reason, "security vulnerability")
		}
		if reason := meta.YankReason("1.0.0"); reason != "" {
			t.Errorf("YankReason(1.0.0) = %q, want empty", reason)
		}
	})

	t.Run("Latest", func(t *testing.T) {
		latest := meta.Latest()
		if latest != "1.1.0" {
			t.Errorf("Latest() = %q, want %q", latest, "1.1.0")
		}
	})

	t.Run("HasVersion", func(t *testing.T) {
		if !meta.HasVersion("1.1.0") {
			t.Error("HasVersion(1.1.0) = false, want true")
		}
		if meta.HasVersion("9.9.9") {
			t.Error("HasVersion(9.9.9) = true, want false")
		}
	})

	t.Run("nil safety", func(t *testing.T) {
		var nilMeta *Metadata
		if nilMeta.IsYanked("1.0.0") {
			t.Error("nil.IsYanked() should return false")
		}
		if nilMeta.Latest() != "" {
			t.Error("nil.Latest() should return empty")
		}
		if nilMeta.HasVersion("1.0.0") {
			t.Error("nil.HasVersion() should return false")
		}
	})
}

func TestSourceType(t *testing.T) {
	t.Run("empty defaults to archive", func(t *testing.T) {
		s := &Source{}
		if s.SourceType() != "archive" {
			t.Errorf("SourceType() = %q, want %q", s.SourceType(), "archive")
		}
	})

	t.Run("explicit type", func(t *testing.T) {
		s := &Source{Type: "git_repository"}
		if s.SourceType() != "git_repository" {
			t.Errorf("SourceType() = %q, want %q", s.SourceType(), "git_repository")
		}
	})

	t.Run("nil safety", func(t *testing.T) {
		var s *Source
		if s.SourceType() != "archive" {
			t.Errorf("nil.SourceType() = %q, want %q", s.SourceType(), "archive")
		}
	})
}

func TestCache(t *testing.T) {
	cacheDir := t.TempDir()

	meta := &Metadata{Versions: []string{"1.0.0"}}
	requestCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		json.NewEncoder(w).Encode(meta)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL), WithCacheDir(cacheDir))
	ctx := context.Background()

	// First request
	_, err := c.Metadata(ctx, "cached")
	if err != nil {
		t.Fatalf("first Metadata() error = %v", err)
	}
	if requestCount != 1 {
		t.Errorf("requestCount = %d, want 1", requestCount)
	}

	// Second request should use cache
	_, err = c.Metadata(ctx, "cached")
	if err != nil {
		t.Fatalf("second Metadata() error = %v", err)
	}
	if requestCount != 1 {
		t.Errorf("requestCount = %d, want 1 (should use cache)", requestCount)
	}

	// Verify cache file exists
	cachePath := filepath.Join(cacheDir, "modules", "cached", "metadata.json")
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Errorf("cache file does not exist at %s", cachePath)
	}
}

func TestErrors(t *testing.T) {
	t.Run("NotFoundError", func(t *testing.T) {
		err := &NotFoundError{Module: "foo", Version: "1.0.0"}

		// Check error message
		msg := err.Error()
		if msg != `bcr: module "foo" version "1.0.0" not found` {
			t.Errorf("Error() = %q", msg)
		}

		// Check Is
		if !errors.Is(err, ErrNotFound) {
			t.Error("errors.Is(err, ErrNotFound) = false")
		}

		// Module only
		err2 := &NotFoundError{Module: "bar"}
		if err2.Error() != `bcr: module "bar" not found` {
			t.Errorf("Error() = %q", err2.Error())
		}
	})

	t.Run("RequestError", func(t *testing.T) {
		err := &RequestError{URL: "https://example.com", StatusCode: 500}
		if msg := err.Error(); msg != "bcr: request to https://example.com failed with status 500" {
			t.Errorf("Error() = %q", msg)
		}

		err2 := &RequestError{URL: "https://example.com", Err: errors.New("timeout")}
		if msg := err2.Error(); msg != "bcr: request to https://example.com failed: timeout" {
			t.Errorf("Error() = %q", msg)
		}

		// Unwrap
		if errors.Unwrap(err2).Error() != "timeout" {
			t.Error("Unwrap() should return underlying error")
		}
	})
}

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(&Metadata{})
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := c.Metadata(ctx, "test")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
