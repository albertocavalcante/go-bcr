package bcr

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFetchDocs_NoDocs(t *testing.T) {
	// Set up a test server that returns source.json without docs_url.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/modules/test_module/1.0.0/source.json" {
			w.Write([]byte(`{"url": "https://example.com/test.tar.gz", "integrity": "sha256-abc"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	_, err := c.FetchDocs(context.Background(), "test_module", "1.0.0", "")
	if err != ErrNoDocs {
		t.Errorf("expected ErrNoDocs, got %v", err)
	}
}

func TestFetchDocs_WithDocs(t *testing.T) {
	// Create a test docs.tar.gz in memory.
	docsArchive := createTestDocsArchive(t, map[string]string{
		"rules.md":     "## my_rule\n\nA test rule.\n",
		"providers.md": "## MyProvider\n\nA test provider.\n",
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/modules/test_module/1.0.0/source.json":
			w.Write([]byte(`{"url": "https://example.com/test.tar.gz", "docs_url": "` + "http://" + r.Host + `/docs/test_module-1.0.0-docs.tar.gz"}`))
		case "/docs/test_module-1.0.0-docs.tar.gz":
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(docsArchive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	dir, err := c.FetchDocs(context.Background(), "test_module", "1.0.0", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Verify extracted files.
	for _, name := range []string{"rules.md", "providers.md"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s is empty", name)
		}
	}
}

func TestHasDocs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/modules/with_docs/1.0.0/source.json":
			w.Write([]byte(`{"url": "https://example.com/test.tar.gz", "docs_url": "https://example.com/docs.tar.gz"}`))
		case "/modules/without_docs/1.0.0/source.json":
			w.Write([]byte(`{"url": "https://example.com/test.tar.gz"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	ctx := context.Background()

	has, err := c.HasDocs(ctx, "with_docs", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("expected HasDocs=true for with_docs")
	}

	has, err = c.HasDocs(ctx, "without_docs", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("expected HasDocs=false for without_docs")
	}

	has, err = c.HasDocs(ctx, "nonexistent", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("expected HasDocs=false for nonexistent")
	}
}

func TestExtractTarGz_PathTraversal(t *testing.T) {
	// Create an archive with a path traversal attempt.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{
		Name: "../../../etc/passwd",
		Mode: 0o644,
		Size: 4,
	})
	tw.Write([]byte("evil"))
	tw.Close()
	gz.Close()

	dir := t.TempDir()
	err := extractTarGz(&buf, dir)
	if err != nil {
		t.Fatal(err)
	}

	// The file should NOT have been extracted.
	if _, err := os.Stat(filepath.Join(dir, "../../../etc/passwd")); err == nil {
		t.Error("path traversal was not prevented")
	}
}

func createTestDocsArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
