// Package bcr provides a client for the Bazel Central Registry.
//
// The Bazel Central Registry (BCR) is the default registry for Bazel's
// external dependency system (Bzlmod). This package provides a lightweight,
// zero-dependency client for querying module metadata and source information.
//
// # Basic Usage
//
//	client := bcr.New()
//
//	// Get module metadata
//	meta, err := client.Metadata(ctx, "rules_go")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Available versions:", meta.Versions)
//
//	// Get source info for a specific version
//	src, err := client.Source(ctx, "rules_go", "0.50.1")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Download URL:", src.URL)
//
// # Custom Registry
//
// To use a private or mirror registry:
//
//	client := bcr.New(bcr.WithBaseURL("https://registry.example.com"))
//
// # Caching
//
// Enable local caching to reduce network requests:
//
//	client := bcr.New(bcr.WithCacheDir("~/.cache/bcr"))
//
// # Error Handling
//
// Use [errors.Is] to check for [ErrNotFound]:
//
//	meta, err := client.Metadata(ctx, "nonexistent")
//	if errors.Is(err, bcr.ErrNotFound) {
//	    // module does not exist
//	}
//
// Use [errors.As] to get detailed error information:
//
//	var notFound *bcr.NotFoundError
//	if errors.As(err, &notFound) {
//	    fmt.Printf("module %s not found\n", notFound.Module)
//	}
package bcr
