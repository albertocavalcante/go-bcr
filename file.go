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

	return nil, false
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

// Ensure FileRegistry implements Registry at compile time.
var _ Registry = (*FileRegistry)(nil)
