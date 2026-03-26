package bcr

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoDocs is returned when a module version does not publish documentation.
var ErrNoDocs = errors.New("bcr: module version does not publish docs")

// FetchDocs downloads and extracts the documentation archive for a module version.
//
// The docs_url field in source.json must be set; otherwise [ErrNoDocs] is returned.
// The archive is expected to be a gzipped tar (docs.tar.gz) containing Stardoc-generated
// Markdown files.
//
// The extracted files are written to dir. If dir is empty, a temporary directory is
// created and its path is returned. The caller is responsible for cleaning up the
// directory when done.
//
// Returns the directory path containing the extracted documentation files.
func (c *Client) FetchDocs(ctx context.Context, module, version, dir string) (string, error) {
	src, err := c.Source(ctx, module, version)
	if err != nil {
		return "", fmt.Errorf("bcr: FetchDocs %s@%s: %w", module, version, err)
	}

	if src.DocsURL == "" {
		return "", ErrNoDocs
	}

	createdTmp := false
	if dir == "" {
		dir, err = os.MkdirTemp("", "bcr-docs-*")
		if err != nil {
			return "", fmt.Errorf("bcr: FetchDocs: %w", err)
		}
		createdTmp = true
	}

	if err := c.downloadAndExtract(ctx, src.DocsURL, dir); err != nil {
		if createdTmp {
			os.RemoveAll(dir)
		}
		return "", fmt.Errorf("bcr: FetchDocs %s@%s: %w", module, version, err)
	}

	return dir, nil
}

// HasDocs reports whether a module version publishes documentation.
func (c *Client) HasDocs(ctx context.Context, module, version string) (bool, error) {
	src, err := c.Source(ctx, module, version)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return src.DocsURL != "", nil
}

func (c *Client) downloadAndExtract(ctx context.Context, url, dir string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	// Limit download to 100MB to prevent abuse from malicious docs_url.
	const maxDocsSize = 100 << 20
	return extractTarGz(io.LimitReader(resp.Body, maxDocsSize), dir)
}

func extractTarGz(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		// Sanitize path to prevent directory traversal.
		name := filepath.Clean(hdr.Name)
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			continue
		}

		target := filepath.Join(dir, name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err)
			}
			f, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			// Limit extraction size to 50MB per file.
			if _, err := io.Copy(f, io.LimitReader(tr, 50<<20)); err != nil {
				f.Close()
				return fmt.Errorf("write %s: %w", target, err)
			}
			f.Close()
		}
	}
}
