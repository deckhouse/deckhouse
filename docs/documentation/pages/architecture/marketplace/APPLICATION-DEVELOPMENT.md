---
title: Application development
permalink: en/architecture/marketplace/application-development.html
description: "Create an Application package for Deckhouse Platform Marketplace: bootstrap, project structure, package.yaml, CI/CD setup, local build, and OCI artifact layout."
---

## Prerequisites

Install `deckhouse-cli` (`d8`):

```bash
sh -c "$(curl -fsSL https://raw.githubusercontent.com/deckhouse/deckhouse-cli/main/tools/install.sh)"
```

Log in to the OCI registry with your [license token](https://license.deckhouse.io/):

```bash
d8 dk cr login -u license-token dev-registry.deckhouse.io --password <LICENSE_TOKEN>
```

## Bootstrapping an Application package

To create a `<APPLICATION_NAME>/` directory in the current working directory with the package skeleton and initialize a Git repository with the first commit, run the command `d8 package bootstrap application <APPLICATION_NAME>`.

Example:

```bash
d8 package bootstrap application myapp --hooks
cd myapp
git remote add origin <GITLAB_REPO_URL>
git push --set-upstream origin main
```

Available options:

| Option | Description |
|---|---|
| `--hooks` | Generate a Go hooks skeleton |
| `--werf` | Use werf for image builds |
| `--extended` | Add an extended set of files |
| `-o, --output <OUTPUT_PATH>` | Path where the package will be created (default: `<CURRENT_WORKING_DIRECTORY>/<APPLICATION_NAME>`) |

## Project structure

In the generated project, the package manifest, schemas, templates, hooks, images, documentation, and continuous integration and continuous delivery (CI/CD) configuration are stored in separate directories and files.

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

The `package.yaml` file is the main manifest for an Application package and defines metadata, type, requirements, and compatibility.

Example `package.yaml` file:

```yaml
apiVersion: v1
type: "Application"
name: redis
descriptions:
  ru: "<RU_DESCRIPTION>"
  en: "Redis — in-memory database"
# Injected automatically at build time.
version: "v1.0.1"
stage: "Preview"
category: "Databases"
# Environment requirements.
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
| `descriptions` | Yes | Localized description for the catalog and user interface (UI) (`ru`, `en`) |
| `version` | Yes | Version in Semantic Versioning (SemVer) format; added automatically at build time |
| `type` | Yes | `Application` or `Module` |
| `stage` | Yes | Maturity stage (`Preview`, `General Availability`, etc.) |
| `category` | Yes | Category for catalog classification |
| `requirements.deckhouse` | No | Minimum Deckhouse Platform (DP) version constraint |
| `requirements.kubernetes` | No | Minimum Kubernetes version constraint |
| `requirements.modules` | No | Module dependencies (SemVer constraints) |

## OpenAPI schemas

The `openapi/` directory defines two schemas:

- `config-values.yaml` (or `settings.yaml`) — the schema for `Application.spec.settings` (user-facing configuration).
- `values.yaml` — the schema for the full set of Helm values.

### Defaulting a grantable cluster-wide resource value (x-deckhouse-grantable-resource)

A `settings` field of type `string` can be bound to a grantable cluster-wide resource managed by the
[`multitenancy-manager`](/modules/multitenancy-manager/) (for example, a StorageClass).

When the field is bound and the user leaves it empty, the resource name configured as the project default is injected into `values`. When the user provides a value, it is checked against the resources available to the project. A value that is not in this list is rejected.

To bind the field to a cluster-wide resource, add the `x-deckhouse-grantable-resource` extension and specify the name of the resource available to the project through a grant (`AvailableClusterResource` or `GrantableClusterResourceDefinition`), for example, `storageclasses`.

{% alert level="info" %}
The resource group, version, and kind (GVK) are defined by the grant. You do not need to specify them in `openapi/settings.yaml`.
{% endalert %}

Example `openapi/settings.yaml` with `x-deckhouse-grantable-resource`:

```yaml
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

- The default is resolved per project from the AvailableClusterResource in the Application's namespace, so different projects can receive different defaults.
- An explicitly specified user value has higher priority than the project default.
- If the custom resource definition (CRD) is absent, no catalog exists for the project, or the catalog has no default value, the field remains unchanged. No value is injected or validated.

### Immutable fields (x-deckhouse-immutable)

Some settings should not be changed after the application configuration is applied. For example, changing a `storageClass` after volumes have been created either has no effect or can cause application failures. To prevent changes to the value of such a field after the application configuration has been successfully applied, add the `x-deckhouse-immutable: true` extension to it.

Example `openapi/settings.yaml` with `x-deckhouse-immutable`:

```yaml
type: object
properties:
  storageClass:
    type: string
    default: default
    x-deckhouse-immutable: true
  postgres:
    type: object
    properties:
      storageClass:
        type: string
        x-deckhouse-immutable: true
      volumeSize:
        type: string
```

Behavior:

- The extension is effective only when set to `true`. Any other value is ignored.
- When `x-deckhouse-immutable` is added to an object, the entire object becomes immutable. After the application configuration has been successfully applied for the first time, changing any nested field is rejected, even if `x-deckhouse-immutable` is not set on that field. Set the extension on an object only when the entire object must be immutable. To make only one field immutable, add `x-deckhouse-immutable` directly to it, as with `postgres.storageClass` in the example above. In this case, the restriction does not apply to `postgres.volumeSize`, and its value can be changed.
- If an update changes an immutable value, the validating webhook rejects it and reports the field name.
- In the web interface, the value of a field with the extension can be set when installing the application. In the edit form of an installed application, the field is read-only.
- Comparison uses the configuration that was actually applied, after schema defaults are applied. Therefore, a field with the extension can be omitted from the manifest only if its `default` matches the value that has already been applied. If there is no default value or it differs from the applied value, the change is rejected.

{% alert level="info" %}
If an entire object is removed from the manifest, default values are not applied to its nested fields. Therefore, removing an object that contains fields using `x-deckhouse-immutable` can cause frozen values to be lost and is rejected.
{% endalert %}

- The mark is not inherited into array elements or map entries that the update adds — a new element has no previous value to be frozen against.

## Local build

To build and publish the package to an OCI registry, run:

```bash
d8 package build -v v0.0.1 -r dev-registry.deckhouse.io/deckhouse/packages
```

For local development, use the [`payload-registry`](/modules/payload-registry/) module as your own container image registry.

## Linting

Validate the package structure and configuration:

```bash
d8 package verify
```

The command reports errors and warnings based on `.pkglint.yaml` and built-in rules.

## CI/CD setup

The CI/CD pipeline publishes package releases to the OCI registry. To publish a release, configure the OCI registry credentials, then create a Git tag in SemVer format and push it to the repository.

### Environment variables

The pipeline uses the following variables to authenticate to the OCI registry:

| Variable | Description |
|---|---|
| `PACKAGES_REGISTRY_LOGIN` | OCI registry username for publishing |
| `PACKAGES_REGISTRY_PASSWORD` | OCI registry password or token |

### Triggering a release

The pipeline is triggered by a Git tag in SemVer format:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The pipeline builds the package and pushes it to the OCI registry. Once the pipeline completes, the package version is available for scanning via PackageRepository.

## OCI artifact layout in the registry

The package and related data are published to an OCI-compatible registry. The package bundle, additional images, and version metadata are stored at separate paths.

| Path | Description |
|---|---|
| `registry.deckhouse.io/deckhouse/<EDITION>/packages:<PACKAGE_NAME>` | Package name tag — used to list packages |
| `registry.deckhouse.io/deckhouse/<EDITION>/packages/<PACKAGE_NAME>:<PACKAGE_VERSION>` | Bundle — contains templates, `openapi/`, `hooks/` |
| `registry.deckhouse.io/deckhouse/<EDITION>/packages/<PACKAGE_NAME>/extra/<IMAGE_NAME>:<PACKAGE_VERSION>` | Additional images (application containers) |
| `registry.deckhouse.io/deckhouse/<EDITION>/packages/<PACKAGE_NAME>/version:<PACKAGE_VERSION>` | Version metadata — contains `package.yaml`, `version.json`, `changelog.yaml` |
| `registry.deckhouse.io/deckhouse/<EDITION>/packages/<PACKAGE_NAME>/version:<RELEASE_CHANNEL>` | Recommended version for a release channel |

### Bundle contents

The main bundle image (`<PACKAGE_NAME>:<PACKAGE_VERSION>`) contains:

```text
├── package.yaml       # Package manifest
├── openapi/           # Settings and values schemas
├── templates/         # Helm templates
└── hooks/             # Lifecycle hooks
```

### Version metadata image contents

The metadata image (`<PACKAGE_NAME>/version:<PACKAGE_VERSION>`) contains:

```text
├── package.yaml       # Package manifest
├── version.json       # SemVer version
└── changelog.yaml     # Release notes
```
