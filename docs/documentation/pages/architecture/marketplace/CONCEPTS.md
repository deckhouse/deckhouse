---
title: Concepts
permalink: en/architecture/marketplace/concepts.html
description: "Core Marketplace concepts: Package types, CRD model, scan-to-deploy lifecycle, Application constraints, and naming limits."
---

This section describes package types, resource model, Application constraints, and application installation steps.

## Package types

A package is an abstract unit that encompasses either an **Application** or a **Module**. The distinction is based on scope and purpose:

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
    PR[PackageRepository] -->|Triggers| PRO[PackageRepositoryOperation]
    PR -->|Populates| APV[ApplicationPackageVersion]
    APV -->|Aggregated by| AP[ApplicationPackage]
    APV -->|Referenced by| APP[Application\nnamespace-scoped]
</pre>

| Resource | Short name | Scope | Role |
|---|---|---|---|
| [PackageRepository](../../reference/api/cr.html#packagerepository) | — | Cluster | Connection to a container registry and scan schedule |
| [PackageRepositoryOperation](../../reference/api/cr.html#packagerepositoryoperation) | `pro` | Cluster | Scan job that discovers versions |
| [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) | `apv` | Cluster | One per discovered package version. Carries metadata, OpenAPI schemas, and requirements |
| [ApplicationPackage](../../reference/api/cr.html#applicationpackage) | `ap` | Cluster | Package summary: which repos have the package, which versions the release channels point to, how many instances use it |
| [Application](../../reference/api/cr.html#application) | — | Namespace | Installed instance. Drives Nelm deployment |

### ApplicationPackageVersion content

[ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) contains package version metadata and information required to configure and deploy an application:

| Content | Field |
| --- | --- |
| Localized package description (`en`/`ru`) | `status.packageMetadata.description` |
| Package maturity stage (`Preview`, `General Availability`, and others) | `status.packageMetadata.stage` |
| DP and Kubernetes version requirements and module dependencies (`mandatory`, `conditional`, `anyOf`, `noneOf`) | `status.packageMetadata.requirements` |
| Application removal confirmation settings. For details, refer to ["Deletion confirmation"](application-development.html#deletion-confirmation) | `status.packageMetadata.disableOptions` |
| Package version changes from `changelog.yaml` | `status.packageMetadata.changelog` |
| OpenAPI v3 schema for validating `Application.spec.settings` | `status.packageSchemas.settingsSchema` |
| OpenAPI v3 schema for validating the final set of `values` passed to hooks and templates | `status.packageSchemas.valuesSchema` |

## Application installation steps

Installing an application from a package involves several stages, from connecting and scanning the package repository to creating an Application resource, deploying the application, and checking its status.

1. Administrator creates a [PackageRepository](../../reference/api/cr.html#packagerepository).
2. DP creates a [PackageRepositoryOperation](../../reference/api/cr.html#packagerepositoryoperation) automatically. The initial scanning is performed when the resource is created, and then repeated based on the [`scanInterval`](../../reference/api/cr.html#packagerepository-v1alpha1-spec-scaninterval) value.
3. The operation scans the repository and creates [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) objects for each discovered version.
4. User creates an [Application](../../reference/api/cr.html#application) in their namespace referencing package repository name ([`packageRepositoryName`](../../reference/api/cr.html#application-v1alpha1-spec-packagerepositoryname)), application package name ([`packageName`](../../reference/api/cr.html#application-v1alpha1-spec-packagename)), and version ([`packageVersion`](../../reference/api/cr.html#application-v1alpha1-spec-packageversion)).
5. DP validates `spec.settings` against the `settingsSchema` from the corresponding ApplicationPackageVersion.
6. Nelm deploys the Helm templates from the package bundle.
7. The `Installed` condition on the Application reflects deployment progress and is the only condition reported until the installation completes. Once it becomes `True`, the other conditions (`ConfigurationApplied`, `Scaled`, `Ready`, and so on) appear.

## Application constraints

All constraints exist to enforce namespace isolation and prevent Applications from interfering with cluster-level resources.

### Functional constraints

The following constraints apply:

1. **No CRD creation**: Application templates must not include CustomResourceDefinition objects.
2. **No cluster-wide objects**: All resources created by an Application must be namespaced.
3. **No cross-Application dependencies**: An Application can declare dependencies only on Modules (via `requirements.modules` in `package.yaml`), not on other Applications.
4. **Hooks are namespace-scoped**: Hooks must not read or write resources outside their own namespace.
5. **Manual install only**: Applications are never activated by default; installation requires explicit user action.

### Naming constraints

The objects of an Application are named in the format `d8a-<INSTANCE_NAME>-<SUFFIX>` (for details, refer to ["Templates"](templates.html#object-names)), so the instance name is a part of every object name. To keep the object names within the Kubernetes limits, the following naming constraints apply:

- **Application instance name** (`metadata.name`) can't exceed **24 characters**. The validating webhook rejects an Application with a longer name.
- **Resource name suffix inside the Application** can't exceed **23 characters** for StatefulSets, Jobs, and CronJobs, whose names must fit 52 characters (4 + 24 + 1 + 23 = 52), and at most **34 characters** for other objects, whose names must fit 63 characters.

For example, the `master` StatefulSet of the `redis-cache` instance (11 characters) is named `d8a-redis-cache-master` (22 characters).
