# Docs Builder

A Go HTTP service built on [Hugo](https://gohugo.io/) that renders the documentation of Deckhouse Platform modules on demand. It runs next to nginx: nginx serves the rendered pages from a shared cache volume and asks docs-builder for a page it does not have yet.

## How it works

1. A reader opens a page of a module version. nginx does not find it in the cache volume and passes the request to docs-builder.
2. docs-builder answers with a placeholder page, downloads the module image from the registry and renders the version as a Hugo site of its own (EN and RU).
3. The rendered pages land in the cache volume, the placeholder reloads, and nginx serves the pages from then on.

docs-builder also builds:

- the page of the list of modules from the modules it knows;
- the search index, `llms.txt` and the corpus of the AI export from one version of every module: the installed one (or the branch of a ModulePullOverride) of an enabled module of a cluster, the most stable release channel otherwise.

Modules come from one of two places:

- **Cluster** (module `documentation`): every module with a registry source, enabled or not, from the `Module` and `ModuleSource` objects and the package repositories (`ModulePackage`, `PackageRepository`); embedded modules come with the Deckhouse image. A module whose `Module` has the source `Embedded` is embedded even when a source offers it too (a module moving out of Deckhouse): Deckhouse runs the embedded copy, so docs-builder offers no version of it. Once the source changes, the next pass of the pinned versions (every 15 minutes) makes it a module from a source.
- **Public site**: the registries given in `--ondemand-registries`, with a page per release channel.
- **Local modules** (a developer's machine, `make -C docs/site external-module` of the DP repository): the module repositories under `--ondemand-local-modules-dir`, each served as one version (`--ondemand-local-version`) of the public site, without a cluster or a registry. The files are polled every second; a module whose files changed is rendered anew and the pages are replaced at once.

With S3 (`--ondemand-s3-dir`) rendered versions survive restarts and are shared by the replicas. The replica holding the `docs-builder-ondemand-leader` Lease reads the registry; the others follow what it recorded in S3.

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`  | `/{lang}/modules/` | Page of the list of modules (a placeholder until it is built) |
| `GET`  | `/{lang}/modules/{module}/...` | Pages of a module version, rendered on demand |
| `GET`  | `/{lang}/modules/{module}/versions/` | Page with the versions of a module |
| `GET`  | `/api/v2/render/{module}/{version}` | State of the render of a version, polled by the placeholder |
| `GET`  | `/api/v2/modules/{module}/versions` | Versions of a module |
| `GET`  | `/api/v2/modules/{module}/menu` | Menu of a module |
| `GET`  | `/readyz` | 200 once the pod may take traffic (with S3: once it took the versions from there) |
| `GET`  | `/healthz` | 200 while the service runs; 500 once a pass of the pinned versions is 10 minutes past its deadline of an hour |

## Configuration

Flags are listed by `docs-builder -help`. `--ondemand-cache-dir` is required.

| Variable | Description |
|----------|-------------|
| `LOG_LEVEL` | Logging level: debug, info, warn, error, fatal |
| `POD_NAME` | Identity in the leader Lease (the host name when empty) |
| `POD_NAMESPACE` | Namespace of the leader Lease |

## Module archive

A module version is rendered from these files of the module image:

```
├── docs/          → content/modules/{module}/{version}/
├── openapi/       → data/modules/{module}/{version}/
├── crds/          → data/modules/{module}/{version}/
├── module.yaml    → data/modules/{module}/{version}/
└── oss.yaml       → data/modules/{module}/{version}/
```

- Files with the `_RU.md` suffix become `.ru.md`.
- `docs/internal`, `docs/development` and similar directories are skipped.
- A module whose files break the Hugo build is dropped from the site and the error is logged.
