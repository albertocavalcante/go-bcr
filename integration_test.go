//go:build integration

package bcr

import (
	"context"
	"testing"
	"time"
)

// Integration tests that hit the real BCR.
// Run with: go test -tags=integration -v

func TestIntegration_RealBCR(t *testing.T) {
	client := New()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("Metadata", func(t *testing.T) {
		meta, err := client.Metadata(ctx, "rules_go")
		if err != nil {
			t.Fatalf("Metadata(rules_go) error = %v", err)
		}

		if len(meta.Versions) == 0 {
			t.Error("expected at least one version")
		}

		if meta.Homepage == "" {
			t.Error("expected homepage to be set")
		}

		t.Logf("rules_go has %d versions, homepage: %s", len(meta.Versions), meta.Homepage)
	})

	t.Run("Source", func(t *testing.T) {
		src, err := client.Source(ctx, "rules_go", "0.50.1")
		if err != nil {
			t.Fatalf("Source(rules_go, 0.50.1) error = %v", err)
		}

		if src.URL == "" {
			t.Error("expected URL to be set")
		}

		if src.Integrity == "" {
			t.Error("expected Integrity to be set")
		}

		t.Logf("URL: %s", src.URL)
		t.Logf("Integrity: %s", src.Integrity)
	})

	t.Run("ModuleFile", func(t *testing.T) {
		content, err := client.ModuleFile(ctx, "rules_go", "0.50.1")
		if err != nil {
			t.Fatalf("ModuleFile() error = %v", err)
		}

		if len(content) == 0 {
			t.Error("expected non-empty MODULE.bazel")
		}

		t.Logf("MODULE.bazel size: %d bytes", len(content))
	})

	t.Run("Latest", func(t *testing.T) {
		version, err := client.Latest(ctx, "rules_go")
		if err != nil {
			t.Fatalf("Latest() error = %v", err)
		}

		if version == "" {
			t.Error("expected non-empty version")
		}

		t.Logf("Latest version: %s", version)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := client.Metadata(ctx, "this-module-definitely-does-not-exist-xyz123")
		if err == nil {
			t.Error("expected error for nonexistent module")
		}

		if !isNotFound(err) {
			t.Errorf("expected NotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("Versions iterator", func(t *testing.T) {
		count := 0
		for v, err := range client.Versions(ctx, "rules_go") {
			if err != nil {
				t.Fatalf("Versions() error = %v", err)
			}
			count++
			if count == 5 {
				break // Just check first 5
			}
			_ = v
		}

		if count < 5 {
			t.Errorf("expected at least 5 versions, got %d", count)
		}
	})
}

func TestIntegration_Cache(t *testing.T) {
	cacheDir := t.TempDir()
	client := New(WithCacheDir(cacheDir))
	ctx := context.Background()

	// First request (uncached)
	start := time.Now()
	_, err := client.Metadata(ctx, "rules_go")
	if err != nil {
		t.Fatalf("first Metadata() error = %v", err)
	}
	firstDuration := time.Since(start)

	// Second request (cached)
	start = time.Now()
	_, err = client.Metadata(ctx, "rules_go")
	if err != nil {
		t.Fatalf("second Metadata() error = %v", err)
	}
	secondDuration := time.Since(start)

	t.Logf("First request: %v", firstDuration)
	t.Logf("Second request (cached): %v", secondDuration)

	// Cached should be significantly faster
	if secondDuration > firstDuration/2 {
		t.Logf("Warning: cached request not significantly faster")
	}
}
