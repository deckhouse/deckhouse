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

Deckhouse Kubernetes Platform (DKP) Marketplace uses five custom resources:

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
| [`Application`](../../reference/api/cr.html#application) | `app` | Namespace | Installed instance; drives Nelm deployment |

### ApplicationPackageVersion content

Each [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) object carries:

- `status.packageMetadata.description` — localized (`en`/`ru`) package description
- `status.packageMetadata.category` — catalog category
- `status.packageMetadata.stage` — maturity stage (`Preview`, `General Availability`, etc.)
- `status.packageMetadata.requirements` — DKP and Kubernetes version constraints; module dependencies (`mandatory`, `conditional`, `anyOf`, `noneOf`)
- `status.packageMetadata.versionCompatibilityRules` — upgrade and downgrade rules
- `status.packageSchemas.settingsSchema` — OpenAPI v3 schema used to validate `Application.spec.settings`
- `status.packageSchemas.valuesSchema` — OpenAPI v3 schema for effective values passed to hooks and templates

## Scan-to-deploy lifecycle

1. Administrator creates [PackageRepository](../../reference/api/cr.html#packagerepository).
2. DKP creates a [PackageRepositoryOperation](../../reference/api/cr.html#packagerepositoryoperation) automatically (first scan on creation, then every `scanInterval`).
3. The operation scans the registry and creates [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) objects for each discovered version.
4. User creates an [Application](../../reference/api/cr.html#application) in their namespace referencing `packageName`, `packageVersion`, and optionally `packageRepositoryName`.
5. DKP validates `spec.settings` against the `settingsSchema` from the corresponding ApplicationPackageVersion.
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

Kubernetes limits Pod names to 63 characters. An Application pod name is composed of:

- instance name — ≤24 chars
- resource name — ≤24 chars
- deployment suffix — 15 chars

Therefore:

- **Application instance name** (`metadata.name`): at most **24 characters**
- **Resource name inside the Application** (e.g., Deployment name suffix): at most **24 characters**

Example: instance `redis-cache` (11 chars) + resource `master-deployment` (17 chars) + suffix (15 chars) = 43 chars total — fits within the 63-character limit.
