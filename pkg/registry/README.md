# 📦 registry

A comprehensive container registry client package for Golang applications that provides an intuitive interface for interacting with OCI-compliant container registries. Built on top of `google/go-containerregistry`, this package offers enhanced functionality with path segmentation, flexible authentication, and seamless integration with the Deckhouse ecosystem.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
  - [Client Interface](#client-interface)
  - [Path Segmentation](#path-segmentation)
- [Creating a Client](#creating-a-client)
- [Authentication](#authentication)
- [Image Operations](#image-operations)
- [Repository Operations](#repository-operations)
- [Advanced Usage](#advanced-usage)
- [Configuration Options](#configuration-options)
- [Error Handling](#error-handling)
- [Best Practices](#best-practices)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

## Overview

The `registry` package provides a high-level, production-ready client for container registry operations. It abstracts the complexity of working with OCI registries while providing powerful features for building repository paths, managing authentication, and performing common registry operations.

Key capabilities:
- **Fluent Path Building**: Chain `WithSegment()` calls to construct complex repository paths
- **Flexible Authentication**: Support for various authentication methods via `authn.Authenticator`
- **Rich Image Operations**: Pull, push, inspect, and extract container images
- **Repository Management**: List tags and enumerate repositories with server-side pagination
- **Thread-Safe**: All operations are safe for concurrent use
- **Context-Aware**: Full support for context cancellation and timeouts

## Features

- **Container Image Management**
  - Pull images with tag or digest references
  - Push images to registries
  - Extract flattened image content
  - Retrieve image configurations and metadata

- **Registry Operations**
  - List all tags in a repository with server-side pagination
  - Enumerate sub-repositories with server-side pagination
  - Check image existence
  - Get image digests and manifests

- **Flexible Configuration**
  - Authentication via `authn.Authenticator` interface
  - TLS configuration (skip verification, insecure HTTP)
  - Structured logging integration
  - Custom transport options

- **Developer-Friendly**
  - Chainable API for building repository paths
  - Type-safe interfaces
  - Comprehensive error types
  - Context support for all operations

## Installation

```bash
go get github.com/deckhouse/deckhouse/pkg/registry
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/deckhouse/deckhouse/pkg/log"
    "github.com/deckhouse/deckhouse/pkg/registry/client"
)

func main() {
    ctx := context.Background()
    logger := log.NewLogger().Named("registry")

    // Create client with authentication
    auth := authn.FromConfig(authn.AuthConfig{
        Username: "myuser",
        Password: "mypassword",
    })
    
    opts := &client.Options{
        Auth:   auth,
        Logger: logger,
    }
    
    // Create base client
    registryClient := client.NewClientWithOptions("registry.example.com", opts)
    
    // Build repository path using segments
    moduleClient := registryClient.
        WithSegment("deckhouse").
        WithSegment("modules").
        WithSegment("my-module")
    
    // List available tags
    tags, err := moduleClient.ListTags(ctx)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Available tags: %v\n", tags)
    
    // Pull and inspect an image
    img, err := moduleClient.GetImage(ctx, "v1.0.0")
    if err != nil {
        log.Fatal(err)
    }
    
    config, err := img.ConfigFile()
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Image labels: %v\n", config.Config.Labels)
}
```

## Core Concepts

### Client Interface

The `Client` interface is the main entry point for all registry operations. It provides methods for image and repository management:

```go
type Client interface {
    // Path management
    WithSegment(segments ...string) Client
    GetRegistry() string
    
    // Image operations
    GetImage(ctx context.Context, tag string, opts ...ImageGetOption) (ClientImage, error)
    PushImage(ctx context.Context, tag string, img v1.Image, opts ...ImagePushOption) error
    GetDigest(ctx context.Context, tag string) (*v1.Hash, error)
    GetManifest(ctx context.Context, tag string) (ManifestResult, error)
    GetImageConfig(ctx context.Context, tag string) (*v1.ConfigFile, error)
    CheckImageExists(ctx context.Context, tag string) error
    
    // Repository operations
    ListTags(ctx context.Context, opts ...ListTagsOption) ([]string, error)
    ListRepositories(ctx context.Context, opts ...ListRepositoriesOption) ([]string, error)
}
```

### Path Segmentation

One of the most powerful features is the ability to build repository paths through chainable `WithSegment()` calls. Each call creates a new client scoped to that path:

```go
// Start with base registry
base := client.NewClientWithOptions("registry.example.com", opts)
// Output: registry.example.com

// Add organization
org := base.WithSegment("myorg")
// Output: registry.example.com/myorg

// Add project
project := org.WithSegment("myproject")
// Output: registry.example.com/myorg/myproject

// Add component
component := project.WithSegment("mycomponent")
// Output: registry.example.com/myorg/myproject/mycomponent
```

You can also add multiple segments at once:

```go
// Single call with multiple segments
component := base.WithSegment("myorg", "myproject", "mycomponent")
// Output: registry.example.com/myorg/myproject/mycomponent
```

## Creating a Client

### Basic Client

```go
import (
    "github.com/deckhouse/deckhouse/pkg/log"
    "github.com/deckhouse/deckhouse/pkg/registry/client"
)

logger := log.NewLogger().Named("registry")

opts := &client.Options{
    Logger: logger,
}

registryClient := client.NewClientWithOptions("registry.example.com", opts)
```

### With Authentication

```go
import "github.com/google/go-containerregistry/pkg/authn"

// Basic authentication
auth := authn.FromConfig(authn.AuthConfig{
    Username: "myuser",
    Password: "mypassword",
})

opts := &client.Options{
    Auth:   auth,
    Logger: logger,
}

registryClient := client.NewClientWithOptions("registry.example.com", opts)
```

### With Token Authentication

```go
// Token-based authentication (e.g., Deckhouse license)
auth := authn.FromConfig(authn.AuthConfig{
    Username:      "json_key",
    Password:      "my-deckhouse-license-token",
    IdentityToken: "my-token",
})

opts := &client.Options{
    Auth:   auth,
    Logger: logger,
}

registryClient := client.NewClientWithOptions("registry.deckhouse.io", opts)
```

### With Custom Authenticator

```go
import "github.com/google/go-containerregistry/pkg/authn"

customAuth := authn.FromConfig(authn.AuthConfig{
    Username: "user",
    Password: "pass",
})

opts := &client.Options{
    Auth:   customAuth,
    Logger: logger,
}

registryClient := client.NewClientWithOptions("registry.example.com", opts)
```

### With TLS Configuration

```go
// Skip TLS verification (for testing)
auth := authn.FromConfig(authn.AuthConfig{
    Username: "myuser",
    Password: "mypassword",
})

opts := &client.Options{
    Auth:          auth,
    TLSSkipVerify: true,
    Logger:        logger,
}

// Use insecure HTTP
opts := &client.Options{
    Insecure: true,
    Logger:   logger,
}

registryClient := client.NewClientWithOptions("registry.example.com", opts)
```

## Authentication

The package supports authentication through the `authn.Authenticator` interface from `go-containerregistry`:

```go
import "github.com/google/go-containerregistry/pkg/authn"

// Basic authentication
auth := authn.FromConfig(authn.AuthConfig{
    Username: "myuser",
    Password: "mypassword",
})

opts := &client.Options{
    Auth:   auth,
    Logger: logger,
}

// Token-based authentication
auth := authn.FromConfig(authn.AuthConfig{
    IdentityToken: "my-token",
})

// OAuth2 token
auth := authn.FromConfig(authn.AuthConfig{
    RegistryToken: "oauth2-token",
})

// Anonymous access (no auth)
opts := &client.Options{
    Logger: logger,
}
```

## Image Operations

### Pull an Image

```go
// Pull by tag
img, err := registryClient.GetImage(ctx, "v1.0.0")
if err != nil {
    log.Fatal(err)
}

// Pull by digest
img, err := registryClient.GetImage(ctx, "@sha256:abc123...")
if err != nil {
    log.Fatal(err)
}
```

### Push an Image

```go
import v1 "github.com/google/go-containerregistry/pkg/v1"

var imageToUpload v1.Image

err := registryClient.PushImage(ctx, "v1.0.1", imageToUpload)
if err != nil {
    log.Fatal(err)
}
```

### Get Image Digest

```go
digest, err := registryClient.GetDigest(ctx, "v1.0.0")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Digest: %s\n", digest.String())
```

### Get Image Manifest

The `GetManifest()` method returns a `ManifestResult` that can represent either a standard manifest or an index manifest (for multi-architecture images).

```go
manifestResult, err := registryClient.GetManifest(ctx, "v1.0.0")
if err != nil {
    log.Fatal(err)
}

// Get the descriptor (contains media type, size, digest)
descriptor := manifestResult.GetDescriptor()
fmt.Printf("Media Type: %s\n", descriptor.GetMediaType())
fmt.Printf("Size: %d bytes\n", descriptor.GetSize())
fmt.Printf("Digest: %s\n", descriptor.GetDigest())

// Check if it's an index manifest (multi-arch image)
if descriptor.GetMediaType().IsIndex() {
    // Handle index manifest (multi-platform)
    indexManifest, err := manifestResult.GetIndexManifest()
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Schema Version: %d\n", indexManifest.GetSchemaVersion())
    
    // List all platform-specific manifests
    for _, manifest := range indexManifest.GetManifests() {
        platform := manifest.GetPlatform()
        if platform != nil {
            fmt.Printf("Platform: %s/%s\n", platform.OS, platform.Architecture)
            fmt.Printf("  Digest: %s\n", manifest.GetDigest())
            fmt.Printf("  Size: %d bytes\n", manifest.GetSize())
        }
    }
} else {
    // Handle regular manifest (single platform)
    manifest, err := manifestResult.GetManifest()
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Schema Version: %d\n", manifest.GetSchemaVersion())
    
    // Access config
    config := manifest.GetConfig()
    fmt.Printf("Config Digest: %s\n", config.GetDigest())
    
    // Access layers
    for i, layer := range manifest.GetLayers() {
        fmt.Printf("Layer %d: %s (%d bytes)\n", i, layer.GetDigest(), layer.GetSize())
    }
    
    // Access annotations
    for key, value := range manifest.GetAnnotations() {
        fmt.Printf("Annotation %s: %s\n", key, value)
    }
}
```

### Get Image Configuration

```go
config, err := registryClient.GetImageConfig(ctx, "v1.0.0")
if err != nil {
    log.Fatal(err)
}

// Access metadata
fmt.Printf("Architecture: %s\n", config.Architecture)
fmt.Printf("OS: %s\n", config.OS)
fmt.Printf("Created: %s\n", config.Created.Time)

// Access labels
for key, value := range config.Config.Labels {
    fmt.Printf("Label %s: %s\n", key, value)
}
```

### Check Image Existence

```go
import "github.com/deckhouse/deckhouse/pkg/registry/client"

err := registryClient.CheckImageExists(ctx, "v1.0.0")
if err == client.ErrImageNotFound {
    fmt.Println("Image not found")
} else if err != nil {
    log.Fatal(err)
} else {
    fmt.Println("Image exists")
}
```

### Extract Image Content

```go
// Pull the image
img, err := registryClient.GetImage(ctx, "v1.0.0")
if err != nil {
    log.Fatal(err)
}

// Extract flattened layers as tar archive
reader := img.Extract()
defer reader.Close()

// Process the tar archive
// Contains all layers flattened into a single stream
```

## Repository Operations

### List Tags

The `ListTags` method supports server-side pagination for large repositories:

```go
import "github.com/deckhouse/deckhouse/pkg/registry/client"

// List first 50 tags
tags, err := registryClient.ListTags(ctx, client.WithTagsLimit(50))
if err != nil {
    log.Fatal(err)
}

for _, tag := range tags {
    fmt.Printf("Tag: %s\n", tag)
}

// Continue pagination from last result
if len(tags) == 50 {
    nextTags, err := registryClient.ListTags(ctx, 
        client.WithTagsLimit(50),
        client.WithTagsLast(tags[len(tags)-1]),
    )
    // Process next page...
}
```

**Available Options:**

```go
// Limit results (server-side)
client.WithTagsLimit(100)

// Continue from specific tag (server-side)
client.WithTagsLast("v1.2.0")
```

**Note:** Pagination is now handled server-side by go-containerregistry, providing better performance for large repositories.

### List Repositories

The `ListRepositories` method supports server-side pagination for large registry namespaces:

```go
import "github.com/deckhouse/deckhouse/pkg/registry/client"

// List first 100 repositories
repos, err := registryClient.ListRepositories(ctx, client.WithReposLimit(100))
if err != nil {
    log.Fatal(err)
}

for _, repo := range repos {
    fmt.Printf("Repository: %s\n", repo)
}

// Continue pagination from last result
if len(repos) == 100 {
    nextRepos, err := registryClient.ListRepositories(ctx, 
        client.WithReposLimit(100),
        client.WithReposLast(repos[len(repos)-1]),
    )
    // Process next page...
}
```

**Available Options:**

```go
// Limit results (server-side)
client.WithReposLimit(50)

// Continue from specific repository (server-side)
client.WithReposLast("myproject")
```

**Note:** Pagination is now handled server-side by go-containerregistry, providing better performance for large registries.

### Discover Repository Structure

```go
// Base client
base := client.NewClientWithOptions("registry.example.com", opts)

// List organizations with pagination
orgs, err := base.ListRepositories(ctx, client.WithReposLimit(50))
if err != nil {
    log.Fatal(err)
}

// For each organization, list projects
for _, org := range orgs {
    orgClient := base.WithSegment(org)
    projects, err := orgClient.ListRepositories(ctx, 
        client.WithReposLimit(20),
    )
    if err != nil {
        log.Printf("Failed to list projects for %s: %v", org, err)
        continue
    }
    
    fmt.Printf("Organization: %s\n", org)
    for _, project := range projects {
        fmt.Printf("  Project: %s\n", project)
    }
}
```

## Advanced Usage

### Working with Context

All operations support context for cancellation and timeouts:

```go
import "time"

// With timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

img, err := registryClient.GetImage(ctx, "v1.0.0")
if err != nil {
    if ctx.Err() == context.DeadlineExceeded {
        log.Println("Operation timed out")
    }
}

// With cancellation
ctx, cancel := context.WithCancel(context.Background())

go func() {
    // Cancel after some condition
    time.Sleep(5 * time.Second)
    cancel()
}()

tags, err := registryClient.ListTags(ctx)
if err != nil && ctx.Err() == context.Canceled {
    log.Println("Operation canceled")
}
```

### Concurrent Operations

The client is thread-safe and can be used concurrently:

```go
import "sync"

func processTags(ctx context.Context, client registry.Client, tags []string) {
    var wg sync.WaitGroup
    
    for _, tag := range tags {
        wg.Add(1)
        go func(t string) {
            defer wg.Done()
            
            digest, err := client.GetDigest(ctx, t)
            if err != nil {
                log.Printf("Failed to get digest for %s: %v", t, err)
                return
            }
            
            fmt.Printf("Tag %s: %s\n", t, digest)
        }(tag)
    }
    
    wg.Wait()
}
```

### Working with Image Layers

```go
img, err := registryClient.GetImage(ctx, "v1.0.0")
if err != nil {
    log.Fatal(err)
}

layers, err := img.Layers()
if err != nil {
    log.Fatal(err)
}

for i, layer := range layers {
    digest, _ := layer.Digest()
    size, _ := layer.Size()
    
    fmt.Printf("Layer %d: %s (%d bytes)\n", i, digest, size)
}
```

### Platform-Specific Image Retrieval

When working with multi-architecture images, you can specify the platform (OS/architecture) to retrieve the correct image variant using the `WithPlatform` option:

```go
import (
    v1 "github.com/google/go-containerregistry/pkg/v1"
    "github.com/deckhouse/deckhouse/pkg/registry/client"
)

// Specify platform for image retrieval
platform := &v1.Platform{
    OS:           "linux",
    Architecture: "amd64",
}

img, err := registryClient.GetImage(ctx, "v1.0.0", client.WithPlatform{Platform: platform})
if err != nil {
    log.Fatal(err)
}
```

**Common Platform Examples:**

```go
// Linux AMD64
platform := &v1.Platform{
    OS:           "linux",
    Architecture: "amd64",
}

// Linux ARM64
platform := &v1.Platform{
    OS:           "linux",
    Architecture: "arm64",
}

// Linux ARM v7
platform := &v1.Platform{
    OS:           "linux",
    Architecture: "arm",
    Variant:      "v7",
}

// Windows AMD64
platform := &v1.Platform{
    OS:           "windows",
    Architecture: "amd64",
}
```

**Use Cases:**

1. **Building Multi-Arch Images**: When creating images for different architectures
   ```go
   // Pull ARM64 base image
   arm64Platform := &v1.Platform{
       OS:           "linux",
       Architecture: "arm64",
   }
   baseImg, err := registryClient.GetImage(ctx, "base:latest", 
       client.WithPlatform{Platform: arm64Platform})
   ```

2. **Cross-Platform Development**: When working on one platform but targeting another
   ```go
   // On AMD64, pull ARM64 image for inspection
   targetPlatform := &v1.Platform{
       OS:           "linux",
       Architecture: "arm64",
   }
   img, err := registryClient.GetImage(ctx, "myapp:v1.0.0",
       client.WithPlatform{Platform: targetPlatform})
   ```

3. **Testing Platform-Specific Variants**: Verify different architecture builds
   ```go
   platforms := []*v1.Platform{
       {OS: "linux", Architecture: "amd64"},
       {OS: "linux", Architecture: "arm64"},
       {OS: "linux", Architecture: "arm", Variant: "v7"},
   }
   
   for _, platform := range platforms {
       img, err := registryClient.GetImage(ctx, "myapp:latest",
           client.WithPlatform{Platform: platform})
       if err != nil {
           log.Printf("Failed to get %s/%s: %v", 
               platform.OS, platform.Architecture, err)
           continue
       }
       
       // Verify image config
       config, _ := img.ConfigFile()
       fmt.Printf("Platform: %s/%s, Size: %d layers\n",
           config.OS, config.Architecture, len(config.RootFS.DiffIDs))
   }
   ```

**Note**: If no platform is specified, the registry will typically return the manifest for the host's native platform. When working with multi-platform manifest lists (OCI Image Index), specifying a platform ensures you get the correct platform-specific variant.

## Configuration Options

The `Options` struct provides comprehensive configuration:

```go
type Options struct {
    // Authentication
    Auth authn.Authenticator  // Authenticator for registry access
    
    // TLS Configuration
    Insecure      bool  // Use HTTP instead of HTTPS
    TLSSkipVerify bool  // Skip TLS certificate verification
    
    // Logging
    Logger *log.Logger  // Custom logger (auto-created if nil)
}
```

### Complete Example

```go
import "github.com/google/go-containerregistry/pkg/authn"

logger := log.NewLogger().Named("registry")

auth := authn.FromConfig(authn.AuthConfig{
    Username: "myuser",
    Password: "mypassword",
})

opts := &client.Options{
    Auth:          auth,
    TLSSkipVerify: false,
    Insecure:      false,
    Logger:        logger,
}

registryClient := client.NewClientWithOptions("registry.example.com", opts)
```

## Error Handling

### Specific Error Types

```go
import (
    "errors"
    "github.com/deckhouse/deckhouse/pkg/registry/client"
)

err := registryClient.CheckImageExists(ctx, "v1.0.0")
if errors.Is(err, client.ErrImageNotFound) {
    // Image doesn't exist (not a fatal error)
    fmt.Println("Image not found")
} else if err != nil {
    // Other error (potentially fatal)
    log.Fatal(err)
}
```

### Transport Errors

```go
import "github.com/google/go-containerregistry/pkg/v1/remote/transport"

img, err := registryClient.GetImage(ctx, "v1.0.0")
if err != nil {
    var transportErr *transport.Error
    if errors.As(err, &transportErr) {
        switch transportErr.StatusCode {
        case 401:
            log.Println("Authentication failed")
        case 403:
            log.Println("Access forbidden")
        case 404:
            log.Println("Image not found")
        case 500:
            log.Println("Registry server error")
        }
    }
}
```

### Graceful Error Handling

```go
// Try multiple tags with fallback
tags := []string{"latest", "stable", "v1.0.0"}

var img registry.ClientImage
var err error

for _, tag := range tags {
    img, err = registryClient.GetImage(ctx, tag)
    if err == nil {
        fmt.Printf("Successfully pulled: %s\n", tag)
        break
    }
    
    if errors.Is(err, client.ErrImageNotFound) {
        continue // Try next tag
    }
    
    log.Fatal(err) // Fatal error
}
```

## Best Practices

### 1. Use Path Segmentation

```go
// Good: Build paths incrementally for flexibility
base := client.NewClientWithOptions("registry.example.com", opts)
org := base.WithSegment("myorg")
project := org.WithSegment("myproject")

// Avoid: Hardcoding full paths (less flexible)
fullPath := base.WithSegment("myorg/project") // Treated as single segment
```

### 2. Reuse Client Instances

```go
// Good: Create once, reuse
registryClient := client.NewClientWithOptions("registry.example.com", opts)

for _, tag := range tags {
    digest, _ := registryClient.GetDigest(ctx, tag)
    // ...
}

// Avoid: Creating new clients repeatedly
for _, tag := range tags {
    client := client.NewClientWithOptions("registry.example.com", opts)
    digest, _ := client.GetDigest(ctx, tag)
}
```

### 3. Always Use Context with Timeout

```go
// Good: Reasonable timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

img, err := registryClient.GetImage(ctx, "v1.0.0")

// Avoid: No timeout (can hang indefinitely)
img, err := registryClient.GetImage(context.Background(), "v1.0.0")
```

### 4. Close Readers

```go
// Good: Always close readers
img, _ := registryClient.GetImage(ctx, "v1.0.0")
reader := img.Extract()
defer reader.Close()

// Process reader...

// Avoid: Not closing (resource leak)
reader := img.Extract()
// Use reader without closing
```

### 5. Handle Errors Appropriately

```go
// Good: Distinguish between different errors
err := registryClient.CheckImageExists(ctx, "v1.0.0")
if errors.Is(err, client.ErrImageNotFound) {
    // Expected, handle gracefully
    useDefaultImage()
} else if err != nil {
    // Unexpected error
    log.Fatal(err)
}

// Avoid: Treating all errors as fatal
if err != nil {
    log.Fatal(err) // Image not found is not fatal
}
```

## Examples

### Mirror Images Between Registries

```go
func mirrorImage(ctx context.Context, source, target registry.Client, tag string) error {
    // Pull from source
    img, err := source.GetImage(ctx, tag)
    if err != nil {
        return fmt.Errorf("pull failed: %w", err)
    }
    
    // Push to target
    err = target.PushImage(ctx, tag, img)
    if err != nil {
        return fmt.Errorf("push failed: %w", err)
    }
    
    return nil
}

// Usage
sourceClient := client.NewClientWithOptions("source.example.com", sourceOpts).
    WithSegment("org", "project")

targetClient := client.NewClientWithOptions("target.example.com", targetOpts).
    WithSegment("mirror", "org", "project")

err := mirrorImage(ctx, sourceClient, targetClient, "v1.0.0")
```

### Synchronize Repository Tags

```go
func syncTags(ctx context.Context, source, target registry.Client) error {
    sourceTags, err := source.ListTags(ctx)
    if err != nil {
        return err
    }
    
    targetTags, err := target.ListTags(ctx)
    if err != nil {
        return err
    }
    
    // Find missing tags
    targetSet := make(map[string]bool)
    for _, tag := range targetTags {
        targetSet[tag] = true
    }
    
    // Mirror missing tags
    for _, tag := range sourceTags {
        if !targetSet[tag] {
            if err := mirrorImage(ctx, source, target, tag); err != nil {
                log.Printf("Failed to mirror %s: %v", tag, err)
                continue
            }
            fmt.Printf("Mirrored: %s\n", tag)
        }
    }
    
    return nil
}
```

### Inspect Image Metadata

```go
func inspectImage(ctx context.Context, client registry.Client, tag string) error {
    config, err := client.GetImageConfig(ctx, tag)
    if err != nil {
        return err
    }
    
    fmt.Printf("Image: %s\n", tag)
    fmt.Printf("Architecture: %s\n", config.Architecture)
    fmt.Printf("OS: %s\n", config.OS)
    fmt.Printf("Created: %s\n", config.Created.Time)
    
    if len(config.Config.Labels) > 0 {
        fmt.Println("Labels:")
        for key, value := range config.Config.Labels {
            fmt.Printf("  %s: %s\n", key, value)
        }
    }
    
    return nil
}
```

## Troubleshooting

### Authentication Failures (401 Unauthorized)

**Problem**: Getting 401 errors when accessing registry.

**Solution**: Verify credentials and authentication method:

```go
// Check credentials are correct
auth := authn.FromConfig(authn.AuthConfig{
    Username: "correct-username",
    Password: "correct-password",
})

opts := &client.Options{
    Auth:   auth,
    Logger: logger, // Enable logging
}
```

### TLS Certificate Verification Errors

**Problem**: Certificate verification failures.

**Solution**: For development/testing (not production):

```go
opts := &client.Options{
    TLSSkipVerify: true,
    Logger:        logger,
}
```

**Better Solution**: Add certificates to system trust store or use custom transport.

### Connection Timeouts

**Problem**: Operations hanging or timing out.

**Solution**: Use appropriate timeouts:

```go
// Increase timeout for large images
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel()

img, err := registryClient.GetImage(ctx, "large-image:latest")
```

### Image Not Found Errors

**Problem**: Cannot find expected images.

**Solution**: Check image existence before operations:

```go
err := registryClient.CheckImageExists(ctx, "v1.0.0")
if errors.Is(err, client.ErrImageNotFound) {
    log.Println("Image doesn't exist, check tag name")
} else if err != nil {
    log.Println("Error checking image:", err)
}
```

### Registry Not Responding

**Problem**: Cannot connect to registry.

**Solution**: For HTTP registries (not HTTPS):

```go
opts := &client.Options{
    Insecure: true, // Enable HTTP
    Logger:   logger,
}
```

### Debug Logging

Enable detailed logging to diagnose issues:

```go
import "log/slog"

logger := log.NewLogger(
    log.WithLevel(slog.LevelDebug),
).Named("registry-debug")

opts := &client.Options{
    Logger: logger,
}

// All operations will log detailed information
```

## License

Apache License 2.0

## Contributing

Contributions are welcome! Please ensure:
- Code follows existing patterns
- All operations are thread-safe
- Tests are included
- Documentation is updated
