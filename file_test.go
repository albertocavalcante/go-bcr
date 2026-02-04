package bcr

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestFileRegistryInterface verifies FileRegistry implements Registry.
func TestFileRegistryInterface(t *testing.T) {
	var _ Registry = (*FileRegistry)(nil)
}

// setupFileRegistry creates a test registry structure in a temp directory.
func setupFileRegistry(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()

	// Create modules/testmod/metadata.json
	modDir := filepath.Join(dir, "modules", "testmod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatal(err)
	}

	meta := &Metadata{
		Versions:   []string{"1.0.0", "2.0.0"},
		Homepage:   "https://example.com",
		Maintainers: []Maintainer{{Name: "Test", GitHub: "test"}},
	}
	metaBytes, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(modDir, "metadata.json"), metaBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create modules/testmod/1.0.0/
	verDir := filepath.Join(modDir, "1.0.0")
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// source.json
	src := &Source{
		URL:       "https://example.com/archive.zip",
		Integrity: "sha256-abc123",
	}
	srcBytes, _ := json.Marshal(src)
	if err := os.WriteFile(filepath.Join(verDir, "source.json"), srcBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	// MODULE.bazel
	moduleContent := []byte(`module(name = "testmod", version = "1.0.0")`)
	if err := os.WriteFile(filepath.Join(verDir, "MODULE.bazel"), moduleContent, 0o644); err != nil {
		t.Fatal(err)
	}

	return dir, func() {}
}

func TestFileRegistryMetadata(t *testing.T) {
	dir, cleanup := setupFileRegistry(t)
	defer cleanup()

	reg := NewFileRegistry(dir)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		meta, err := reg.Metadata(ctx, "testmod")
		if err != nil {
			t.Fatalf("Metadata() error = %v", err)
		}
		if len(meta.Versions) != 2 {
			t.Errorf("got %d versions, want 2", len(meta.Versions))
		}
		if meta.Homepage != "https://example.com" {
			t.Errorf("Homepage = %q, want %q", meta.Homepage, "https://example.com")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := reg.Metadata(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestFileRegistrySource(t *testing.T) {
	dir, cleanup := setupFileRegistry(t)
	defer cleanup()

	reg := NewFileRegistry(dir)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		src, err := reg.Source(ctx, "testmod", "1.0.0")
		if err != nil {
			t.Fatalf("Source() error = %v", err)
		}
		if src.URL != "https://example.com/archive.zip" {
			t.Errorf("URL = %q, want %q", src.URL, "https://example.com/archive.zip")
		}
	})

	t.Run("module not found", func(t *testing.T) {
		_, err := reg.Source(ctx, "nonexistent", "1.0.0")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("version not found", func(t *testing.T) {
		_, err := reg.Source(ctx, "testmod", "9.9.9")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestFileRegistryModuleFile(t *testing.T) {
	dir, cleanup := setupFileRegistry(t)
	defer cleanup()

	reg := NewFileRegistry(dir)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		content, err := reg.ModuleFile(ctx, "testmod", "1.0.0")
		if err != nil {
			t.Fatalf("ModuleFile() error = %v", err)
		}
		expected := `module(name = "testmod", version = "1.0.0")`
		if string(content) != expected {
			t.Errorf("content = %q, want %q", content, expected)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := reg.ModuleFile(ctx, "testmod", "9.9.9")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})
}

func TestFileRegistryString(t *testing.T) {
	reg := NewFileRegistry("/path/to/registry")
	if s := reg.String(); s != "file:///path/to/registry" {
		t.Errorf("String() = %q, want %q", s, "file:///path/to/registry")
	}
}

func TestNewFileRegistryFromURL(t *testing.T) {
	tests := []struct {
		input    string
		wantPath string
		wantOK   bool
	}{
		// file:// URLs
		{"file:///path/to/registry", "/path/to/registry", true},
		{"file:///C:/path/to/registry", "/C:/path/to/registry", true},

		// Unix absolute paths
		{"/absolute/path", "/absolute/path", true},
		{"/", "/", true},

		// Windows absolute paths
		{`C:\path\to\registry`, `C:\path\to\registry`, true},
		{`D:\registry`, `D:\registry`, true},
		{"C:/path/to/registry", "C:/path/to/registry", true},
		{`c:\lowercase`, `c:\lowercase`, true},
		{`Z:\some\path`, `Z:\some\path`, true},

		// Not file paths
		{"https://example.com", "", false},
		{"http://example.com", "", false},
		{"relative/path", "", false},
		{"./relative", "", false},
		{"../parent", "", false},

		// Windows relative paths (NOT absolute)
		{"C:relative", "", false},
		{"C:", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			reg, ok := NewFileRegistryFromURL(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if ok && reg.root != tt.wantPath {
				t.Errorf("root = %q, want %q", reg.root, tt.wantPath)
			}
		})
	}
}

func TestIsWindowsAbsolutePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// Valid Windows absolute paths
		{`C:\`, true},
		{`C:\path`, true},
		{`C:\path\to\registry`, true},
		{"C:/", true},
		{"C:/path", true},
		{"C:/path/to/registry", true},
		{`D:\registry`, true},
		{"d:/registry", true},
		{`Z:\some\path`, true},
		{`a:\lowercase`, true},

		// Invalid - not absolute paths
		{"", false},
		{"C", false},
		{"C:", false},
		{"C:path", false},       // Relative to current dir on C:
		{"/unix/path", false},   // Unix path
		{"relative/path", false},
		{"./relative", false},
		{"1:/invalid", false}, // Invalid drive letter
		{"@:/invalid", false}, // Invalid drive letter
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isWindowsAbsolutePath(tt.path)
			if got != tt.want {
				t.Errorf("isWindowsAbsolutePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFileRegistryType(t *testing.T) {
	reg := NewFileRegistry("/path/to/registry")
	if got := reg.Type(); got != "file" {
		t.Errorf("Type() = %q, want %q", got, "file")
	}
}
