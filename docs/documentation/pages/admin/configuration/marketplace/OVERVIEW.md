---
title: Marketplace
permalink: en/admin/configuration/marketplace/
description: "Configure and manage Marketplace in Deckhouse Platform. Connect package repositories, monitor scanning operations, and make application packages available for users."
relatedLinks:
  - title: "Using Marketplace"
    url: ../../../user/marketplace/
---

Marketplace is a system for managing Deckhouse Platform (DP) delivery units (Packages). It lets administrators connect container registries with packages, discover available packages, and make them available to project users for installation.

{% alert level="info" %}
Marketplace is available starting from DP version 1.76.
{% endalert %}

## Administrator tasks

A cluster administrator:

1. Connects a container registry with packages by creating a [PackageRepository](package-repository.html) resource.
2. Monitors the scanning operations that discover packages in the registry.
3. Grants users the permissions to view package versions and to create Application objects in their namespaces. `ApplicationPackage` and `ApplicationPackageVersion` are cluster-wide resources.

Users interact with packages through the Application object (read more in the [Using → Marketplace](../../../user/marketplace/) section).

## Key resources

| Resource | Short name | Scope | Description |
|---|---|---|---|
| [`PackageRepository`](../../../reference/api/cr.html#packagerepository) | — | Cluster | Container registry with packages and its scan settings |
| [`PackageRepositoryOperation`](../../../reference/api/cr.html#packagerepositoryoperation) | `pro` | Cluster | Scanning operation on a repository |
| [`ApplicationPackageVersion`](../../../reference/api/cr.html#applicationpackageversion) | `apv` | Cluster | Discovered version of a package |
| [`ApplicationPackage`](../../../reference/api/cr.html#applicationpackage) | `ap` | Cluster | Package summary: the repositories that provide the package, the versions the release channels point to, and the applications that use it |
| [`Application`](../../../reference/api/cr.html#application) | — | Namespace | Installed application instance (managed by users) |

The [Package repositories](package-repository.html) section describes how to connect a container registry and check the repository status.

The [Scanning](scanning.html) section describes how to monitor scan operations and trigger manual scans.
