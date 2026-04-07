# 📦 registry

A comprehensive container registry client package for Golang applications that provides an intuitive interface for interacting with OCI-compliant container registries. Built on top of `google/go-containerregistry`, this package offers enhanced functionality with path segmentation, flexible authentication, and seamless integration with the Deckhouse ecosystem.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
  - [Package Layout](#package-layout)
  - [Interface Hierarchy](#interface-hierarchy)
- [Core Concepts](#core-concepts)
  - [Path Segmentation](#path-segmentation)
- [Creating a Client](#creating-a-client)
- [Authentication](#authentication)
- [Image Operations](#image-operations)
  - [Pull an Image](#pull-an-image)
  - [Push an Image](#push-an-image)
  - [Push an Image Index (Multi-Arch)](#push-an-image-index-multi-arch)
  - [Get Image Digest](#get-image-digest)
  - [Get Image Manifest](#get-image-manifest)
  - [Get Image Configuration](#get-image-configuration)
  - [Check Image Existence](#check-image-existence)
  - [Extract Image Content](#extract-image-content)
- [Tag and Lifecycle Operations](#tag-and-lifecycle-operations)
  - [Tag an Image (Retag)](#tag-an-image-retag)
  - [Delete a Tag](#delete-a-tag)
  - [Delete by Digest](#delete-by-digest)
  - [Copy an Image](#copy-an-image)
- [Repository Operations](#repository-operations)
- [Advanced Usage](#advanced-usage)
  - [Transport Middlewares](#transport-middlewares)
  - [Custom Transport](#custom-transport)
  - [Proxy Configuration](#proxy-configuration)
  - [Platform-Specific Image Retrieval](#platform-specific-image-retrieval)
  - [Working with Context](#working-with-context)
  - [Concurrent Operations](#concurrent-operations)
  - [Working with Image Layers](#working-with-image-layers)
- [Configuration Options](#configuration-options)
- [Error Handling](#error-handling)
- [Best Practices](#best-practices)
- [Examples](#examples)
- [Troubleshooting](#troubleshooting)

## Overview

The `registry` package provides a high-level, production-ready client for container registry operations. It abstracts the complexity of working with OCI registries while providing powerful features for building repository paths, managing authentication, and performing common registry operations.

Key capabilities:
- **Fluent Path Building**: Chain `WithSegment()` calls to construct complex repository paths
- **Flexible Authentication**: Support for various authentication methods via `authn.Authenticator`, keychains, and Docker config JSON
- **Rich Image Operations**: Pull, push, inspect, extract, retag, copy, and delete container images
- **Multi-Arch Support**: Push and pull image indexes (multi-platform manifest lists)
- **Repository Management**: List tags and enumerate repositories with server-side pagination
- **Transport Middlewares**: Composable middleware chain for metrics, tracing, logging, and rate-limiting
- **Thread-Safe**: All operations are safe for concurrent use
- **Context-Aware**: Full support for context cancellation and timeouts

## Features

- **Container Image Management**
  - Pull images with tag or digest references
  - Push single images and multi-arch image indexes
  - Extract flattened image content
  - Retrieve image configurations and metadata
  - Platform-specific image retrieval for multi-arch images

- **Tag & Lifecycle Operations**
  - Retag images without re-uploading layers (efficient manifest PUT)
  - Delete tags from registries
  - Delete manifests by digest
  - Copy images between registries (server-side mount when possible)

- **Registry Operations**
  - List all tags in a repository with server-side pagination
  - Enumerate sub-repositories with server-side pagination
  - Check image existence (HEAD with GET fallback)
  - Get image digests and manifests

- **Flexible Configuration**
  - Authentication via `authn.Authenticator`, `authn.Keychain`, or Docker config JSON
  - TLS configuration (custom CA, skip verification, insecure HTTP)
  - Custom HTTP transports and explicit proxy support
  - Transport middleware chain (metrics, tracing, logging, rate-limiting)
  - Structured logging integration
  - Per-operation timeouts

- **Developer-Friendly**
  - Chainable API for building repository paths
  - Clean interface/implementation separation (`registry` interfaces, `client` implementation)
  - Type-safe option interfaces with apply pattern
  - Comprehensive error types and sentinel errors
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

    "github.com/google/go-containerregistry/pkg/authn"

    decklog "github.com/deckhouse/deckhouse/pkg/log"
    "github.com/deckhouse/deckhouse/pkg/registry/client"
)

func main() {
    ctx := context.Background()
    logger := decklog.NewLogger().Named("registry")

    // Create base client using functional options (preferred)
    registryClient := client.New("registry.example.com",
        client.WithLoginPassword("myuser", "mypassword"),
        client.WithLogger(logger),
    )

    // Or using the Options struct directly
    auth := authn.FromConfig(authn.AuthConfig{
        Username: "myuser",
        Password: "mypassword",
    })
    opts := &client.Options{
        Auth:   auth,
        Logger: logger,
    }
    registryClient = client.NewClientWithOptions("registry.example.com", opts)

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

    // Retag without re-uploading layers
    if err := moduleClient.TagImage(ctx, "v1.0.0", "latest"); err != nil {
        log.Fatal(err)
    }

    // Copy image to another registry
    destClient := client.New("mirror.example.com",
        client.WithLoginPassword("user", "pass"),
    ).WithSegment("deckhouse", "modules", "my-module")

    if err := moduleClient.CopyImage(ctx, "v1.0.0", destClient, "v1.0.0"); err != nil {
        log.Fatal(err)
    }
}
```

## Architecture

### Package Layout

```
pkg/registry/
├── client.go       # Client interface definition
├── errors.go       # Sentinel errors (ErrImageNotFound)
├── image.go        # Image, ManifestResult, Manifest, IndexManifest, Descriptor interfaces
├── options.go      # Option interfaces (ImageGetOption, ImagePushOption, ListTagsOption, etc.)
├── go.mod
├── README.md
└── client/         # Concrete implementation
    ├── auth.go         # Docker config JSON parsing, credential extraction
    ├── client.go       # Client struct and all registry operations
    ├── image.go        # Image, ManifestResult, Manifest, IndexManifest, Descriptor structs
    ├── middleware.go    # TransportMiddleware, RoundTripperFunc, WithMiddleware
    └── options.go      # Options struct, functional options (With*), transport building
```

### Interface Hierarchy

The package separates **interfaces** (top-level `registry` package) from **implementations** (`client` sub-package), allowing consumers to depend only on the interfaces.

**`registry.Client`** — Main entry point for all operations:

```
Client
├── WithSegment(segments ...string) Client
├── GetRegistry() string
├── GetImage(ctx, tag, ...ImageGetOption) (Image, error)
├── PushImage(ctx, tag, v1.Image, ...ImagePushOption) error
├── PushIndex(ctx, tag, v1.ImageIndex, ...ImagePushOption) error
├── GetDigest(ctx, tag) (*v1.Hash, error)
├── GetManifest(ctx, tag) (ManifestResult, error)
├── GetImageConfig(ctx, tag) (*v1.ConfigFile, error)
├── CheckImageExists(ctx, tag) error
├── ListTags(ctx, ...ListTagsOption) ([]string, error)
├── ListRepositories(ctx, ...ListRepositoriesOption) ([]string, error)
├── DeleteTag(ctx, tag) error
├── DeleteByDigest(ctx, v1.Hash) error
├── TagImage(ctx, sourceTag, destTag) error
└── CopyImage(ctx, srcTag, dest Client, destTag) error
```

**`registry.Image`** — extends `v1.Image` with extraction:

```
Image (embeds v1.Image)
└── Extract() io.ReadCloser
```

**`registry.ManifestResult`** — wraps manifest or index manifest:

```
ManifestResult
├── GetMediaType() types.MediaType
├── GetManifest() (Manifest, error)
├── GetIndexManifest() (IndexManifest, error)
└── GetDescriptor() Descriptor
```

**`registry.Manifest`** / **`registry.IndexManifest`** / **`registry.Descriptor`** — typed manifest access.

## Core Concepts

### Path Segmentation

One of the most powerful features is the ability to build repository paths through chainable `WithSegment()` calls. Each call creates a **new** client scoped to that path (the original client is unchanged):

```go
// Start with base registry
base := client.New("registry.example.com", opts...)
// Path: registry.example.com

// Add organization
org := base.WithSegment("myorg")
// Path: registry.example.com/myorg

// Add project
project := org.WithSegment("myproject")
// Path: registry.example.com/myorg/myproject

// Add component
component := project.WithSegment("mycomponent")
// Path: registry.example.com/myorg/myproject/mycomponent
```

You can also add multiple segments at once:

```go
// Single call with multiple segments
component := base.WithSegment("myorg", "myproject", "mycomponent")
// Path: registry.example.com/myorg/myproject/mycomponent
```

Segments are trimmed of leading/trailing slashes. Empty segment lists return the same client.

## Creating a Client

### Using Functional Options (Preferred)

```go
import (
    "time"

    "github.com/google/go-containerregistry/pkg/authn"

    "github.com/deckhouse/deckhouse/pkg/log"
    "github.com/deckhouse/deckhouse/pkg/registry/client"
)

// Anonymous / default keychain
registryClient := client.New("registry.example.com",
    client.WithLogger(log.NewLogger().Named("registry")),
)

// With explicit credentials
registryClient := client.New("registry.example.com",
    client.WithLoginPassword("myuser", "mypassword"),
    client.WithLogger(log.NewLogger().Named("registry")),
)

// With TLS options and timeout
registryClient := client.New("registry.example.com",
    client.WithAuth(auth),
    client.WithTLSSkipVerify(true),
    client.WithTimeout(30*time.Second),
    client.WithLogger(log.NewLogger().Named("registry")),
)
```

### Using the Options Struct

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

### With Keychain

```go
// Use a custom keychain (e.g., Kubernetes service-account keychain)
// Keychain is used only when Auth is nil.
registryClient := client.New("registry.example.com",
    client.WithKeychain(myKeychain),
)
```

### From a Docker config JSON

`WithDockercfg` parses a raw or base64-encoded `dockerconfig.json` and extracts
credentials for the target repository. Returns `authn.Anonymous` when the matching
entry has empty username and password.

```go
dockercfgOpt, err := client.WithDockercfg("registry.example.com", dockerCfgBase64)
if err != nil {
    log.Fatal(err)
}

registryClient := client.New("registry.example.com", dockercfgOpt)
```

### With TLS Configuration

```go
// Skip TLS verification (for testing)
registryClient := client.New("registry.example.com",
    client.WithAuth(auth),
    client.WithTLSSkipVerify(true),
)

// Custom CA certificate
registryClient := client.New("registry.example.com",
    client.WithAuth(auth),
    client.WithCA(caPEM),
)

// Use insecure HTTP
registryClient := client.New("registry.example.com",
    client.WithInsecure(true),
)
```

## Authentication

The package supports multiple authentication strategies. `Auth` takes precedence over `Keychain`; if neither is set, `authn.DefaultKeychain` is used.

| Method | Function | Description |
|---|---|---|
| Explicit authenticator | `WithAuth(auth)` | Any `authn.Authenticator` implementation |
| Username / password | `WithLoginPassword(u, p)` | Convenience wrapper around `authn.Basic` |
| Docker config JSON | `WithDockercfg(repo, cfg)` | Parses raw or base64-encoded config |
| Keychain | `WithKeychain(kc)` | Custom `authn.Keychain` (used when Auth is nil) |

```go
import "github.com/google/go-containerregistry/pkg/authn"

// Basic authentication
auth := authn.FromConfig(authn.AuthConfig{
    Username: "myuser",
    Password: "mypassword",
})
registryClient := client.New("registry.example.com", client.WithAuth(auth))

// Convenience helper – equivalent to the above
registryClient := client.New("registry.example.com",
    client.WithLoginPassword("myuser", "mypassword"),
)

// Token-based authentication
auth = authn.FromConfig(authn.AuthConfig{
    IdentityToken: "my-token",
})

// OAuth2 token
auth = authn.FromConfig(authn.AuthConfig{
    RegistryToken: "oauth2-token",
})

// Anonymous access — use anonymous authenticator
registryClient := client.New("registry.example.com",
    client.WithAuth(authn.Anonymous),
)
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

// Get the reference string used to pull the image
// Requires type assertion to *client.Image
fmt.Printf("Pull reference: %s\n", img.(*client.Image).GetPullReference())
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

### Push an Image Index (Multi-Arch)

Push a manifest list / OCI image index that references platform-specific images:

```go
import v1 "github.com/google/go-containerregistry/pkg/v1"

var idx v1.ImageIndex

err := registryClient.PushIndex(ctx, "v1.0.0", idx)
if err != nil {
    log.Fatal(err)
}
```

### Get Image Digest

Uses HEAD first, falling back to GET if HEAD is unsupported or returns 404:

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

    // Access subject (OCI referrers)
    if subject := manifest.GetSubject(); subject != nil {
        fmt.Printf("Subject: %s\n", subject.GetDigest())
    }
}
```

**`ManifestResult` implementation detail (`client.ManifestResult`):**

- `IsIndex() bool` — convenience check for index manifests
- Raw manifest bytes are lazily decoded on first call to `GetManifest()` or `GetIndexManifest()`
- Calling `GetManifest()` on an index returns `client.ErrIsIndexManifest`
- Calling `GetIndexManifest()` on a non-index returns `client.ErrIsNotIndexManifest`
- `client.NewManifestResultFromBytes(manifestBytes)` constructs a result from raw JSON

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

Uses HEAD first, falling back to GET if HEAD fails (for registries that don't
support HEAD on manifests):

```go
import "github.com/deckhouse/deckhouse/pkg/registry"

err := registryClient.CheckImageExists(ctx, "v1.0.0")
if errors.Is(err, registry.ErrImageNotFound) {
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

## Tag and Lifecycle Operations

### Tag an Image (Retag)

Add a new tag pointing to the same manifest as an existing tag — a single manifest PUT with no layer re-upload:

```go
// Promote v1.0.0 to latest
err := registryClient.TagImage(ctx, "v1.0.0", "latest")
if err != nil {
    log.Fatal(err)
}
```

### Delete a Tag

```go
err := registryClient.DeleteTag(ctx, "v1.0.0")
if err != nil {
    if errors.Is(err, registry.ErrImageNotFound) {
        fmt.Println("Tag does not exist")
    } else {
        log.Fatal(err)
    }
}
```

### Delete by Digest

Delete a manifest by its digest, removing all tags that reference it:

```go
import v1 "github.com/google/go-containerregistry/pkg/v1"

digest, _ := v1.NewHash("sha256:abc123...")

err := registryClient.DeleteByDigest(ctx, digest)
if err != nil {
    if errors.Is(err, registry.ErrImageNotFound) {
        fmt.Println("Manifest does not exist")
    } else {
        log.Fatal(err)
    }
}
```

### Copy an Image

Copy an image between registries without pulling layers locally when possible.
When both source and destination are `*client.Client`, server-side mount is used.
Multi-arch indexes are handled automatically.

```go
sourceClient := client.New("source.example.com",
    client.WithLoginPassword("user", "pass"),
).WithSegment("org", "project")

destClient := client.New("dest.example.com",
    client.WithLoginPassword("user", "pass"),
).WithSegment("mirror", "org", "project")

// Copies image including all layers
err := sourceClient.CopyImage(ctx, "v1.0.0", destClient, "v1.0.0")
if err != nil {
    log.Fatal(err)
}
```

**Fallback behavior:** If the destination is an interface `registry.Client` rather
than a concrete `*client.Client`, the image is pulled and re-pushed via `PushImage`.

## Repository Operations

### List Tags

The `ListTags` method supports server-side pagination for large repositories:

```go
import "github.com/deckhouse/deckhouse/pkg/registry/client"

// List all tags (auto-paginates internally)
tags, err := registryClient.ListTags(ctx)
if err != nil {
    log.Fatal(err)
}

// List first 50 tags (single page)
tags, err := registryClient.ListTags(ctx, client.WithTagsLimit(50))
if err != nil {
    log.Fatal(err)
}

for _, tag := range tags {
    fmt.Printf("Tag: %s\n", tag)
}

// Manual pagination
if len(tags) == 50 {
    nextTags, err := registryClient.ListTags(ctx,
        client.WithTagsLimit(50),
        client.WithTagsLast(tags[len(tags)-1]),
    )
    // Process next page...
}
```

**Available Options:**

| Function | Description |
|---|---|
| `client.WithTagsLimit(n)` | Cap results to n tags (single page) |
| `client.WithTagsLast(tag)` | Continue from a specific tag (pagination cursor) |

**Implementation detail:** When pagination options are set, the client uses direct HTTP
requests (with authenticated transport) against the `/v2/<repo>/tags/list` endpoint,
following `Link` headers for multi-page results. Response bodies are limited to 8 MiB.
Without options, `remote.List()` is used instead.

### List Repositories

The `ListRepositories` method supports server-side pagination:

```go
import "github.com/deckhouse/deckhouse/pkg/registry/client"

// List all repositories
repos, err := registryClient.ListRepositories(ctx)
if err != nil {
    log.Fatal(err)
}

// With pagination
repos, err := registryClient.ListRepositories(ctx, client.WithReposLimit(100))
if err != nil {
    log.Fatal(err)
}

// Continue pagination
if len(repos) == 100 {
    nextRepos, err := registryClient.ListRepositories(ctx,
        client.WithReposLimit(100),
        client.WithReposLast(repos[len(repos)-1]),
    )
    // Process next page...
}
```

**Available Options:**

| Function | Description |
|---|---|
| `client.WithReposLimit(n)` | Cap results to n repos (single page via `CatalogPage`) |
| `client.WithReposLast(repo)` | Continue from a specific repository (pagination cursor) |

**Implementation detail:** Uses `remote.CatalogPage` when pagination options are set,
falls back to `remote.Catalog` otherwise.

## Advanced Usage

### Transport Middlewares

The package supports a composable transport middleware chain for cross-cutting
concerns like metrics, tracing, logging, or rate-limiting:

```go
import (
    "net/http"
    "github.com/deckhouse/deckhouse/pkg/registry/client"
)

// Define a middleware using the TransportMiddleware type
func loggingMiddleware(next http.RoundTripper) http.RoundTripper {
    return client.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
        log.Printf("-> %s %s", req.Method, req.URL)
        resp, err := next.RoundTrip(req)
        if err == nil {
            log.Printf("<- %d %s", resp.StatusCode, req.URL)
        }
        return resp, err
    })
}

func metricsMiddleware(next http.RoundTripper) http.RoundTripper {
    return client.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
        start := time.Now()
        resp, err := next.RoundTrip(req)
        duration := time.Since(start)
        recordMetric(req.Method, req.URL.Path, duration)
        return resp, err
    })
}

// Apply middlewares — first middleware wraps the outermost layer
registryClient := client.New("registry.example.com",
    client.WithMiddleware(metricsMiddleware, loggingMiddleware),
    client.WithAuth(auth),
)
```

**`client.RoundTripperFunc`** is an adapter that allows ordinary functions to be used
as `http.RoundTripper`, similar to `http.HandlerFunc`.

Middlewares can also be set via the `Options` struct:

```go
opts := &client.Options{
    Auth:        auth,
    Middlewares: []client.TransportMiddleware{metricsMiddleware, loggingMiddleware},
}
registryClient := client.NewClientWithOptions("registry.example.com", opts)
```

### Custom Transport

Provide a fully custom `http.RoundTripper`. When set, `CA`, `TLSSkipVerify`,
`Insecure`, and `ProxyURL` transport-level settings are ignored (a warning is logged):

```go
customTransport := &http.Transport{
    // ... your custom settings
}

registryClient := client.New("registry.example.com",
    client.WithCustomTransport(customTransport),
    client.WithAuth(auth),
)
```

### Proxy Configuration

Set an explicit HTTP/HTTPS proxy for registry requests. Overrides any proxy
configured via environment variables (`HTTP_PROXY` / `HTTPS_PROXY`). Pass `nil` to
disable proxying entirely:

```go
import "net/url"

proxyURL, _ := url.Parse("http://proxy.internal:3128")

registryClient := client.New("registry.example.com",
    client.WithProxy(proxyURL),
    client.WithAuth(auth),
)
```

### Platform-Specific Image Retrieval

When working with multi-architecture images, specify the platform to retrieve the
correct variant using the `WithPlatform` option:

```go
import (
    v1 "github.com/google/go-containerregistry/pkg/v1"
    "github.com/deckhouse/deckhouse/pkg/registry/client"
)

platform := &v1.Platform{
    OS:           "linux",
    Architecture: "arm64",
}

img, err := registryClient.GetImage(ctx, "v1.0.0", client.WithPlatform{Platform: platform})
if err != nil {
    log.Fatal(err)
}
```

**Common platforms:**

```go
// Linux AMD64
&v1.Platform{OS: "linux", Architecture: "amd64"}

// Linux ARM64
&v1.Platform{OS: "linux", Architecture: "arm64"}

// Linux ARM v7
&v1.Platform{OS: "linux", Architecture: "arm", Variant: "v7"}
```

**Note**: If no platform is specified, the registry typically returns the manifest for the host's native platform.

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
    time.Sleep(5 * time.Second)
    cancel()
}()

tags, err := registryClient.ListTags(ctx)
if err != nil && ctx.Err() == context.Canceled {
    log.Println("Operation canceled")
}
```

**Note:** The client also supports `Options.Timeout` / `WithTimeout(d)`, which applies
an automatic `context.WithTimeout` wrapper around every operation.

### Concurrent Operations

The client is thread-safe and can be used concurrently:

```go
import "sync"

func processTags(ctx context.Context, c registry.Client, tags []string) {
    var wg sync.WaitGroup

    for _, tag := range tags {
        wg.Add(1)
        go func(t string) {
            defer wg.Done()

            digest, err := c.GetDigest(ctx, t)
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

## Configuration Options

### Options Struct

The `Options` struct provides comprehensive configuration:

```go
type Options struct {
    // Authentication — Auth takes precedence over Keychain.
    // If neither is set, authn.DefaultKeychain is used.
    Auth     authn.Authenticator // Explicit authenticator
    Keychain authn.Keychain      // Custom keychain (alternative to Auth)

    // HTTP / TLS
    Insecure      bool              // Use plain HTTP instead of HTTPS
    TLSSkipVerify bool              // Skip TLS certificate verification
    CA            string            // PEM-encoded custom CA certificate
    Scheme        string            // "http" or "https" (deprecated: prefer Insecure)

    // Transport
    Transport   http.RoundTripper      // Custom transport (overrides CA/TLS/Insecure/Proxy)
    ProxyURL    *url.URL               // Explicit proxy URL (overrides env vars)
    Middlewares []TransportMiddleware   // Transport middleware chain

    // Request behaviour
    UserAgent string        // User-Agent header value
    Timeout   time.Duration // Per-operation timeout (0 = no limit)

    // Logging
    Logger *log.Logger // Custom logger (auto-created as "registry-client" if nil)
}
```

### Functional Options

All functional options are passed to `client.New()`:

| Function | Signature | Description |
|---|---|---|
| `WithAuth` | `(authn.Authenticator)` | Set an explicit authenticator |
| `WithKeychain` | `(authn.Keychain)` | Set a custom keychain |
| `WithLoginPassword` | `(user, pass string)` | Set Basic auth credentials |
| `WithDockercfg` | `(repo, cfg string) (Option, error)` | Parse Docker config JSON |
| `WithInsecure` | `(bool)` | Enable plain HTTP |
| `WithTLSSkipVerify` | `(bool)` | Disable TLS verification |
| `WithCA` | `(string)` | Set a PEM-encoded custom CA certificate |
| `WithUserAgent` | `(string)` | Set the User-Agent header |
| `WithTimeout` | `(time.Duration)` | Set per-operation timeout |
| `WithLogger` | `(*log.Logger)` | Set the logger |
| `WithCustomTransport` | `(http.RoundTripper)` | Set a custom HTTP transport |
| `WithProxy` | `(*url.URL)` | Set an explicit proxy URL |
| `WithMiddleware` | `(...TransportMiddleware)` | Add transport middlewares |
| `WithScheme` | `(string)` | Set URL scheme (**deprecated**: use `WithInsecure`) |

### Transport Constants

The default transport uses these sensible defaults:

| Constant | Value |
|---|---|
| `defaultTimeout` (dial/keep-alive) | 120 s |
| `defaultMaxIdleConns` | 100 |
| `defaultIdleConnTimeout` | 90 s |
| `defaultTLSHandshakeTimeout` | 10 s |
| `defaultExpectContinueTimeout` | 1 s |

### Complete Example

```go
import (
    "net/url"
    "time"

    "github.com/google/go-containerregistry/pkg/authn"

    "github.com/deckhouse/deckhouse/pkg/log"
    "github.com/deckhouse/deckhouse/pkg/registry/client"
)

logger := log.NewLogger().Named("registry")
proxyURL, _ := url.Parse("http://proxy.internal:3128")

// Functional options style (preferred)
registryClient := client.New("registry.example.com",
    client.WithLoginPassword("myuser", "mypassword"),
    client.WithCA(caPEM),
    client.WithTimeout(2*time.Minute),
    client.WithProxy(proxyURL),
    client.WithMiddleware(metricsMiddleware),
    client.WithLogger(logger),
)

// Equivalent using Options struct
auth := authn.FromConfig(authn.AuthConfig{
    Username: "myuser",
    Password: "mypassword",
})

opts := &client.Options{
    Auth:        auth,
    CA:          caPEM,
    Timeout:     2 * time.Minute,
    ProxyURL:    proxyURL,
    Middlewares: []client.TransportMiddleware{metricsMiddleware},
    Logger:      logger,
}

registryClient = client.NewClientWithOptions("registry.example.com", opts)
```

## Error Handling

### Sentinel Errors

| Error | Package | Description |
|---|---|---|
| `ErrImageNotFound` | `registry` and `client` | Image tag or digest does not exist |
| `ErrIsIndexManifest` | `client` | `GetManifest()` called on an index manifest |
| `ErrIsNotIndexManifest` | `client` | `GetIndexManifest()` called on a non-index manifest |

```go
import (
    "errors"

    "github.com/deckhouse/deckhouse/pkg/registry"
)

err := registryClient.CheckImageExists(ctx, "v1.0.0")
if errors.Is(err, registry.ErrImageNotFound) {
    fmt.Println("Image not found")
} else if err != nil {
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

var img registry.Image
var err error

for _, tag := range tags {
    img, err = registryClient.GetImage(ctx, tag)
    if err == nil {
        fmt.Printf("Successfully pulled: %s\n", tag)
        break
    }

    if errors.Is(err, registry.ErrImageNotFound) {
        continue // Try next tag
    }

    log.Fatal(err) // Fatal error
}
```

## Best Practices

### 1. Use Path Segmentation

```go
// Good: Build paths incrementally for flexibility
base := client.New("registry.example.com", opts...)
org := base.WithSegment("myorg")
project := org.WithSegment("myproject")

// Avoid: Hardcoding full paths (less flexible)
fullPath := base.WithSegment("myorg/project") // Treated as single segment
```

### 2. Reuse Client Instances

```go
// Good: Create once, reuse
registryClient := client.New("registry.example.com", opts...)

for _, tag := range tags {
    digest, _ := registryClient.GetDigest(ctx, tag)
    // ...
}

// Avoid: Creating new clients repeatedly
for _, tag := range tags {
    c := client.New("registry.example.com", opts...)
    digest, _ := c.GetDigest(ctx, tag)
}
```

### 3. Always Use Context with Timeout

```go
// Good: Reasonable timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

img, err := registryClient.GetImage(ctx, "v1.0.0")

// Or use client-level timeout
registryClient := client.New("registry.example.com",
    client.WithTimeout(5*time.Minute),
)
```

### 4. Close Readers

```go
// Good: Always close readers
img, _ := registryClient.GetImage(ctx, "v1.0.0")
reader := img.Extract()
defer reader.Close()
```

### 5. Handle Errors Appropriately

```go
// Good: Distinguish between different errors
err := registryClient.CheckImageExists(ctx, "v1.0.0")
if errors.Is(err, registry.ErrImageNotFound) {
    useDefaultImage()
} else if err != nil {
    log.Fatal(err)
}
```

### 6. Use CopyImage for Mirroring

```go
// Good: Server-side copy avoids pulling layers locally
err := sourceClient.CopyImage(ctx, "v1.0.0", destClient, "v1.0.0")

// Avoid: Manual pull + push (pulls all layers locally)
img, _ := sourceClient.GetImage(ctx, "v1.0.0")
destClient.PushImage(ctx, "v1.0.0", img)
```

### 7. Use TagImage for Promotion

```go
// Good: Single manifest PUT, no layer upload
err := registryClient.TagImage(ctx, "v1.0.0", "latest")

// Avoid: Pull + push just to retag
img, _ := registryClient.GetImage(ctx, "v1.0.0")
registryClient.PushImage(ctx, "latest", img)
```

## Examples

### Mirror Images Between Registries

```go
func mirrorImage(ctx context.Context, source, target registry.Client, tag string) error {
    return source.CopyImage(ctx, tag, target, tag)
}

// Usage
sourceClient := client.New("source.example.com", sourceOpts...).
    WithSegment("org", "project")

targetClient := client.New("target.example.com", targetOpts...).
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

    // Copy missing tags
    for _, tag := range sourceTags {
        if !targetSet[tag] {
            if err := source.CopyImage(ctx, tag, target, tag); err != nil {
                log.Printf("Failed to copy %s: %v", tag, err)
                continue
            }
            fmt.Printf("Copied: %s\n", tag)
        }
    }

    return nil
}
```

### Inspect Image Metadata

```go
func inspectImage(ctx context.Context, c registry.Client, tag string) error {
    config, err := c.GetImageConfig(ctx, tag)
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

### Clean Up Old Tags

```go
func cleanupOldTags(ctx context.Context, c registry.Client, keep int) error {
    tags, err := c.ListTags(ctx)
    if err != nil {
        return err
    }

    if len(tags) <= keep {
        return nil
    }

    // Delete oldest tags (assumes lexicographic ordering)
    for _, tag := range tags[:len(tags)-keep] {
        if err := c.DeleteTag(ctx, tag); err != nil {
            if errors.Is(err, registry.ErrImageNotFound) {
                continue // Already deleted
            }
            log.Printf("Failed to delete %s: %v", tag, err)
        }
    }

    return nil
}
```

### Promote Image Between Environments

```go
func promoteImage(ctx context.Context, c registry.Client, srcTag, envTag string) error {
    // Verify source exists
    if err := c.CheckImageExists(ctx, srcTag); err != nil {
        return fmt.Errorf("source image %s: %w", srcTag, err)
    }

    // Retag without re-uploading
    return c.TagImage(ctx, srcTag, envTag)
}

// Usage
err := promoteImage(ctx, registryClient, "v1.2.3", "production")
```

## Troubleshooting

### Authentication Failures (401 Unauthorized)

**Problem**: Getting 401 errors when accessing registry.

**Solution**: Verify credentials and authentication method:

```go
auth := authn.FromConfig(authn.AuthConfig{
    Username: "correct-username",
    Password: "correct-password",
})

registryClient := client.New("registry.example.com",
    client.WithAuth(auth),
    client.WithLogger(logger), // Enable logging
)
```

### TLS Certificate Verification Errors

**Problem**: Certificate verification failures.

**Solution**: For development/testing (not production):

```go
registryClient := client.New("registry.example.com",
    client.WithTLSSkipVerify(true),
    client.WithLogger(logger),
)
```

**Better Solution**: Provide the CA certificate:

```go
registryClient := client.New("registry.example.com",
    client.WithCA(caPEM),
)
```

### Connection Timeouts

**Problem**: Operations hanging or timing out.

**Solution**: Use appropriate timeouts:

```go
// Client-level timeout
registryClient := client.New("registry.example.com",
    client.WithTimeout(10*time.Minute),
)

// Or context-level timeout for large images
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
defer cancel()

img, err := registryClient.GetImage(ctx, "large-image:latest")
```

### Image Not Found Errors

**Problem**: Cannot find expected images.

**Solution**: Check image existence before operations:

```go
err := registryClient.CheckImageExists(ctx, "v1.0.0")
if errors.Is(err, registry.ErrImageNotFound) {
    log.Println("Image doesn't exist, check tag name")
} else if err != nil {
    log.Println("Error checking image:", err)
}
```

### Registry Not Responding

**Problem**: Cannot connect to registry.

**Solution**: For HTTP registries (not HTTPS):

```go
registryClient := client.New("registry.example.com",
    client.WithInsecure(true),
    client.WithLogger(logger),
)
```

### Proxy Issues

**Problem**: Registry behind a corporate proxy.

**Solution**: Configure proxy explicitly:

```go
proxyURL, _ := url.Parse("http://proxy.internal:3128")

registryClient := client.New("registry.example.com",
    client.WithProxy(proxyURL),
)
```

### Debug Logging

Enable detailed logging to diagnose issues:

```go
import "log/slog"

logger := log.NewLogger(
    log.WithLevel(slog.LevelDebug),
).Named("registry-debug")

registryClient := client.New("registry.example.com",
    client.WithLogger(logger),
)

// All operations will log detailed information including
// registry host, segments, tags, and operation results
```

## License

Apache License 2.0

## Contributing

Contributions are welcome! Please ensure:
- Code follows existing patterns (interfaces in `registry`, implementations in `client`)
- All operations are thread-safe
- New options follow the functional option pattern (`With*` functions)
- Tests are included
- Documentation is updated
