package bcr

import "context"

// Registry is the interface for Bazel module registries.
//
// This abstraction allows for different registry backends:
//   - HTTP/HTTPS registries (BCR, mirrors)
//   - Local filesystem registries
//   - Custom implementations
//
// All methods accept a context for cancellation and timeout control.
type Registry interface {
	// Metadata fetches module metadata from the registry.
	// Returns [ErrNotFound] if the module does not exist.
	Metadata(ctx context.Context, module string) (*Metadata, error)

	// Source fetches source information for a specific module version.
	// Returns [ErrNotFound] if the module or version does not exist.
	Source(ctx context.Context, module, version string) (*Source, error)

	// ModuleFile fetches the MODULE.bazel content for a specific version.
	// Returns [ErrNotFound] if the module or version does not exist.
	ModuleFile(ctx context.Context, module, version string) ([]byte, error)
}

// Ensure Client implements Registry at compile time.
var _ Registry = (*Client)(nil)
