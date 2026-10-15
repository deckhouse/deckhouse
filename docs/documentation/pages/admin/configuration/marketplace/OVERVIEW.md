---
title: Marketplace
permalink: en/admin/configuration/marketplace/
description: "Configure and manage Marketplace in Deckhouse Platform. Connect package repositories, monitor scanning operations, and make application packages available for users."
relatedLinks:
  - title: "Using Marketplace"
    url: ../../../user/marketplace/
---

Marketplace is a package management system in Deckhouse Platform (DP). It lets administrators connect package repositories, discover available packages, and make them available to project users to install applications.

{% alert level="info" %}
Marketplace is available in DP starting from version 1.76.
{% endalert %}

For the Marketplace architecture, including core concepts, application development and configuration, templates, Nelm annotations, hooks, and the application lifecycle, refer to ["Architecture"](../../../architecture/marketplace/).

To learn about installing and removing applications from Marketplace, as well as troubleshooting, refer to ["Usage"](../../../user/marketplace/).

## Administrator tasks

A cluster administrator:

1. Connects a package repository by creating a [PackageRepository](../../../reference/api/cr.html#packagerepository) resource.
2. Monitors the repository scanning operations by Marketplace, discovering available packages.
3. Grants users the permissions to view package versions and to create Application objects in their namespaces.

Users interact with packages through the [Application object](../../../reference/api/cr.html#application).

## Marketplace resources

The following resources are used for working with the Marketplace:

| Resource | Short name | Scope | Description |
|---|---|---|---|
| [PackageRepository](../../../reference/api/cr.html#packagerepository) | — | Cluster | Package repository and its scanning settings |
| [PackageRepositoryOperation](../../../reference/api/cr.html#packagerepositoryoperation) | `pro` | Cluster | A repository scanning operation |
| [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) | `apv` | Cluster | Discovered version of a package |
| [ApplicationPackage](../../../reference/api/cr.html#applicationpackage) | `ap` | Cluster | Package summary: the repositories it's available in, the versions the release channels point to, and the applications that use it |
| [Application](../../../reference/api/cr.html#application) | — | Namespace | Installed application instance managed by a user |

The ["Package repositories"](package-repository.html) section describes how to connect a repository and check its status.

The ["Scanning"](scanning.html) section describes how to monitor scanning operations and trigger a scanning manually.
