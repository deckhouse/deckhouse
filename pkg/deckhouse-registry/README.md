# deckhouse-registry

`github.com/deckhouse/deckhouse/pkg/deckhouse-registry` (package `dhregistry`) models the artifact registry structure of the Deckhouse ecosystem as a typed tree, so callers never build registry paths by hand.

It is a separate Go module layered on top of [`pkg/registry`](../registry): that module owns the transport (auth, TLS, pagination, `remote.*` calls), this one owns the layout.

## Packages

The root package holds the vocabulary shared across the library — `Edition`, the sentinel errors — and assembles the tree. Below it, one package per sub-tree, each declaring the path segments it owns.

```
dhregistry          Registry, Edition, errors — the library's vocabulary
├── service         BasicService: one repository, the node every sub-tree embeds
├── bundle          repositories holding full images; adds Digests
├── release         release-image reader, and the Channel vocabulary
├── definition      module.yaml and package.yaml mappings
├── digests         images_digests.json decoder, used by bundle
├── extra           the /extra/<name> catalog, shared by module and packages
├── deckhouse       <root>/<edition>, /release-channel, /install, /install-standalone
├── module          <root>/<edition>/modules[/<module>[/release|/extra/<extra>]]
├── packages        <root>/<edition>/packages[/<package>[/version|/extra/<extra>]]
├── security        <root>/<edition>/security/<name>
├── deckhouse-cli   <root>/<edition>/deckhouse-cli[/version|/plugins/<plugin>[/version]]  (package cli)
└── internal/
    └── cache       memoizes the services built for dynamic segments
```

The dependency graph is a DAG: `digests` and `definition` are the leaves, `bundle` sits on `digests` and `service`, `release` and `extra` on `service`, the five sub-tree packages on top of those, and only the root imports all of them.

Directory names match the registry segment each package owns, so `deckhouse-cli/` holds `package cli` — the segment is not a valid Go identifier. Import it as `cli "…/deckhouse-registry/deckhouse-cli"`.

Configuring a registry and handling its results needs just the root import: `Edition`, `Channel`, the decoded-metadata types, the sentinel errors and `ValidateName` all live or are re-exported there. Import a sub-package when you need to name one of its types (`*module.Service`, `*release.Service`) in your own signatures.

## Install

```shell
go get github.com/deckhouse/deckhouse/pkg/deckhouse-registry
```

## Usage

```go
import (
	"github.com/deckhouse/deckhouse/pkg/registry/client"

	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
)

// The client points at the registry root — the path above the edition.
root := client.New("registry.deckhouse.io",
	client.WithAuth(auth),
).WithSegment("deckhouse")

reg := dhregistry.New(root,
	dhregistry.WithEdition(dhregistry.FEEdition),
	dhregistry.WithLogger(logger),
)

// Resolve a release channel to a concrete version.
version, err := reg.Deckhouse().Releases().Version(ctx, "stable")

// Read a module's release metadata.
meta, err := reg.Modules().Module("stronghold").Releases().Metadata(ctx, "alpha")

// Enumerate the module catalog.
modules, err := reg.Modules().List(ctx)

// Pull an auxiliary image of a module.
img, err := reg.Modules().Module("neuvector").Extra().Image("scanner").GetImage(ctx, "3")
```

When the edition is not known up front, let it be detected from the path:

```go
reg := dhregistry.NewForPath(root)          // reads the edition off the last path segment
root, edition := dhregistry.SplitEdition(s) // or split a configured path yourself
```

`NewForPath` cannot un-scope a client, so when it detects an edition the client is taken to be edition-scoped already. Use `New` with an explicitly root-scoped client when the edition-independent installer is needed.

## Structure

Every node of the tree embeds a `*BasicService` over exactly one OCI repository, and exposes `Path()`, `Ref(tag)`, `GetImage`, `GetDigest`, `GetManifest`, `GetImageConfig`, `Exists`, `ListTags` and `ListRepositories`. Because `Path` and `Ref` need no registry access, the tree doubles as a pure path builder.

| Accessor | Type | Repository |
|---|---|---|
| `Deckhouse()` | `*deckhouse.Service` | `<root>/<edition>` |
| `Deckhouse().Releases()` | `*release.Service` | `<root>/<edition>/release-channel` |
| `Deckhouse().Install()` | `*service.BasicService` | `<root>/<edition>/install` |
| `Deckhouse().InstallStandalone()` | `*service.BasicService` | `<root>/<edition>/install-standalone` |
| `Security()` | `*security.Catalog` | `<root>/<edition>/security` |
| `Security().Image(name)` | `*service.BasicService` | `<root>/<edition>/security/<name>` |
| `Modules()` | `*module.Catalog` | `<root>/<edition>/modules` |
| `Modules().Module(m)` | `*module.Service` | `<root>/<edition>/modules/<m>` |
| `Modules().Module(m).Releases()` | `*release.Service` | `<root>/<edition>/modules/<m>/release` |
| `Modules().Module(m).Extra()` | `*extra.Catalog` | `<root>/<edition>/modules/<m>/extra` |
| `Modules().Module(m).Extra().Image(e)` | `*service.BasicService` | `<root>/<edition>/modules/<m>/extra/<e>` |
| `Packages()` | `*packages.Catalog` | `<root>/<edition>/packages` |
| `Packages().Package(p)` | `*packages.Service` | `<root>/<edition>/packages/<p>` |
| `Packages().Package(p).Versions()` | `*release.Service` | `<root>/<edition>/packages/<p>/version` |
| `Packages().Package(p).Extra()` | `*extra.Catalog` | `<root>/<edition>/packages/<p>/extra` |
| `Packages().Package(p).Extra().Image(e)` | `*service.BasicService` | `<root>/<edition>/packages/<p>/extra/<e>` |
| `CLI()` | `*cli.Service` | `<root>/<edition>/deckhouse-cli` |
| `CLI().Versions()` | `*release.Service` | `<root>/<edition>/deckhouse-cli/version` |
| `CLI().Plugins()` | `*cli.PluginCatalog` | `<root>/<edition>/deckhouse-cli/plugins` |
| `CLI().Plugins().Plugin(p)` | `*cli.Plugin` | `<root>/<edition>/deckhouse-cli/plugins/<p>` |
| `CLI().Plugins().Plugin(p).Versions()` | `*release.Service` | `<root>/<edition>/deckhouse-cli/plugins/<p>/version` |
| `Installer()` | `*service.BasicService` | `<root>/installer` |

The installer is the only edition-independent node; everything else hangs off the edition sub-path. Under `NoEdition` the edition sub-path disappears and `Root()` equals `EditionRoot()`, which is how dev roots such as `dev-registry.deckhouse.io/sys/deckhouse-oss` are addressed.

### Catalogs

`module.Catalog`, `packages.Catalog`, `cli.PluginCatalog` and `extra.Catalog` are *catalogs*: repositories whose tags are names rather than versions. Listing a catalog enumerates what it publishes, and each tag points at a scratch image.

```go
reg.Modules().List(ctx)               // ["stronghold", "neuvector", ...]
reg.Modules().Ref("stronghold")       // registry.deckhouse.io/deckhouse/fe/modules:stronghold
```

### Releases

A release image is a scratch image carrying only metadata. Every sub-tree publishes them, and their tags are either channel names (`alpha`, `beta`, `early-access`, `stable`, `rock-solid`, `lts`) or concrete versions — so one service answers both "what is on stable" and "what does v1.73.0 declare".

What the image carries differs by kind, so each sub-tree has its own release type:

| Service | Repository | version.json | Manifest |
|---|---|---|---|
| `deckhouse.ReleaseService` | `<edition>/release-channel` | rollout fields populated | none |
| `module.ReleaseService` | `modules/<m>/release` | version + suspend only | `module.yaml` |
| `packages.VersionService` | `packages/<p>/version` | version + suspend only | `package.yaml` |

```go
// Deckhouse release: version.json drives how the upgrade is staged.
meta, err := reg.Deckhouse().Releases().Metadata(ctx, "stable")
meta.Version              // "v1.73.0"
meta.Suspend              // must not be rolled out
meta.Requirements         // {"k8s": ">= 1.27", ...}
meta.Disruptions          // {"1.73": ["ingressNginx"]}
meta.Canary["stable"]     // rollout waves and interval

// Module release: same version.json getters, plus its manifest.
rel := reg.Modules().Module("stronghold").Releases()
version, err := rel.Version(ctx, "alpha")   // resolve a channel to a version
def, err := rel.Definition(ctx, "alpha")    // decoded module.yaml
def.Weight; def.Requirements.Deckhouse      // ">= 1.70"

// Package release: the v2 counterpart.
pkg, err := reg.Packages().Package("elma").Versions().Definition(ctx, "v1.0.1")
pkg.IsModule(); pkg.IsApplication()         // one schema, two package types
pkg.Requirements.Deckhouse.Constraint       // ">= 1.70"
```

The two manifests are not two spellings of one schema, which is why `definition` keeps them apart. Requirements differ most: `module.yaml` states them as bare version ranges and a flat module map, `package.yaml` wraps each in a constraint object and splits module dependencies into mandatory/conditional/anyOf/noneOf buckets. Both are mapped as written.

`definition` decodes and nothing more — validating requirement buckets, resolving semver constraints and projecting onto cluster resources stay with the consumer.

Common to all three: `Metadata` (decoded version.json), `Version` (resolve a channel to a version), `Changelog` and `Channels`. A missing manifest or changelog gives `ErrFileNotFound` — older module releases legitimately ship none, and the manifest has to be read from the module image instead.

Every mapped file is returned decoded, never as bytes. Each result keeps the undecoded original on `Raw` for consumers applying their own schema, and `File`/`Files` read anything the library does not map.

Each getter pulls the image again. When you need more than one file, `Files(ctx, tag, names...)` reads them in a single pass:

```go
files, err := rel.Files(ctx, "alpha", release.VersionFile, release.ModuleFile, release.ChangelogFile)
```

### Bundles and image digests

A *bundle* is a full image — one shipping the artifact itself, as opposed to a release image or a scratch catalog entry. Six repositories hold them, and `bundle.Service` is the type they share: `Digests` reads `images_digests.json`, mapping every image the bundle contains to its digest.

Neither the location nor the shape is uniform. Both were verified against the live registry at v1.76.6:

| Accessor | Repository | File inside | Shape |
|---|---|---|---|
| `Deckhouse()` | `<edition>` | `deckhouse/modules/images_digests.json` | nested |
| `Deckhouse().Install()` | `<edition>/install` | `deckhouse/candi/images_digests.json` | nested |
| `Deckhouse().InstallStandalone()` | `<edition>/install-standalone` | `deckhouse/candi/…` | nested |
| `Installer()` | `<root>/installer` | `deckhouse/candi/…` | nested |
| `Modules().Module(m)` | `modules/<m>` | `images_digests.json` | flat |
| `Packages().Package(p)` | `packages/<p>` | `images_digests.json` | flat |

The shape follows what the bundle contains, not what kind of bundle it is: an image carrying the images of many modules keys them by module, one carrying only its own does not. `Digests` detects which:

```go
// The Deckhouse image bundles every module of its edition, so it nests.
d, err := reg.Deckhouse().Digests(ctx, "v1.73.0")
d.IsNested()                                // true
d.Modules()                                 // ["ingressNginx", "userAuthn", ...]
d.Lookup("ingressNginx", "controller")      // digest, ok

// A module or package bundles only its own images, so it is flat.
d, err = reg.Modules().Module("stronghold").Digests(ctx, "v1.0.1")
d.Images                                    // {"controller": "sha256:...", ...}
d.Lookup("", "controller")
```

Keys are lowerCamelCase at both levels — `controlPlaneManager`, `ingressNginx` — not the kebab-case a module is known by elsewhere, so `Lookup("control-plane-manager", …)` misses. Values are full `sha256:` digests.

Only the six repositories above are `bundle.Service`; the rest of the tree is a plain `BasicService` with no `Digests` at all, so a release repository or catalog cannot be asked for digests by mistake. Each bundle is constructed with its own path (`bundle.ModulesPath`, `bundle.CandiPath`, `bundle.RootPath`), and reading the wrong one misses rather than silently succeeding.

Reading a bundle means pulling and flattening a full image, which for the Deckhouse image is hundreds of megabytes.

## Errors

`ErrImageNotFound` is re-exported from `pkg/registry`; use `dhregistry.IsNotFound(err)` or `errors.Is`. `Exists` folds a missing image into `(false, nil)` and reports anything else as an error.

The rest are re-exported at the root too: `ErrNoVersionMetadata` (release image without version.json), `ErrFileNotFound` (release image without a requested metadata file) and `ErrNoDigests` (bundle without images_digests.json).

The accessors for dynamic segments (module, package, plugin, extra, security image) do not validate their argument. An empty or malformed name collapses out of the path and silently addresses the parent repository — `Module("")` resolves to the module catalog, not to an error. Callers taking names from user input or a CR should check them first:

```go
if err := dhregistry.ValidateName(name); err != nil { /* reject before use */ }
```

## Concurrency

`Registry` and every service in the tree are safe for concurrent use. Repeated lookups of the same dynamic name return the same service instance.
