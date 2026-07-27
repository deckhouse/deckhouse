# crd-enricher

`crd-enricher` post-processes CustomResourceDefinition manifests rendered by
[controller-gen](https://book.kubebuilder.io/reference/controller-gen) (kubebuilder)
and injects custom schema fields that controller-gen cannot emit on its own —
`x-doc-examples`, `x-doc-default`, `x-doc-deprecated`, `x-kubernetes-sensitive-data`
and a handful of CRD-level normalizations.

It reads kubebuilder-style markers placed next to your Go API structs and writes
the corresponding keys into the matching nodes of the already generated
`openAPIV3Schema`, editing the CRD YAML files **in place**.

- [Why it exists](#why-it-exists)
- [The `x-doc-*` fields and their purpose](#the-x-doc--fields-and-their-purpose)
- [Quick start: add it to your pipeline](#quick-start-add-it-to-your-pipeline)
- [Marker reference](#marker-reference)
- [Automatic example generation](#automatic-example-generation)
- [CLI reference](#cli-reference)
- [How it works](#how-it-works)
- [Warnings and gotchas](#warnings-and-gotchas)

## Why it exists

Deckhouse renders its user-facing API documentation from CRD OpenAPI schemas.
The documentation site (docs-builder) understands a set of Deckhouse-specific
schema extensions prefixed with `x-doc-`. controller-gen only emits the standard
kubebuilder markers (`description`, `default`, `enum`, `pattern`, …), so there is
no native way to attach these documentation fields, provide curated examples, or
apply the small schema normalizations the hand-curated Deckhouse CRDs expect.

`crd-enricher` closes that gap. You annotate the Go structs once, next to the
kubebuilder markers you already write, and the enricher folds the extra fields
into the generated CRDs on every regeneration — no hand-editing of generated
YAML.

```
                    controller-gen                     crd-enricher
Go API structs ───────────────────────►  CRD YAML  ──────────────────────►  enriched CRD YAML
(kubebuilder +      (standard schema)    (bin/crd/...)   (reads the same       (+ x-doc-*, +
 crd-enricher                                            Go structs for         x-kubernetes-*,
 markers)                                                its own markers)        normalized shape)
```

## The `x-doc-*` fields and their purpose

These fields are consumed by the Deckhouse documentation renderer
(`docs/site/backends/docs-builder-template/.../openapi/`) to produce the
per-field reference pages on the documentation site. They have no effect on the
Kubernetes apiserver — they are purely documentation metadata — with the single
exception of `x-kubernetes-sensitive-data`, which the apiserver acts on.

| Field | Source marker | Purpose |
| --- | --- | --- |
| `x-doc-examples` | `deckhouse:documentation:examples` | Sample values shown in the docs for a field, and the assembled "example resource" block. A list; the marker may be repeated. |
| `x-doc-default` | `deckhouse:documentation:default` | The **documented** default value, shown in the docs when the real default is computed at runtime and cannot be expressed as a `kubebuilder:default`. |
| `x-doc-deprecated` | `deckhouse:documentation:deprecated` | Marks a field as deprecated in the docs (renders a deprecation badge). |
| `x-kubernetes-sensitive-data` | `deckhouse:sensitive-data` | **Behavioral**, not documentation. Tells the apiserver's `CRDSensitiveData` feature to encrypt the value in etcd, filter it by RBAC and mask it in audit logs. |
| *(arbitrary standard field)* | `raw:<key>` | Injects a plain schema field controller-gen cannot produce for a given Go type (e.g. `pattern` on a `metav1.Duration`, or an overridden `description`). |

Why not just use `kubebuilder:default` / native `example`?

- **`x-doc-default` vs `kubebuilder:default`** — a `kubebuilder:default` is applied
  by the apiserver and mutates stored objects. Many Deckhouse fields have a
  default that is applied by a controller at runtime (e.g. `scanInterval: 3m`),
  not by the apiserver. You want the docs to say "defaults to `3m`" without the
  apiserver actually writing `3m` into every object. `x-doc-default` documents the
  value without changing admission behavior.
- **`x-doc-examples`** — controller-gen has no marker for examples at all. The
  enricher both accepts explicit examples and synthesizes a complete example
  resource for the CRD root (see [Automatic example generation](#automatic-example-generation)).

## Quick start: add it to your pipeline

The tool mirrors the controller-gen invocation contract so it can be dropped in
right after CRD generation, reusing the same `paths=` argument.

### 1. Install the binary

```bash
go install github.com/deckhouse/deckhouse/pkg/crd-enricher/cmd/crd-enricher@latest
```

### 2. Run it after controller-gen

```bash
# Step 1 — generate CRDs the usual way
controller-gen crd paths="./pkg/apis/..." output:crd:artifacts:config=bin/crd/bases

# Step 2 — enrich them in place, pointing at the SAME Go packages
crd-enricher paths="./pkg/apis/..." crds=bin/crd/bases
```

`paths=` selects the Go packages that hold the API structs (the source of the
markers); `crds=` points at the directory of CRD YAML files produced by
controller-gen. The files are edited in place, so re-running is idempotent — a
CRD with no markers is re-encoded byte-for-byte and left untouched.

### 3. Wire it into a Makefile

This is exactly how the Deckhouse repository does it (see the root `Makefile`,
targets `generate-crds` → `enrich-crds`). A self-contained version:

```makefile
LOCALBIN         ?= $(shell pwd)/bin
CONTROLLER_GEN   ?= $(LOCALBIN)/controller-gen
CRD_ENRICHER     ?= $(LOCALBIN)/crd-enricher

CONTROLLER_TOOLS_VERSION ?= v0.19.0
CRD_ENRICHER_VERSION     ?= v0.0.1

API_PATHS ?= ./pkg/apis/...
CRD_DIR   ?= $(CURDIR)/bin/crd/bases

# Install crd-enricher on demand.
$(CRD_ENRICHER): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install \
		github.com/deckhouse/deckhouse/pkg/crd-enricher/cmd/crd-enricher@$(CRD_ENRICHER_VERSION)

.PHONY: generate-crds
generate-crds: $(CONTROLLER_GEN)
	@rm -rf $(CRD_DIR)
	$(CONTROLLER_GEN) crd paths="$(API_PATHS)" output:crd:artifacts:config=$(CRD_DIR)

.PHONY: enrich-crds
enrich-crds: generate-crds $(CRD_ENRICHER)
	@echo "Enriching CRDs with custom x-doc-* fields..."
	$(CRD_ENRICHER) \
		paths="$(API_PATHS)" \
		crds=$(CRD_DIR) \
		dir=$(CURDIR)
```

Then `make enrich-crds` regenerates and enriches in one step. The key points:

- `enrich-crds` **depends on** `generate-crds`, so the CRDs always exist and are
  fresh before enrichment.
- The `paths=` value is shared between both tools — the enricher reads the same
  Go packages to find its markers.
- `dir=$(CURDIR)` sets the working directory used to resolve the package
  patterns, so the target works regardless of where `make` is invoked from.

## Marker reference

Markers are ordinary Go comments beginning with `+`, exactly like kubebuilder
markers. Every enricher marker is namespaced with the canonical `crd-enricher:`
prefix. There is **no bare or legacy form** — the prefix is always required.

The value after `=` is parsed as **YAML**, so scalars, lists and maps all work.

```
+crd-enricher:deckhouse:documentation:<entity>[=<value>]   # documentation fields
+crd-enricher:deckhouse:sensitive-data                     # sensitive field flag
+crd-enricher:raw:<key>[=<value>]                          # raw schema injection
+crd-enricher:crd:<key>[=<value>]                          # CRD-level setting (type-level only)
```

Markers can be attached to **struct fields** and to **struct types**. A
type-level marker applies to the schema node of that type (for the root type,
that is `openAPIV3Schema`).

### `examples` — sample values

Rendered as `x-doc-examples`. The marker may be **repeated**; each value is
collected into a list, and a value that is itself a YAML list is flattened in.

```go
type ModuleSourceSpec struct {
	// Interval for registry scan.
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern=`^(\d+h)?(\d+m)?(\d+s)?$`
	// +crd-enricher:deckhouse:documentation:default=3m
	// +crd-enricher:deckhouse:documentation:examples=5m
	// +crd-enricher:deckhouse:documentation:examples=1h
	// +crd-enricher:deckhouse:documentation:examples=6h30m
	ScanInterval *metav1.Duration `json:"scanInterval,omitempty"`
}
```

produces on the `scanInterval` schema node:

```yaml
scanInterval:
  type: string
  pattern: ^(\d+h)?(\d+m)?(\d+s)?$
  x-doc-default: 3m
  x-doc-examples:
    - 5m
    - 1h
    - 6h30m
```

A single list marker is equivalent (and common for enums):

```go
// +kubebuilder:validation:Enum=HTTP;HTTPS
// +crd-enricher:deckhouse:documentation:examples=[HTTP, HTTPS]
Scheme string `json:"scheme,omitempty"`
```

A type-level example on the root struct supplies a complete resource example
(overriding the auto-generated one):

```go
// +kubebuilder:object:root=true
// +crd-enricher:deckhouse:documentation:examples={apiVersion: deckhouse.io/v1alpha1, kind: ModuleConfig, metadata: {name: module-1}, spec: {enabled: true, settings: {}, version: 1}}
type ModuleConfig struct { ... }
```

### `default` — documented default

Rendered as `x-doc-default`. Use it for defaults applied at runtime rather than
by the apiserver (where a real `kubebuilder:default` would be wrong).

```go
// +crd-enricher:deckhouse:documentation:default=6h
ScanInterval *metav1.Duration `json:"scanInterval,omitempty"`
```

→ `x-doc-default: 6h`

### `deprecated` — deprecation flag

A value-less flag. Rendered as `x-doc-deprecated: true`.

```go
// Desirable default release channel for modules in the current source.
// +crd-enricher:deckhouse:documentation:deprecated=true
ReleaseChannel string `json:"releaseChannel,omitempty"`
```

→ `x-doc-deprecated: true`

(Any value-less documentation entity becomes a boolean `x-doc-<entity>`; any
valued one stores its parsed YAML value.)

### `sensitive-data` — mark a field as sensitive

Rendered as `x-kubernetes-sensitive-data: true`. This is a **behavioral** flag:
the apiserver's `CRDSensitiveData` feature encrypts the value in etcd, filters it
by RBAC and masks it in audit logs. Place it on a field, or on an object/array
node to mark the whole subtree. **It must not be placed on the root type** — the
enricher drops it there with a warning, because the root also covers the system
`apiVersion`/`kind`/`metadata` fields the apiserver cannot encrypt.

```go
type PackageRepositorySpecRegistry struct {
	// Container registry access token in Base64.
	// +crd-enricher:deckhouse:sensitive-data
	// +crd-enricher:deckhouse:documentation:examples=<base64 encoded credentials>
	DockerCFG string `json:"dockerCfg,omitempty"`

	// Password for authenticating to the container registry.
	// +crd-enricher:deckhouse:sensitive-data
	// +crd-enricher:deckhouse:documentation:examples=<password>
	Password string `json:"password,omitempty"`
}
```

→ each field gets `x-kubernetes-sensitive-data: true` plus its `x-doc-examples`.

### `raw:<key>` — inject an arbitrary standard schema field

Some standard schema fields cannot be produced by controller-gen for certain Go
types. `raw:<key>` sets `<key>` **directly** (not under an `x-doc-*` prefix). A
**dotted** `<key>` walks into nested schema nodes; the intermediate nodes must
already exist (controller-gen must have emitted them), otherwise you get a
warning instead of a silently grown schema.

Set a `pattern` on a `metav1.Duration` (which controller-gen renders as an
opaque string):

```go
// +crd-enricher:raw:pattern=^(\d+h)?(\d+m)?(\d+s)?$
ScanInterval *metav1.Duration `json:"scanInterval,omitempty"`
```

→ `pattern: ^(\d+h)?(\d+m)?(\d+s)?$`

Override the descriptions controller-gen pulls from shared meta types (the value
is YAML, so a quoted string with `\n` escapes works). Applied as a type-level
marker on the root struct:

```go
// +crd-enricher:raw:properties.apiVersion.description="APIVersion defines the versioned schema of this representation of an object.\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources)"
// +crd-enricher:raw:properties.kind.description="Kind is a string value representing the REST resource this object represents.\nMore info [in the Kubernetes documentation](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds)"
type ApplicationPackage struct { ... }
```

This also works to override a shared item description on a slice field, e.g.
`raw:items.description` or `raw:items.properties.reason.description` on a
`[]metav1.Condition` field.

### `crd:<key>` — CRD-level settings (type-level only)

Placed on the **root type**, these configure the CRD document itself — things
controller-gen cannot express or that the hand-curated Deckhouse CRDs normalize.
Each setting is its own `crd:<key>=<value>` marker. A value-less marker is
treated as `true`.

```go
// +kubebuilder:object:root=true
// +crd-enricher:crd:preserveUnknownFields=false
// +crd-enricher:crd:minimal=true
// +crd-enricher:crd:stripFormat=true
type ModuleConfig struct { ... }
```

| Setting | Effect |
| --- | --- |
| `preserveUnknownFields=<bool>` | Sets `spec.preserveUnknownFields` on the CRD. |
| `minimal=true` | Switches to the curated "minimal" style: drops `listKind`, the implicit `apiVersion`/`kind`/`metadata` root properties, and the leading `---` document separator. |
| `stripFormat=true` | Removes **every** schema-level `format` key (controller-gen infers `int32`, `date-time`, etc.; the curated CRDs drop them). |
| `stripFormat=[int32]` | Removes only the listed format values, keeping the rest (e.g. keep `date-time`, drop `int32`). |
| `exampleScope=tree` | Attaches a generated composite example to **every** object node, not just the CRD root (see below). |

Setting any `crd:*` marker also strips the `controller-gen.kubebuilder.io/version`
annotation and switches the file to the curated style (no leading `---`).

> **Note:** CRD labels and annotations are **not** set here — they are emitted
> natively by controller-gen from `+kubebuilder:metadata:labels` and
> `+kubebuilder:metadata:annotations`.

## Automatic example generation

Beyond explicit `examples` markers, the enricher synthesizes `x-doc-examples`
from the bottom up so a CRD carries a complete, ready-to-copy usage example
without anyone hand-writing it:

- Every **scalar leaf** yields one representative value. Precedence:
  1. its first explicit `examples` marker, else
  2. the schema `default` (`kubebuilder:default`), else
  3. the documented `x-doc-default`, else
  4. the first `enum` value, else
  5. a type-based placeholder (`"string"`, `0`, `false`, or `2024-01-01T00:00:00Z`
     for `format: date-time`).
- **Composite nodes** (objects, arrays, maps) aggregate their children into a
  structured example. A map (`additionalProperties`) uses `key` as the sample key.

The **CRD root** always receives a synthesized example carrying `apiVersion`,
`kind` and `metadata: {name: example}` together with the aggregated `spec`; the
`status` subtree is omitted (examples document the desired state a user submits).

By default only the root is annotated. `crd:exampleScope=tree` makes every object
node carry its own aggregated example as well.

**Explicit examples always win** — a node that already has an `examples` marker is
never overwritten by generation.

## CLI reference

```
crd-enricher paths=<go-packages> crds=<crd-dir> [dir=<workdir>]
```

| Argument | Meaning |
| --- | --- |
| `paths=` | Comma-separated Go package patterns holding the API structs (the source of markers). Repeatable / comma-joined. |
| `crds=` | Directory of CRD YAML files produced by controller-gen, enriched in place. |
| `output:crd:artifacts:config=` | Alias for `crds=`, so the same controller-gen-style argument can be reused. |
| `dir=` | Optional working directory used to resolve the package patterns. Defaults to the current directory. |
| `-h`, `--help`, `help` | Print usage. |

Example:

```bash
crd-enricher \
  paths="./deckhouse-controller/pkg/apis/deckhouse.io/..." \
  crds=bin/crd/bases \
  dir=$(pwd)
```

On success it prints one `enriched <file>` line per modified file, or
`no CRDs required enrichment` when nothing changed. It exits non-zero on error
(e.g. unloadable packages or an unknown argument).

### Using it as a library

```go
import crdenricher "github.com/deckhouse/deckhouse/pkg/crd-enricher"

changed, err := crdenricher.Run(crdenricher.Options{
	Paths:  []string{"./pkg/apis/..."},
	CRDDir: "bin/crd/bases",
	Dir:    ".", // optional
})
```

`Run` returns the list of modified files. Non-fatal problems (markers pointing at
schema nodes that don't exist, unresolvable `raw:` paths, sensitive-data on the
root) are collected as warnings — construct an `Enricher` directly if you want to
inspect `Enricher.Warnings()`.

## How it works

1. **Load** the Go packages named by `paths=` with `golang.org/x/tools/go/packages`
   (full type info), and collect every marker attached to each type and field.
   Types carrying `+kubebuilder:object:root=true` are recorded as CRD roots, keyed
   by API version (derived from the package name, e.g. `v1alpha1`) and kind.
2. **Parse** each CRD YAML file with `sigs.k8s.io/yaml` — the same library
   controller-gen uses — so files without markers round-trip byte-for-byte.
3. **Match** each CRD (by `spec.names.kind` + version) to its Go root type, then
   walk the Go struct and the `openAPIV3Schema` in lockstep: pointers are
   dereferenced, slices descend into `items`, maps into `additionalProperties`,
   embedded `,inline` structs merge into the current node, and JSON tags map Go
   fields to schema properties.
4. **Apply** the markers to the matching schema nodes, apply CRD-level settings
   once from the root type, then **generate** examples bottom-up.
5. **Write** the result back only if it changed, preserving the leading `---`
   separator unless the CRD opted into the curated (`minimal`) style.

## Warnings and gotchas

- **The prefix is mandatory.** `+x-doc-default=3m` is *not* recognized as an
  enricher marker (it's treated as an unknown/legacy marker and ignored). Always
  write `+crd-enricher:deckhouse:documentation:default=3m`.
- **Markers must match a real schema node.** If a field has a marker but
  controller-gen emitted no property for it (wrong JSON name, field pruned), you
  get a warning: `Type.Field: marker present but schema has no property "x"`.
- **Dotted `raw:` paths must already exist.** `raw:items.description` only works
  if controller-gen already produced `items`; otherwise it warns rather than
  creating the node.
- **`sensitive-data` on the root type is dropped** with a warning — put it on
  `spec` or a specific field.
- **Values are YAML, not strings.** `examples=1` yields the integer `1`;
  `examples="1"` yields the string `"1"`; `stripFormat=[int32]` yields a list.
  Quote when you need a string.
- **Run order matters.** Always run `crd-enricher` *after* controller-gen against
  the *same* directory and package paths. It edits in place and is idempotent.
</content>
</invoke>
