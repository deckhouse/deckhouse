---
title: Application development
permalink: en/architecture/marketplace/application-development.html
description: "Create an Application package for Deckhouse Kubernetes Platform Marketplace: bootstrap, project structure, package.yaml, CI/CD setup, local build, and OCI artifact layout."
---

## Prerequisites

Install `deckhouse-cli` (`d8`):

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/deckhouse/deckhouse-cli/main/tools/install.sh)"
```

Log in to the package registry with your [license token](https://license.deckhouse.io/):

```bash
d8 dk cr login -u license-token dev-registry.deckhouse.io --password <YOUR_TOKEN>
```

## Bootstrapping a new Application

`d8 package bootstrap application <name>` creates a `<name>/` directory in the current working directory with the package skeleton and initializes a git repository with the first commit.

```bash
d8 package bootstrap application myapp --hooks
cd myapp
git remote add origin <gitlab-repo.git>
git push --set-upstream origin main
```

**Flags:**

| Flag | Description |
|---|---|
| `--hooks` | Generate a Go hooks skeleton |
| `--werf` | Use werf for image builds |
| `--extended` | Add an extended set of files |
| `-o, --output <path>` | Custom output path (default: `<cwd>/<name>`) |

## Project structure

```text
myapp/
├── .gitignore
├── .gitlab-ci.yml          # CI/CD pipeline
├── changelog.yaml
├── docs/
│   └── README.md           # Application documentation
├── hooks/                  # Go hooks
│   ├── hooks.yaml
│   └── batch/
│       ├── go.mod
│       ├── go.sum
│       ├── main.go
│       └── triggers/
│           └── hook.go
├── images/                 # Image sources or pull instructions
│   └── myapp/
│       └── werf.inc.yaml
├── openapi/
│   ├── config-values.yaml  # OpenAPI schema for Application.spec.settings
│   └── values.yaml         # OpenAPI schema for Helm values
├── oss.yaml
├── package.yaml            # Package manifest
└── templates/              # Helm templates
    ├── deployment.yaml
    ├── registry-secret.yaml
    └── service.yaml
```

## package.yaml

The central manifest for an Application package. Defines metadata, type, requirements, and compatibility.

```yaml
apiVersion: v1
type: "Application"
name: redis
descriptions:
  ru: "Redis - in-memory база данных"
  en: "Redis - in-memory database"
# Injected automatically at build time.
version: "v1.0.1"
stage: "Preview"
category: "Databases"
# Environmental Requirements.
requirements:
  deckhouse: 
    constraint: ">= 1.70"
  kubernetes: 
    constraint: ">= 1.31"
  modules:
    mandatory:
      - name: cert-manager 
        constraint: ">= 1.0.0"
```

**Field reference:**

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Unique package name |
| `descriptions` | Yes | Localized description for catalog and UI (`ru`, `en`) |
| `version` | Yes | Semver version; injected at build time |
| `type` | Yes | `Application` or `Module` |
| `stage` | Yes | Maturity stage (`Preview`, `General Availability`, etc.) |
| `category` | Yes | Category for catalog classification |
| `requirements.deckhouse` | No | Minimum DKP version constraint |
| `requirements.kubernetes` | No | Minimum Kubernetes version constraint |
| `requirements.modules` | No | Module dependencies (semver constraints) |

## OpenAPI schemas

The `openapi/` directory defines two schemas:

- `config-values.yaml` (or `settings.yaml`) — the schema for `Application.spec.settings` (user-facing configuration).
- `values.yaml` — the schema for the full set of Helm values.

### Defaulting from cluster resource grants (`x-deckhouse-grantable-resource`)

A `settings` field of `type: string` can be bound to a grantable cluster resource managed by the
[multitenancy-manager](/modules/multitenancy-manager/) (for example a `StorageClass`). When the field is bound:

- if the user leaves it empty, the project's **default** granted name is injected into the values;
- if the user provides a value, it is checked against the names **available** to the project, and rejected otherwise.

Add the `x-deckhouse-grantable-resource` extension to the field and reference the grantable resource by name (the
`AvailableClusterResource` / `GrantableClusterResourceDefinition` name, e.g. `storageclasses`). The
underlying resource's GVK is owned by the grant definition and must **not** be specified here.

```yaml
# openapi/settings.yaml
type: object
properties:
  storageClass:
    type: string
    x-deckhouse-grantable-resource: storageclasses
  postgres:
    type: object
    properties:
      storageClass:
        type: string
        x-deckhouse-grantable-resource: postgresclasses
```

Behavior:

- The default is resolved per project from the `AvailableClusterResource` in the Application's namespace, so different projects can receive different defaults.
- An explicit user value always wins over the injected default.
- If the multitenancy feature is inactive for the resource (the CRD is absent, no catalog exists for the project, or the catalog has no default), the field is left untouched — no defaulting and no validation.

## Local build

Build and push the package to a registry:

```bash
d8 package build -v v0.0.1 -r dev-registry.deckhouse.io/deckhouse/packages
```

For local development, use the [payload-registry](https://deckhouse.ru/modules/payload-registry/) module as a personal registry.

## Linting

Validate the package structure and configuration:

```bash
d8 package verify
```

Reports errors and warnings based on `.pkglint.yaml` and built-in rules.

One of the built-in rules is mandatory object naming: every object a template renders must be named `d8a-{{ .Application.Instance.Name }}-<suffix>`. The `d8a-` prefix is reserved for application objects and protects them from being modified by cluster users. Objects that hooks create get the same prefix from the runtime, see [Hooks](hooks.html).

## CI/CD setup

### Environment variables

| Variable | Description |
|---|---|
| `PACKAGES_REGISTRY_LOGIN` | Registry login for publishing |
| `PACKAGES_REGISTRY_PASSWORD` | Registry password or token |

### Triggering a release

The pipeline is triggered by a semver git tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The pipeline builds the package and pushes it to the registry. Once the pipeline completes, the package version is available for scanning via PackageRepository.

## OCI artifact layout in the registry

```text
registry.deckhouse.io/deckhouse/<edition>/packages:<name>
    Package name tag — for listing support

registry.deckhouse.io/deckhouse/<edition>/packages/<name>:<version>
    Bundle — contains templates, openapi/, hooks/

registry.deckhouse.io/deckhouse/<edition>/packages/<name>/extra/<image>:<version>
    Additional images (application containers)

registry.deckhouse.io/deckhouse/<edition>/packages/<name>/version:<version>
    Version metadata — contains package.yaml, version.json, changelog.yaml

registry.deckhouse.io/deckhouse/<edition>/packages/<name>/version:<release-channel>
    Recommended version for a release channel
```

### Bundle contents

The main bundle image (`<name>:<version>`) contains:

```text
├── package.yaml       # Package manifest
├── openapi/           # Settings and values schemas
├── templates/         # Helm templates
└── hooks/             # Lifecycle hooks
```

### Version metadata image contents

The metadata image (`<name>/version:<version>`) contains:

```text
├── package.yaml       # Package manifest
├── version.json       # Semver version
└── changelog.yaml     # Release notes
```
