---
title: Marketplace
permalink: en/admin/configuration/marketplace/
description: "Configure and manage Marketplace in Deckhouse Kubernetes Platform. Connect package repositories, monitor scanning operations, and make application packages available for users."
relatedLinks:
  - title: "Using Marketplace"
    url: ../../../user/marketplace/
---

Marketplace is a system for managing Deckhouse Kubernetes Platform (DKP) delivery units (Packages). It lets administrators connect package registries, discover available packages, and make them available to project users for installation.

{% alert level="info" %}
Marketplace is available starting from DKP version 1.76.
{% endalert %}

## Administrator tasks

A cluster administrator do the following tasks:

1. Connect a package registry by creating a [PackageRepository](package-repository.html) resource.
2. Monitor scanning operations that discover packages in the registry.
3. Ensure users can see available package versions and install them into their namespaces.

Users interact with packages through the Application object (read more in the [Using → Marketplace](../../../user/marketplace/) section).

## Key resources

| Resource | Short name | Scope | Description |
|---|---|---|---|
| [`PackageRepository`](../../../reference/api/cr.html#packagerepository) | — | Cluster | Source registry for packages |
| [`PackageRepositoryOperation`](../../../reference/api/cr.html#packagerepositoryoperation) | `pro` | Cluster | Scanning operation on a repository |
| [`ApplicationPackageVersion`](../../../reference/api/cr.html#applicationpackageversion) | `apv` | Cluster | Discovered version of a package |
| [`ApplicationPackage`](../../../reference/api/cr.html#applicationpackage) | — | Cluster | Aggregate metadata for a package |
| [`Application`](../../../reference/api/cr.html#application) | `app` | Namespace | Installed application instance (managed by users) |

The [Package repositories](package-repository.html) section describes how to connect a registry and check its status.

The [Scanning](scanning.html) section describes how to monitor scan operations and trigger manual scans.
