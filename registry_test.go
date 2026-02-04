package bcr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRegistryInterface verifies that Client implements Registry.
func TestRegistryInterface(t *testing.T) {
	// This test will fail to compile if Client doesn't implement Registry
	var _ Registry = (*Client)(nil)
}

// TestRegistryMethods tests the Registry interface methods through Client.
func TestRegistryMethods(t *testing.T) {
	meta := &Metadata{
		Versions:   []string{"1.0.0", "2.0.0"},
		Homepage:   "https://example.com",
		Maintainers: []Maintainer{{Name: "Test"}},
	}
	src := &Source{
		URL:       "https://example.com/archive.zip",
		Integrity: "sha256-abc123",
	}
	moduleContent := []byte(`module(name = "testmod", version = "1.0.0")`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/modules/testmod/metadata.json":
			json.NewEncoder(w).Encode(meta)
		case "/modules/testmod/1.0.0/source.json":
			json.NewEncoder(w).Encode(src)
		case "/modules/testmod/1.0.0/MODULE.bazel":
			w.Write(moduleContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Use Registry interface, not concrete Client
	var reg Registry = New(WithBaseURL(srv.URL))
	ctx := context.Background()

	t.Run("Metadata via interface", func(t *testing.T) {
		got, err := reg.Metadata(ctx, "testmod")
		if err != nil {
			t.Fatalf("Metadata() error = %v", err)
		}
		if len(got.Versions) != 2 {
			t.Errorf("got %d versions, want 2", len(got.Versions))
		}
	})

	t.Run("Source via interface", func(t *testing.T) {
		got, err := reg.Source(ctx, "testmod", "1.0.0")
		if err != nil {
			t.Fatalf("Source() error = %v", err)
		}
		if got.URL != src.URL {
			t.Errorf("URL = %q, want %q", got.URL, src.URL)
		}
	})

	t.Run("ModuleFile via interface", func(t *testing.T) {
		got, err := reg.ModuleFile(ctx, "testmod", "1.0.0")
		if err != nil {
			t.Fatalf("ModuleFile() error = %v", err)
		}
		if string(got) != string(moduleContent) {
			t.Errorf("content = %q, want %q", got, moduleContent)
		}
	})
}
