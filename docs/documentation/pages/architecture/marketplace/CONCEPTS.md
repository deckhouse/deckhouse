---
title: Concepts
permalink: en/architecture/marketplace/concepts.html
description: "Core Marketplace concepts: Package types, CRD model, scan-to-deploy lifecycle, Application constraints, and naming limits."
---

## Package types

A **Package** is an abstract unit that encompasses either an **Application** or a **Module**. The distinction is based on scope and purpose:

| Characteristic | Module | Application |
|---|---|---|
| **Purpose** | Infrastructure extension for the cluster | User workload |
| **Scope** | Cluster-wide (one per cluster) | Namespaced (unlimited instances) |
| **Multiple instances** | No (1:1 with the cluster) | Yes (N instances in different namespaces) |
| **Enabled by default** | Can be enabled via bundle | Only by explicit user action |
| **CRD creation** | Allowed | Forbidden |
| **Cluster-wide objects** | Allowed | Forbidden |

## Resource model

Deckhouse Platform (DP) Marketplace uses five custom resources:

<script src="/assets/js/mermaid.min.js"></script>
<script>mermaid.initialize({ startOnLoad: true });</script>

<pre class="mermaid">
flowchart TD
    PR[PackageRepository] -->|triggers| PRO[PackageRepositoryOperation]
    PR -->|populates| APV[ApplicationPackageVersion]
    APV -->|aggregated by| AP[ApplicationPackage]
    APV -->|referenced by| APP[Application\nnamespace-scoped]
</pre>

| Resource | Short | Scope | Role |
|---|---|---|---|
| [`PackageRepository`](../../reference/api/cr.html#packagerepository) | — | Cluster | Registry connection and scan schedule |
| [`PackageRepositoryOperation`](../../reference/api/cr.html#packagerepositoryoperation) | `pro` | Cluster | Scan job that discovers versions |
| [`ApplicationPackageVersion`](../../reference/api/cr.html#applicationpackageversion) | `apv` | Cluster | One per discovered package version; carries metadata, OpenAPI schemas, and requirements |
| [`ApplicationPackage`](../../reference/api/cr.html#applicationpackage) | — | Cluster | Informational aggregate: which repos have the package, how many instances use it |
| [`Application`](../../reference/api/cr.html#application) | — | Namespace | Installed instance; drives Nelm deployment |

### ApplicationPackageVersion content

Each [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) object carries:

- `status.packageMetadata.description` — localized (`en`/`ru`) package description
- `status.packageMetadata.stage` — maturity stage (`Preview`, `General Availability`, etc.)
- `status.packageMetadata.requirements` — DP and Kubernetes version constraints; module dependencies (`mandatory`, `conditional`, `anyOf`, `noneOf`)
- `status.packageMetadata.disableOptions` — confirmation before the application is deleted (see [Deletion confirmation](application-development.html#deletion-confirmation))
- `status.packageMetadata.changelog` — changes in the version from `changelog.yaml`
- `status.packageSchemas.settingsSchema` — OpenAPI v3 schema used to validate `Application.spec.settings`
- `status.packageSchemas.valuesSchema` — OpenAPI v3 schema for effective values passed to hooks and templates

## Scan-to-deploy lifecycle

1. Administrator creates [PackageRepository](../../reference/api/cr.html#packagerepository).
2. DP creates a [PackageRepositoryOperation](../../reference/api/cr.html#packagerepositoryoperation) automatically (first scan on creation, then every `scanInterval`).
3. The operation scans the registry and creates [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) objects for each discovered version.
4. User creates an [Application](../../reference/api/cr.html#application) in their namespace referencing `packageName`, `packageVersion`, and optionally `packageRepositoryName`.
5. DP validates `spec.settings` against the `settingsSchema` from the corresponding ApplicationPackageVersion.
6. Nelm deploys the Helm templates from the package bundle.
7. Conditions on the Application reflect deployment progress: `Installed` → `ConfigurationApplied` → `Scaled` → `Ready`.

## Application constraints

All constraints exist to enforce namespace isolation and prevent Applications from interfering with cluster-level resources.

### Functional constraints

1. **No CRD creation** — Application templates must not include `CustomResourceDefinition` objects.
2. **No cluster-wide objects** — all resources created by an Application must be namespaced.
3. **No cross-Application dependencies** — an Application can declare dependencies only on Modules (via `requirements.modules` in `package.yaml`), not on other Applications.
4. **Hooks are namespace-scoped** — hooks must not read or write resources outside their own namespace.
5. **Manual install only** — Applications are never activated by default; installation requires explicit user action.

### Naming constraints

The objects of an Application are named `d8a-<INSTANCE_NAME>-<SUFFIX>` (see [Templates](templates.html#object-names)), so the instance name is a part of every object name. To keep the object names within the Kubernetes limits:

- **Application instance name** (`metadata.name`): at most **24 characters**. The validating webhook rejects an Application with a longer name.
- **Resource name suffix inside the Application**: at most **23 characters** for StatefulSets, Jobs, and CronJobs, whose names must fit 52 characters (4 + 24 + 1 + 23 = 52), and at most **34 characters** for other objects, whose names must fit 63 characters.

Example: the `master` StatefulSet of the `redis-cache` instance (11 characters) is named `d8a-redis-cache-master` (22 characters).
