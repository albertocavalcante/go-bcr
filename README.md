# go-bcr

[![CI](https://github.com/albertocavalcante/go-bcr/actions/workflows/ci.yml/badge.svg)](https://github.com/albertocavalcante/go-bcr/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/albertocavalcante/go-bcr.svg)](https://pkg.go.dev/github.com/albertocavalcante/go-bcr)
[![Go Version](https://img.shields.io/github/go-mod/go-version/albertocavalcante/go-bcr)](go.mod)
[![License](https://img.shields.io/github/license/albertocavalcante/go-bcr)](LICENSE)

A lightweight, zero-dependency Go client for the [Bazel Central Registry](https://bcr.bazel.build).

## Requirements

**Go 1.25+** — This package uses `iter.Seq2` from the standard library.

## Installation

```bash
go get github.com/albertocavalcante/go-bcr
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/albertocavalcante/go-bcr"
)

func main() {
    client := bcr.New()
    ctx := context.Background()

    // Get module metadata
    meta, err := client.Metadata(ctx, "rules_go")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("rules_go has %d versions\n", len(meta.Versions))
    fmt.Printf("Latest: %s\n", meta.Latest())

    // Get source info
    src, err := client.Source(ctx, "rules_go", "0.50.1")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("URL: %s\n", src.URL)
    fmt.Printf("Integrity: %s\n", src.Integrity)
}
```

### With Caching

```go
client := bcr.New(
    bcr.WithCacheDir("~/.cache/bcr"),
    bcr.WithCacheTTL(time.Hour),
)
```

### Custom Registry

```go
client := bcr.New(bcr.WithBaseURL("https://registry.example.com"))
```

### Iterating Versions

```go
for version, err := range client.Versions(ctx, "rules_go") {
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(version)
}
```

### Error Handling

```go
meta, err := client.Metadata(ctx, "nonexistent")
if errors.Is(err, bcr.ErrNotFound) {
    // Module doesn't exist
}

var notFound *bcr.NotFoundError
if errors.As(err, &notFound) {
    fmt.Printf("Module %q not found\n", notFound.Module)
}
```

## API

### Client Methods

| Method | Description |
|--------|-------------|
| `Metadata(ctx, module)` | Get module metadata (versions, maintainers, etc.) |
| `Source(ctx, module, version)` | Get source info (URL, integrity, patches) |
| `ModuleFile(ctx, module, version)` | Get MODULE.bazel content |
| `Latest(ctx, module)` | Get latest non-yanked version |
| `Versions(ctx, module)` | Iterate over all versions |
| `Exists(ctx, module)` | Check if module exists |
| `VersionExists(ctx, module, version)` | Check if version exists |

### Options

| Option | Description |
|--------|-------------|
| `WithBaseURL(url)` | Set registry URL (default: https://bcr.bazel.build) |
| `WithHTTPClient(client)` | Set custom HTTP client |
| `WithCacheDir(dir)` | Enable local caching |
| `WithCacheTTL(duration)` | Set cache TTL (default: 1 hour) |
| `WithUserAgent(ua)` | Set User-Agent header |

### Types

```go
type Metadata struct {
    Versions       []string
    YankedVersions map[string]string
    Maintainers    []Maintainer
    Homepage       string
    Repository     []string
}

type Source struct {
    Type        string            // "archive", "git_repository", "local_path"
    URL         string
    Integrity   string
    StripPrefix string
    Patches     map[string]string
    PatchStrip  int
    // ... git/local fields
}

type Maintainer struct {
    Name     string
    Email    string
    GitHub   string
    GitHubID int64
}
```

## License

MIT
