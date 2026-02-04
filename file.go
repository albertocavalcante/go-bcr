package bcr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileRegistry is a Registry backed by a local filesystem.
//
// The filesystem must follow the BCR directory structure:
//
//	modules/
//	├── <module_name>/
//	│   ├── metadata.json
//	│   └── <version>/
//	│       ├── MODULE.bazel
//	│       └── source.json
type FileRegistry struct {
	root string
}

// NewFileRegistry creates a new file-based registry rooted at the given path.
func NewFileRegistry(root string) *FileRegistry {
	return &FileRegistry{root: root}
}

// NewFileRegistryFromURL creates a FileRegistry from a URL string.
//
// Supported formats:
//   - file:///path/to/registry
//   - /absolute/path (Unix)
//   - C:\path\to\registry (Windows)
//   - C:/path/to/registry (Windows with forward slashes)
//
// Returns (nil, false) if the URL is not a file path.
func NewFileRegistryFromURL(url string) (*FileRegistry, bool) {
	// Handle file:// scheme
	if strings.HasPrefix(url, "file://") {
		path := strings.TrimPrefix(url, "file://")
		return NewFileRegistry(path), true
	}

	// Handle absolute Unix paths
	if strings.HasPrefix(url, "/") {
		return NewFileRegistry(url), true
	}

	// Handle Windows absolute paths (C:\... or C:/...)
	if isWindowsAbsolutePath(url) {
		return NewFileRegistry(url), true
	}

	return nil, false
}

// isWindowsAbsolutePath checks if a path is a Windows absolute path.
// Matches patterns like: C:\path, C:/path, D:\path, d:/path
// Does NOT match: C:path (relative), C: (just drive), or UNC paths.
func isWindowsAbsolutePath(path string) bool {
	if len(path) < 3 {
		return false
	}

	// Check for drive letter (A-Z or a-z)
	letter := path[0]
	if !((letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z')) {
		return false
	}

	// Must be followed by colon
	if path[1] != ':' {
		return false
	}

	// Must be followed by path separator (\ or /)
	return path[2] == '\\' || path[2] == '/'
}

// Metadata fetches module metadata from the filesystem.
func (r *FileRegistry) Metadata(ctx context.Context, module string) (*Metadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	path := filepath.Join(r.root, "modules", module, "metadata.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &NotFoundError{Module: module}
		}
		return nil, fmt.Errorf("bcr: failed to read metadata for %s: %w", module, err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("bcr: failed to parse metadata for %s: %w", module, err)
	}

	return &meta, nil
}

// Source fetches source information from the filesystem.
func (r *FileRegistry) Source(ctx context.Context, module, version string) (*Source, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	path := filepath.Join(r.root, "modules", module, version, "source.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &NotFoundError{Module: module, Version: version}
		}
		return nil, fmt.Errorf("bcr: failed to read source for %s@%s: %w", module, version, err)
	}

	var src Source
	if err := json.Unmarshal(data, &src); err != nil {
		return nil, fmt.Errorf("bcr: failed to parse source for %s@%s: %w", module, version, err)
	}

	return &src, nil
}

// ModuleFile fetches the MODULE.bazel content from the filesystem.
func (r *FileRegistry) ModuleFile(ctx context.Context, module, version string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	path := filepath.Join(r.root, "modules", module, version, "MODULE.bazel")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &NotFoundError{Module: module, Version: version}
		}
		return nil, fmt.Errorf("bcr: failed to read MODULE.bazel for %s@%s: %w", module, version, err)
	}

	return data, nil
}

// String returns a string representation of the registry.
func (r *FileRegistry) String() string {
	return "file://" + r.root
}

// ListModules returns all module names in the registry.
func (r *FileRegistry) ListModules(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	modulesDir := filepath.Join(r.root, "modules")
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrListingNotSupported
		}
		return nil, fmt.Errorf("bcr: failed to list modules: %w", err)
	}

	var modules []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Verify it's a valid module (has metadata.json)
			metaPath := filepath.Join(modulesDir, entry.Name(), "metadata.json")
			if _, err := os.Stat(metaPath); err == nil {
				modules = append(modules, entry.Name())
			}
		}
	}

	return modules, nil
}

// Ensure FileRegistry implements Registry at compile time.
var _ Registry = (*FileRegistry)(nil)

// Ensure FileRegistry implements ModuleLister at compile time.
var _ ModuleLister = (*FileRegistry)(nil)
