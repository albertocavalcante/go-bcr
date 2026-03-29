package bcr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPresubmit(t *testing.T) {
	content := `matrix:
  platform: [debian10, ubuntu2004, macos, windows]
  bazel: [7.x, 8.x]
tasks:
  verify_targets:
    build_targets:
      - "@rules_go//..."
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/modules/rules_go/0.53.0/presubmit.yml" {
			w.Write([]byte(content))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	data, err := c.Presubmit(context.Background(), "rules_go", "0.53.0")
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty presubmit data")
	}
	if string(data) != content {
		t.Errorf("got %q, want %q", string(data), content)
	}
}

func TestPresubmit_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	_, err := c.Presubmit(context.Background(), "nonexistent", "1.0.0")
	if err == nil {
		t.Error("expected error for nonexistent module")
	}
}
