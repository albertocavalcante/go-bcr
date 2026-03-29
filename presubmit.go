package bcr

import (
	"context"
	"fmt"
	"path"
)

// Presubmit fetches the raw presubmit.yml content for a module version.
//
// The presubmit.yml defines the CI matrix (platforms, Bazel versions, test targets)
// that the module was tested against. Returns [ErrNotFound] if the module or
// version does not exist.
func (c *Client) Presubmit(ctx context.Context, module, version string) ([]byte, error) {
	urlPath := path.Join("modules", module, version, "presubmit.yml")

	if c.cache != nil {
		if data, ok := c.cache.get(urlPath, false); ok {
			return data, nil
		}
	}

	data, err := c.fetch(ctx, urlPath, module, version)
	if err != nil {
		return nil, fmt.Errorf("bcr: presubmit %s@%s: %w", module, version, err)
	}

	if c.cache != nil {
		c.cache.set(urlPath, data)
	}

	return data, nil
}
