# fake — In-Memory Registry Client for Tests

Package `fake` provides a fully in-memory implementation of the `registry.Client`
interface. It is designed to replace real HTTP-based registry calls in unit and
integration tests so they run fast, deterministically, and without network
access.

## Core Types

| Type | Purpose |
|------|---------|
| `Registry` | In-memory OCI registry scoped to a single host (e.g. `gcr.io`). Stores images organized by repository path and tag. Thread-safe. |
| `Client` | Implements `registry.Client`. Routes every call (`GetImage`, `PushImage`, `ListTags`, …) to the correct `Registry` based on the path built with `WithSegment`. |
| `ImageBuilder` | Fluent builder that assembles a `v1.Image` from plain-text files and OCI metadata (labels, platform, env, …). |

## Quick Start

```go
// 1. Create a registry for a host.
reg := fake.NewRegistry("registry.example.com")

// 2. Build an image with the files and labels your code expects.
img := fake.NewImageBuilder().
    WithFile("version.json", `{"version":"v1.65.0"}`).
    WithLabel("org.opencontainers.image.version", "v1.65.0").
    MustBuild()

// 3. Add the image to a repository path + tag.
reg.MustAddImage("deckhouse/ee", "v1.65.0", img)

// 4. Create a client backed by this registry.
client := fake.NewClient(reg)

// 5. Scope the client to a repository and use it like the real thing.
scoped := client.WithSegment("deckhouse").WithSegment("ee")
tags, _ := scoped.ListTags(ctx)               // ["v1.65.0"]
got, _  := scoped.GetImage(ctx, "v1.65.0")    // returns the fake image
```

### Multiple Registries

`NewClient` accepts multiple registries. The client inspects the path
accumulated by `WithSegment` to dispatch each call to the right one:

```go
src := fake.NewRegistry("src.example.com")
dst := fake.NewRegistry("dst.example.com")
client := fake.NewClient(src, dst)

// Push to dst via its host path.
client.WithSegment("dst.example.com", "repo").PushImage(ctx, "latest", img)
```

### Building Images

`ImageBuilder` supports the most common image properties:

```go
img, err := fake.NewImageBuilder().
    WithFile("app.yaml", configYAML).       // embed arbitrary files
    WithLabel("version", "1.0").            // OCI / Docker labels
    WithPlatform("linux", "arm64").         // OS + architecture
    WithVariant("v8").                      // platform variant
    WithEnv("FOO=bar").                     // environment variables
    WithEntrypoint("/bin/app").             // entrypoint
    WithCmd("--serve").                     // default command
    WithWorkingDir("/app").                 // working directory
    Build()
```

## Deckhouse Stub — Pre-built Registry Fixture

```go
package fake

import (
	"fmt"

	v1 "github.com/google/go-containerregistry/pkg/v1"

	dkpclient "github.com/deckhouse/deckhouse/pkg/registry"
)

// defaultSource is the registry root used by NewRegistryClientFake.
const defaultSource = "registry.deckhouse.ru/deckhouse/fe"

// releaseChannelData maps a release-channel tag to the version its image
// carries in version.json.
var releaseChannelData = map[string]string{
	"alpha":        "v1.72.10",
	"beta":         "v1.71.0",
	"early-access": "v1.70.0",
	"stable":       "v1.69.0",
	"rock-solid":   "v1.68.0",
}

// changelogYAML is the sample changelog file embedded in every fake image.
const changelogYAML = `candi:
  fixes:
  - summary: "Fix deckhouse containerd start after installing new containerd-deckhouse package."
    pull_request: "https://github.com/deckhouse/deckhouse/pull/6329"
`

// imagesDigestsJSON is the sample images-tags file embedded in fake version images.
const imagesDigestsJSON = `{}`

// NewRegistryClientFake creates a [dkpclient.Client] pre-populated with
// Deckhouse-shaped registry data that mirrors the structure expected by the
// platform test suite.
//
// The fake exposes a registry at [defaultSource]
// ("registry.deckhouse.ru/deckhouse/fe") with the following structure:
//
//   - root repository (empty path): tags alpha, beta, early-access, stable,
//     rock-solid, v1.72.10, v1.71.0, v1.70.0, v1.69.0, v1.68.0, pr12345.
//
//   - "release-channel" repository: tags alpha, beta, early-access, stable,
//     rock-solid.  Each image carries version.json with the channel's current
//     version (e.g. alpha → v1.72.10).
//
//   - "install" and "install-standalone" repositories: same tags as root.
func NewRegistryClientFake() dkpclient.Client {
	reg := NewRegistry(defaultSource)

	// ---- release-channel repository ----
	for channel, version := range releaseChannelData {
		img := releaseChannelImage(version)
		reg.MustAddImage("release-channel", channel, img)
		// Version-tagged release-channel images are required by non-DryRun full-discovery pull.
		reg.MustAddImage("release-channel", version, img)
	}

	// ---- root-level and installer repositories ----
	rootTags := []struct {
		tag     string
		version string
	}{
		{"alpha", "v1.72.10"},
		{"beta", "v1.71.0"},
		{"early-access", "v1.70.0"},
		{"stable", "v1.69.0"},
		{"rock-solid", "v1.68.0"},
		{"v1.72.10", "v1.72.10"},
		{"v1.71.0", "v1.71.0"},
		{"v1.70.0", "v1.70.0"},
		{"v1.69.0", "v1.69.0"},
		{"v1.68.0", "v1.68.0"},
		{"pr12345", "dev"}, // custom non-semver tag
	}

	for _, rt := range rootTags {
		img := platformImage(rt.version)
		reg.MustAddImage("", rt.tag, img)
		reg.MustAddImage("install", rt.tag, img)
		reg.MustAddImage("install-standalone", rt.tag, img)
	}

	return NewClient(reg)
}

// platformImage creates a fake v1.Image for the root (edition) repository
// containing the files that the deckhouse platform service reads during
// version discovery.
func platformImage(version string) v1.Image {
	return NewImageBuilder().
		WithFile("version.json", fmt.Sprintf(`{"version":%q}`, version)).
		WithFile("changelog.yaml", changelogYAML).
		WithFile("deckhouse/candi/images_digests.json", imagesDigestsJSON).
		WithLabel("org.opencontainers.image.version", version).
		MustBuild()
}

// releaseChannelImage creates a fake v1.Image for the release-channel
// repository containing version.json that DeckhouseReleaseService reads.
func releaseChannelImage(version string) v1.Image {
	return NewImageBuilder().
		WithFile("version.json", fmt.Sprintf(`{"version":%q}`, version)).
		MustBuild()
}
```

`NewRegistryClientFake()` returns a `registry.Client` pre-populated with
the repository layout that Deckhouse platform services expect. Use it when
your test exercises code that reads release channels, version images, or
installer tags and you don't need to customize the registry content.

```go
client := fake.NewRegistryClientFake()
```

The stub creates a single registry at `registry.deckhouse.ru/deckhouse/fe`
with the following structure:

```
registry.deckhouse.ru/deckhouse/fe
├── (root)                     # tags: alpha, beta, early-access, stable, rock-solid,
│                              #        v1.72.10, v1.71.0, v1.70.0, v1.69.0, v1.68.0, pr12345
├── release-channel/           # tags: alpha → v1.72.10, beta → v1.71.0,
│                              #        early-access → v1.70.0, stable → v1.69.0,
│                              #        rock-solid → v1.68.0
├── install/                   # same tags as root
└── install-standalone/        # same tags as root
```

Each image carries the files that the real pull/discovery logic reads:

- `version.json` — e.g. `{"version":"v1.72.10"}`
- `changelog.yaml` — sample changelog entry
- `deckhouse/candi/images_digests.json` — empty JSON object (root images only)

The stub is a ready-made example of how to model a realistic Deckhouse
registry layout with the fake package. Copy and adapt
`NewRegistryClientFake()` in `deckhouse_stub.go` when you need a different
edition, extra modules, or custom version sets.
