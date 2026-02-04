package bcr

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestModuleListerInterface verifies implementations.
func TestModuleListerInterface(t *testing.T) {
	var _ ModuleLister = (*Client)(nil)
	var _ ModuleLister = (*FileRegistry)(nil)
}

func TestFileRegistryListModules(t *testing.T) {
	dir := t.TempDir()

	// Create several modules
	modules := []string{"rules_go", "rules_python", "protobuf"}
	for _, mod := range modules {
		modDir := filepath.Join(dir, "modules", mod)
		if err := os.MkdirAll(modDir, 0o755); err != nil {
			t.Fatal(err)
		}
		meta := &Metadata{Versions: []string{"1.0.0"}}
		data, _ := json.Marshal(meta)
		if err := os.WriteFile(filepath.Join(modDir, "metadata.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	reg := NewFileRegistry(dir)
	ctx := context.Background()

	t.Run("lists all modules", func(t *testing.T) {
		got, err := reg.ListModules(ctx)
		if err != nil {
			t.Fatalf("ListModules() error = %v", err)
		}
		slices.Sort(got)
		slices.Sort(modules)
		if !slices.Equal(got, modules) {
			t.Errorf("ListModules() = %v, want %v", got, modules)
		}
	})

	t.Run("empty registry", func(t *testing.T) {
		emptyDir := t.TempDir()
		os.MkdirAll(filepath.Join(emptyDir, "modules"), 0o755)
		emptyReg := NewFileRegistry(emptyDir)

		got, err := emptyReg.ListModules(ctx)
		if err != nil {
			t.Fatalf("ListModules() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("ListModules() = %v, want empty", got)
		}
	})

	t.Run("no modules directory", func(t *testing.T) {
		noModDir := t.TempDir()
		noModReg := NewFileRegistry(noModDir)

		_, err := noModReg.ListModules(ctx)
		if err == nil {
			t.Fatal("expected error for missing modules directory")
		}
		if !errors.Is(err, ErrListingNotSupported) {
			t.Errorf("error = %v, want ErrListingNotSupported", err)
		}
	})
}

func TestClientListModules(t *testing.T) {
	modules := []string{"rules_go", "rules_python", "protobuf"}

	t.Run("with index.json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/modules/index.json" {
				json.NewEncoder(w).Encode(modules)
				return
			}
			http.NotFound(w, r)
		}))
		defer srv.Close()

		c := New(WithBaseURL(srv.URL))
		got, err := c.ListModules(context.Background())
		if err != nil {
			t.Fatalf("ListModules() error = %v", err)
		}
		slices.Sort(got)
		slices.Sort(modules)
		if !slices.Equal(got, modules) {
			t.Errorf("ListModules() = %v, want %v", got, modules)
		}
	})

	t.Run("no index returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer srv.Close()

		c := New(WithBaseURL(srv.URL))
		_, err := c.ListModules(context.Background())
		if err == nil {
			t.Fatal("expected error when index.json not available")
		}
		if !errors.Is(err, ErrListingNotSupported) {
			t.Errorf("error = %v, want ErrListingNotSupported", err)
		}
	})
}
