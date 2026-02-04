package bcr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
)

// DefaultBaseURL is the URL of the official Bazel Central Registry.
const DefaultBaseURL = "https://bcr.bazel.build"

// Client is a Bazel Central Registry client.
//
// Client is safe for concurrent use. All methods that perform I/O
// accept a context for cancellation and timeout control.
type Client struct {
	baseURL   string
	http      *http.Client
	userAgent string
	cache     *cache
}

// New creates a new registry client with the given options.
//
// With no options, the client connects to the official BCR at
// https://bcr.bazel.build with no caching.
func New(opts ...Option) *Client {
	cfg := &clientConfig{
		baseURL:   DefaultBaseURL,
		http:      http.DefaultClient,
		userAgent: "go-bcr/1.0",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	c := &Client{
		baseURL:   cfg.baseURL,
		http:      cfg.http,
		userAgent: cfg.userAgent,
	}

	if cfg.cacheDir != "" {
		c.cache = newCache(cfg.cacheDir, cfg.cacheTTL)
	}

	return c
}

// clientConfig holds configuration during client construction.
type clientConfig struct {
	baseURL   string
	http      *http.Client
	userAgent string
	cacheDir  string
	cacheTTL  time.Duration
}

// Option configures a [Client].
type Option func(*clientConfig)

// WithBaseURL sets the registry base URL.
//
// Default: https://bcr.bazel.build
func WithBaseURL(baseURL string) Option {
	return func(c *clientConfig) {
		c.baseURL = baseURL
	}
}

// WithHTTPClient sets a custom HTTP client for requests.
//
// Default: [http.DefaultClient]
func WithHTTPClient(client *http.Client) Option {
	return func(c *clientConfig) {
		c.http = client
	}
}

// WithUserAgent sets the User-Agent header for requests.
//
// Default: "go-bcr/1.0"
func WithUserAgent(ua string) Option {
	return func(c *clientConfig) {
		c.userAgent = ua
	}
}

// WithCacheDir enables local caching in the specified directory.
//
// The cache stores metadata and source information to reduce
// network requests. Pass an empty string to disable caching.
//
// Default: no caching
func WithCacheDir(dir string) Option {
	return func(c *clientConfig) {
		c.cacheDir = dir
	}
}

// WithCacheTTL sets the cache time-to-live duration.
//
// Cached entries older than this duration are considered stale
// and will be refetched. This only applies to metadata; source
// information is cached indefinitely as it's immutable.
//
// Default: 1 hour
func WithCacheTTL(ttl time.Duration) Option {
	return func(c *clientConfig) {
		c.cacheTTL = ttl
	}
}

// Metadata fetches module metadata from the registry.
//
// Returns [ErrNotFound] if the module does not exist.
func (c *Client) Metadata(ctx context.Context, module string) (*Metadata, error) {
	urlPath := path.Join("modules", module, "metadata.json")

	// Check cache first
	if c.cache != nil {
		if data, ok := c.cache.get(urlPath, true); ok {
			var meta Metadata
			if err := json.Unmarshal(data, &meta); err == nil {
				return &meta, nil
			}
		}
	}

	data, err := c.fetch(ctx, urlPath, module, "")
	if err != nil {
		return nil, err
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("bcr: failed to parse metadata for %s: %w", module, err)
	}

	// Cache the result
	if c.cache != nil {
		c.cache.set(urlPath, data)
	}

	return &meta, nil
}

// Source fetches source information for a specific module version.
//
// Returns [ErrNotFound] if the module or version does not exist.
func (c *Client) Source(ctx context.Context, module, version string) (*Source, error) {
	urlPath := path.Join("modules", module, version, "source.json")

	// Check cache (source info is immutable, no TTL needed)
	if c.cache != nil {
		if data, ok := c.cache.get(urlPath, false); ok {
			var src Source
			if err := json.Unmarshal(data, &src); err == nil {
				return &src, nil
			}
		}
	}

	data, err := c.fetch(ctx, urlPath, module, version)
	if err != nil {
		return nil, err
	}

	var src Source
	if err := json.Unmarshal(data, &src); err != nil {
		return nil, fmt.Errorf("bcr: failed to parse source for %s@%s: %w", module, version, err)
	}

	// Cache the result
	if c.cache != nil {
		c.cache.set(urlPath, data)
	}

	return &src, nil
}

// ModuleFile fetches the MODULE.bazel content for a specific version.
//
// Returns [ErrNotFound] if the module or version does not exist.
func (c *Client) ModuleFile(ctx context.Context, module, version string) ([]byte, error) {
	urlPath := path.Join("modules", module, version, "MODULE.bazel")

	// Check cache (immutable)
	if c.cache != nil {
		if data, ok := c.cache.get(urlPath, false); ok {
			return data, nil
		}
	}

	data, err := c.fetch(ctx, urlPath, module, version)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if c.cache != nil {
		c.cache.set(urlPath, data)
	}

	return data, nil
}

// Latest returns the latest non-yanked version of a module.
//
// Returns [ErrNotFound] if the module does not exist or all versions are yanked.
func (c *Client) Latest(ctx context.Context, module string) (string, error) {
	meta, err := c.Metadata(ctx, module)
	if err != nil {
		return "", err
	}

	latest := meta.Latest()
	if latest == "" {
		return "", &NotFoundError{Module: module}
	}
	return latest, nil
}

// Versions returns an iterator over all versions of a module.
//
// The iterator yields versions in registry order (oldest first).
// Use [Metadata] if you need the full metadata including yank information.
func (c *Client) Versions(ctx context.Context, module string) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		meta, err := c.Metadata(ctx, module)
		if err != nil {
			yield("", err)
			return
		}
		for _, v := range meta.Versions {
			if !yield(v, nil) {
				return
			}
		}
	}
}

// Exists reports whether a module exists in the registry.
func (c *Client) Exists(ctx context.Context, module string) (bool, error) {
	_, err := c.Metadata(ctx, module)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// VersionExists reports whether a specific version exists.
func (c *Client) VersionExists(ctx context.Context, module, version string) (bool, error) {
	meta, err := c.Metadata(ctx, module)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return meta.HasVersion(version), nil
}

// ListModules returns all available module names from the registry.
//
// This requires the registry to provide a modules/index.json file.
// Returns [ErrListingNotSupported] if the index is not available.
func (c *Client) ListModules(ctx context.Context) ([]string, error) {
	urlPath := path.Join("modules", "index.json")

	data, err := c.fetch(ctx, urlPath, "", "")
	if err != nil {
		if isNotFound(err) {
			return nil, ErrListingNotSupported
		}
		return nil, err
	}

	var modules []string
	if err := json.Unmarshal(data, &modules); err != nil {
		return nil, fmt.Errorf("bcr: failed to parse module index: %w", err)
	}

	return modules, nil
}

// fetch makes an HTTP GET request and returns the response body.
func (c *Client) fetch(ctx context.Context, urlPath, module, version string) ([]byte, error) {
	u, err := url.JoinPath(c.baseURL, urlPath)
	if err != nil {
		return nil, fmt.Errorf("bcr: invalid URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("bcr: failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &RequestError{URL: u, Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &NotFoundError{
			Module:     module,
			Version:    version,
			StatusCode: resp.StatusCode,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &RequestError{URL: u, StatusCode: resp.StatusCode}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &RequestError{URL: u, Err: fmt.Errorf("failed to read response: %w", err)}
	}

	return data, nil
}

// isNotFound reports whether err indicates a not-found condition.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if err == ErrNotFound {
		return true
	}
	if _, ok := err.(*NotFoundError); ok {
		return true
	}
	// Check wrapped errors
	if u, ok := err.(interface{ Unwrap() error }); ok {
		return isNotFound(u.Unwrap())
	}
	return false
}

// --- Cache implementation ---

type cache struct {
	dir string
	ttl time.Duration
	mu  sync.RWMutex
}

func newCache(dir string, ttl time.Duration) *cache {
	if ttl == 0 {
		ttl = time.Hour
	}
	return &cache{dir: dir, ttl: ttl}
}

func (c *cache) path(key string) string {
	return filepath.Join(c.dir, filepath.FromSlash(key))
}

func (c *cache) get(key string, checkTTL bool) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	p := c.path(key)
	info, err := os.Stat(p)
	if err != nil {
		return nil, false
	}

	if checkTTL && time.Since(info.ModTime()) > c.ttl {
		return nil, false
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	return data, true
}

func (c *cache) set(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	p := c.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return // ignore cache write errors
	}
	_ = os.WriteFile(p, data, 0o644)
}
