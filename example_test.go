package bcr_test

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/albertocavalcante/go-bcr"
)

func Example() {
	client := bcr.New()
	ctx := context.Background()

	// Get module metadata
	meta, err := client.Metadata(ctx, "rules_go")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("rules_go has %d versions\n", len(meta.Versions))
	fmt.Printf("Latest: %s\n", meta.Latest())
}

func Example_customRegistry() {
	// Use a mirror or private registry
	client := bcr.New(bcr.WithBaseURL("https://registry.example.com"))
	_ = client
}

func Example_withCaching() {
	// Enable local caching
	client := bcr.New(bcr.WithCacheDir("/tmp/bcr-cache"))
	_ = client
}

func ExampleClient_Latest() {
	client := bcr.New()
	ctx := context.Background()

	version, err := client.Latest(ctx, "rules_go")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Latest rules_go: %s\n", version)
}

func ExampleClient_Source() {
	client := bcr.New()
	ctx := context.Background()

	src, err := client.Source(ctx, "rules_go", "0.50.1")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Download URL: %s\n", src.URL)
	fmt.Printf("Integrity: %s\n", src.Integrity)
}

func ExampleClient_Versions() {
	client := bcr.New()
	ctx := context.Background()

	fmt.Println("Available versions:")
	for v, err := range client.Versions(ctx, "rules_go") {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(v)
	}
}

func ExampleNotFoundError() {
	client := bcr.New()
	ctx := context.Background()

	_, err := client.Metadata(ctx, "nonexistent-module")
	if errors.Is(err, bcr.ErrNotFound) {
		fmt.Println("Module not found")
	}

	// Get detailed information
	var notFound *bcr.NotFoundError
	if errors.As(err, &notFound) {
		fmt.Printf("Module %q does not exist\n", notFound.Module)
	}
}

func ExampleMetadata_IsYanked() {
	client := bcr.New()
	ctx := context.Background()

	meta, err := client.Metadata(ctx, "rules_go")
	if err != nil {
		log.Fatal(err)
	}

	// Check if a specific version is yanked
	if meta.IsYanked("0.29.0") {
		reason := meta.YankReason("0.29.0")
		fmt.Printf("Version 0.29.0 is yanked: %s\n", reason)
	}
}
