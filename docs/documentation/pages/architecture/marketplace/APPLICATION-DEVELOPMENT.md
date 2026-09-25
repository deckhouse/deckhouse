---
title: Application development
permalink: en/architecture/marketplace/application-development.html
description: "Create an Application package for Deckhouse Platform Marketplace: bootstrap, project structure, package.yaml, requirements, local rendering, verification, build, CI/CD, and OCI artifact layout."
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

To create a `<APPLICATION_NAME>/` directory in the current working directory with the package skeleton and initialize a Git repository with the first commit, run the command `d8 package bootstrap app <APPLICATION_NAME>` (`application` is an alias of `app`).

Example:

```bash
d8 package bootstrap app myapp --hooks
cd myapp
git remote add origin <GITLAB_REPO_URL>
git push --set-upstream origin main
```

Available options:

| Option | Description |
|---|---|
| `--hooks` | Generate Go hooks: a hook example and a settings validation hook |
| `--werf` | Describe the image build in a `werf.inc.yaml` file instead of a `Dockerfile` |
| `--extended` | Add the `oss.yaml` file with the list of open source components used by the application |
| `-o, --output <OUTPUT_PATH>` | Path where the package will be created (default: `<CURRENT_WORKING_DIRECTORY>/<APPLICATION_NAME>`) |

## Project structure

In the generated project, the package manifest, schemas, templates, hooks, images, documentation, and continuous integration and continuous delivery (CI/CD) configuration are stored in separate directories and files.

```text
myapp/
├── .gitignore
├── .gitlab-ci.yml             # CI/CD pipeline.
├── .pkglint.yaml              # Settings of the d8 package verify command.
├── changelog.yaml             # Changes in the package version.
├── docs/
│   ├── README.md              # Application documentation.
│   ├── README_RU.md
│   ├── CONFIGURATION.md
│   ├── CONFIGURATION_RU.md
│   └── icon.svg               # Application icon.
├── hooks/                     # Go hooks (--hooks).
│   ├── hooks.yaml             # Build instructions for the hooks binary.
│   └── batch/
│       ├── go.mod
│       ├── go.sum
│       ├── main.go
│       ├── settings/
│       │   └── check.go       # Settings validation hook.
│       └── triggers/
│           └── hook.go        # Hook example.
├── images/                    # Container images of the application.
│   └── echo/
│       └── Dockerfile         # werf.inc.yaml with --werf.
├── openapi/
│   ├── settings.yaml          # OpenAPI schema for Application.spec.settings.
│   ├── doc-ru-settings.yaml   # Russian descriptions of the settings.
│   └── values.yaml            # OpenAPI schema for Helm values.
├── oss.yaml                   # Open source components (--extended).
├── package.yaml               # Package manifest.
└── templates/                 # Helm templates.
    ├── _helpers/              # Template helpers.
    ├── deployment.yaml
    ├── pdb.yaml
    ├── registry-secret.yaml
    ├── service.yaml
    └── vpa.yaml
```

Don't add `Chart.yaml` or `values.yaml` to the package root: `d8 package build` doesn't include these files in the package bundle. DP renders the templates as a chart without metadata, and default values are described in the [OpenAPI schemas](settings.html).

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
| `requirements.modules` | No | Module dependencies (SemVer constraints). See [Requirements](#requirements) |
| `disable` | No | Confirmation before the application is deleted. See [Deletion confirmation](#deletion-confirmation) |

When DP scans a repository, it publishes the description, stage, requirements, deletion confirmation, and the contents of `changelog.yaml` in the `status.packageMetadata` field of the [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) resource.

### Requirements

The `requirements` section defines the conditions under which the application can be installed and run:

| Field | Description |
|---|---|
| `requirements.deckhouse.constraint` | Constraint on the DP version |
| `requirements.kubernetes.constraint` | Constraint on the Kubernetes version |
| `requirements.modules.mandatory` | Modules that must be enabled. `constraint` is optional |
| `requirements.modules.conditional` | Modules that are not required but, if enabled, must match `constraint`. `constraint` is required |
| `requirements.modules.anyOf` | Groups of alternative modules: at least one module of each group must be enabled and match its `constraint`, if it is specified |
| `requirements.modules.noneOf` | Groups of incompatible modules: none of the modules of a group may be enabled. `constraint` narrows the incompatible versions; without it, all versions of the module are incompatible |

Each `anyOf` and `noneOf` group has a unique `name`, which is used in error messages, an optional `description`, and a non-empty `modules` list. A module can be listed in only one of the `mandatory`, `conditional`, `anyOf`, and `noneOf` sections. Within `anyOf` or `noneOf`, the same module can be listed in several groups.

Example:

```yaml
requirements:
  deckhouse:
    constraint: ">= 1.76"
  kubernetes:
    constraint: ">= 1.31"
  modules:
    mandatory:
      - name: cert-manager
    conditional:
      - name: prometheus
        constraint: ">= 1.60"
    anyOf:
      - name: storage
        description: "A storage for application data"
        modules:
          - name: sds-local-volume
          - name: csi-ceph
    noneOf:
      - name: local-path-storage
        description: "Local path volumes are not supported"
        modules:
          - name: local-path-provisioner
```

DP checks the requirements:

- When an Application is created or changed. If the requirements are not met, the request is rejected, and the message names the unmet requirement.
- Before the installation. DP installs the application only after the modules from `requirements.modules.mandatory` are enabled.
- While the application is installed. If the requirements stop being met, for example, a mandatory module is disabled, DP uninstalls the application (see [Lifecycle and debugging](lifecycle.html#suspension)).

Module versions are compared with constraints without the pre-release and build metadata parts.

### Deletion confirmation

The `disable` section defines a confirmation the user must give before the application is deleted in the web interface:

```yaml
disable:
  confirmation: true
  messages:
    en: "Deleting the application deletes all the data stored in its volumes."
    ru: "<RU_MESSAGE>"
```

DP publishes the section in the `status.packageMetadata.disableOptions` field of the ApplicationPackageVersion, and the web interface shows the message in the language of the user. DP doesn't check the confirmation when an Application is deleted through the Kubernetes API.

### changelog.yaml

The `changelog.yaml` file describes the changes in the package version:

```yaml
features:
  - "Added support for Redis 7.4."
fixes:
  - "Fixed the readiness probe of the replica."
```

DP publishes the contents of the file in the `status.packageMetadata.changelog` field of the ApplicationPackageVersion.

## OpenAPI schemas

The `openapi/` directory defines two schemas:

- `settings.yaml` — the schema for `Application.spec.settings` (user-facing configuration). The legacy name `config-values.yaml` is also supported.
- `values.yaml` — the schema for the full set of Helm values.

The schemas, validation rules, and the extensions that control defaulting, immutability, and the settings form in the web interface are described in [Application settings](settings.html).

## Templates and hooks

- The values available to templates and the rules for templates are described in [Templates](templates.html).
- Go hooks and the settings validation hook are described in [Hooks](hooks.html).
- How DP installs, updates, and deletes an application and how to debug it is described in [Lifecycle and debugging](lifecycle.html).

## Local rendering

To render the templates of the package and print the resulting manifests, run the following command in the package directory:

```bash
d8 package render
```

Each object in the output is preceded by a comment with the name of its template file.

Available options:

| Option | Description |
|---|---|
| `--file <FILE_NAME>` | Print only the objects rendered from the template with the specified file name |
| `--render-file <PATH>` | Write the manifests to a file, without the template file name comments |
| `-r, --remote <REPOSITORY>/<PACKAGE_NAME>:<PACKAGE_VERSION>` | Render a published package bundle instead of the local directory. A tag or a digest is required |
| `--remote-user`, `--remote-password` | Credentials for the registry. They can also be set in the `PACKAGE_REMOTE_USER` and `PACKAGE_REMOTE_PASSWORD` environment variables |

The command uses stub values instead of the values from a cluster: the `test` instance in the `default` namespace, the `dev` package version, and settings generated from `openapi/settings.yaml` (from `x-example`, `x-examples`, `enum`, or `default`). Image references are stubs keyed by the names of the image directories. To render the templates of an installed application with its actual values, use `deckhouse-controller packages render` (see [Lifecycle and debugging](lifecycle.html#state-in-dp)).

## Linting

To check the structure, manifests, and schemas of the package, run the following command in the package directory:

```bash
d8 package verify
```

The command reports errors and warnings of the built-in rules and exits with an error if at least one error is found. To get the full list of rules with descriptions, run `d8 package doc`.

The rules are grouped by linters:

| Linter | What it checks |
|---|---|
| `package` | Required files (`changelog.yaml`, `docs/`), absence of build artifacts (`werf.yaml`, `.werf/`, `.helmignore`), and correctness of `requirements` in `package.yaml` |
| `openapi` | Types of `x-deckhouse-*` extension values, `x-deckhouse-ui-advanced` only on top-level settings, `enum` values in CamelCase, and a `doc-ru-*` file for each schema except `values.yaml` |
| `templates` | Object names (the `d8a-<INSTANCE_NAME>-` prefix, the suffix length of Job and CronJob names), absence of `metadata.namespace`, PodDisruptionBudget and VerticalPodAutoscaler objects for workloads, and named `targetPort` in Services. The templates are rendered with the `test` instance name in the `default` namespace |
| `docs` | Non-empty `docs/README.md`, a Russian version of every document, and no Cyrillic in English documents |
| `images` | Image directory names without `_` and the format of patch files |
| `icon` | Application icon `docs/icon.{png,webp,jpg,jpeg,svg}`: format, size up to 150 KB, and dimensions up to 300×300 pixels |
| `oss` | Format of `oss.yaml`, if the file exists |

Available options: `--hide-warnings` (don't show warnings), `--show-ignored` (show the findings of ignored rules), `--lint-config <PATH>` (path to the settings file).

To check a published package, run `d8 package verify remote <REPOSITORY> <PACKAGE_NAME>`. By default, the command checks the bundle of the latest version. Use `--version <PACKAGE_VERSION>` to choose the version and `--release` to also check the version metadata image. The command uses the registry credentials saved with `d8 dk cr login`.

### .pkglint.yaml

The `.pkglint.yaml` file lowers the severity of the rules. The command looks for it in the package directory and its parent directories (for `verify remote`, in the current directory).

Example:

```yaml
version: "1"
static:            # d8 package verify.
  linters:
    templates:
      rules:
        vpa:
          impact: ignored
        service-port:
          impact: warn
remote:            # d8 package verify remote.
  bundle:
    linters:
      docs:
        impact: warn
```

The `impact` field takes the `error`, `warn`, and `ignored` values. The impact of a rule can't be higher than the impact of its linter. The rules of the `package` and `openapi` linters can't be configured, and the `instance-prefix`, `instance-namespace`, and `job-name` rules follow the impact of the `templates` linter.

## Local build

To build and publish the package to an OCI registry, run:

```bash
d8 package build -v v0.0.1 -r dev-registry.deckhouse.io/deckhouse/packages
```

For local development, use the [`payload-registry`](/modules/payload-registry/) module as your own container image registry.

Specifics of the command:

- Specify the root path of the packages in `-r`, the same as the `spec.registry.repo` field of the PackageRepository resource. The command appends the package name to the path itself.
- Specify the version in the `vMAJOR.MINOR.PATCH` format. DP finds only the versions in this format when it scans the repository.
- The images are built with werf (`d8 delivery-kit`) for the `linux/amd64` platform, so the package directory must be a Git repository. Uncommitted changes are also included in the build.
- The `images/` directory must contain at least one image.
- If the version already exists in the registry, the command exits without building. To rebuild the version, use `-f`.

Available options:

| Option | Environment variable | Description |
|---|---|---|
| `-v, --version` | — | Package version (required) |
| `-r, --repo` | `PACKAGE_BUILD_REPOSITORY` | Root path of the packages in the registry. Without it, the package is only built locally |
| `-u, --user`, `-t, --token` | `PACKAGE_BUILD_REPOSITORY_USER`, `PACKAGE_BUILD_REPOSITORY_TOKEN` | Registry credentials |
| `--final-repo`, `--final-user`, `--final-token` | `PACKAGE_BUILD_FINAL_REPOSITORY`, `PACKAGE_BUILD_FINAL_REPOSITORY_USER`, `PACKAGE_BUILD_FINAL_REPOSITORY_TOKEN` | Registry path and credentials to publish the package to, if they differ from the build registry |
| `-f, --force` | — | Rebuild and publish a version that already exists in the registry |
| `--insecure` | `PACKAGE_BUILD_INSECURE` | Allow HTTP and skip the verification of the TLS certificates of the registries |
| `--sign`, `--sign-cert`, `--sign-key` | `PACKAGE_BUILD_SIGN_CERT`, `PACKAGE_BUILD_SIGN_KEY` | Sign the images with the specified certificate and key |

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

The package and related data are published to an OCI-compatible registry. The package bundle, container images, and version metadata are stored at separate paths.

| Path | Description |
|---|---|
| `<REPOSITORY>:<PACKAGE_NAME>` | Package name tag — used to list packages |
| `<REPOSITORY>/<PACKAGE_NAME>:<PACKAGE_VERSION>` | Bundle — contains templates, `openapi/`, `hooks/` |
| `<REPOSITORY>/<PACKAGE_NAME>@<DIGEST>` | Container images of the application. Templates reference them by digest (see [Templates](templates.html#container-images)) |
| `<REPOSITORY>/<PACKAGE_NAME>/version:<PACKAGE_VERSION>` | Version metadata |
| `<REPOSITORY>/<PACKAGE_NAME>/release-channel:<RELEASE_CHANNEL>` | Version recommended for a release channel |

Here `<REPOSITORY>` is the root path of the packages, for example, `registry.deckhouse.io/deckhouse/<EDITION>/packages`.

### Bundle contents

The main bundle image (`<PACKAGE_NAME>:<PACKAGE_VERSION>`) contains:

```text
├── package.yaml         # Package manifest with the version.
├── images_digests.json  # Digests of the container images.
├── openapi/             # Settings and values schemas.
├── templates/           # Helm templates.
├── charts/              # Helm subcharts, if any.
├── hooks/               # Hooks binary.
├── docs/                # Documentation and icon.
├── changelog.yaml       # Release notes.
└── oss.yaml             # Open source components, if any.
```

### Version metadata image contents

The metadata image (`<PACKAGE_NAME>/version:<PACKAGE_VERSION>`) contains:

```text
├── package.yaml       # Package manifest.
├── version.json       # SemVer version.
├── changelog.yaml     # Release notes.
├── openapi/           # Settings and values schemas.
└── docs/              # Documentation and icon.
```

DP reads the version metadata when it scans the repository and creates ApplicationPackageVersion objects from it.

### Release channels

A release channel is a tag of the `<PACKAGE_NAME>/release-channel` path that points to a copy of the version metadata image of the recommended version. DP recognizes the `alpha`, `beta`, `early-access`, `stable`, `rock-solid`, and `lts` channels. `d8 package build` doesn't create release channel tags; they are published by the CI/CD pipeline.

When DP scans a repository, it records the versions the channels point to in the `status.releaseChannels` field of the [ApplicationPackage](../../reference/api/cr.html#applicationpackage) resource:

```yaml
status:
  releaseChannels:
    my-registry:        # PackageRepository name.
      alpha: v0.2.0
      stable: v0.1.21
```

The channels are informational: the version of an application is set in `spec.packageVersion` and changes only when the user changes it.
