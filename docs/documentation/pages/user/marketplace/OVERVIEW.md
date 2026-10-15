---
title: Marketplace
permalink: en/user/marketplace/
description: "Using Marketplace in Deckhouse Platform. Browse available application packages, install them into your namespace, and manage their lifecycle."
---

Marketplace lets you install ready-made applications into your namespace from the repositories connected by the cluster administrator. Each application is installed as an [Application](../../reference/api/cr.html#application) resource and can exist in multiple independent instances, for example, separate Redis instances for caching and for sessions in the same namespace.

{% alert level="info" %}
Marketplace and the Application resource are available in Deckhouse Platform starting from version 1.76.
{% endalert %}

To learn about connecting package repositories and managing repository scanning, refer to ["Administration"](../../admin/configuration/marketplace/).

For the Marketplace architecture, including core concepts, application development and configuration, templates, Nelm annotations, hooks, and the application lifecycle, refer to ["Architecture"](../../architecture/marketplace/).

## Prerequisites

Before you can install an application, the cluster administrator must connect at least one package repository. Check that [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) objects are available:

```bash
d8 k get apv
```

If the output is empty, the repository has not been scanned yet or contains no packages. If the command fails with a `Forbidden` error, you don't have permission to view package versions. In both cases, contact your administrator.

For a description of how to browse available versions, install, update, and delete applications, refer to ["Installing and managing applications"](applications.html).

For a description of how to diagnose and resolve problems with installed applications, refer to ["Troubleshooting"](troubleshooting.html).
