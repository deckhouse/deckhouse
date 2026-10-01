---
title: Marketplace
permalink: en/user/marketplace/
description: "Using Marketplace in Deckhouse Platform. Browse available application packages, install them into your namespace, and manage their lifecycle."
---

This section describes how to use Marketplace in Deckhouse Platform (DP).

Marketplace lets you install ready-made applications into your namespace from the container registries connected by the cluster administrator. Each application is installed as an [Application](../../reference/api/cr.html#application) resource and can exist in multiple independent instances, for example, separate Redis instances for caching and for sessions in the same namespace.

{% alert level="info" %}
Marketplace and the Application resource are available starting from DP version 1.76.
{% endalert %}

## Prerequisites

Before you can install an application, the cluster administrator must connect at least one container registry with packages. Check that [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) objects are available:

```bash
d8 k get apv
```

If the output is empty, the registry has not been scanned yet or contains no packages. If the command fails with a `Forbidden` error, you don't have permission to view package versions. In both cases, contact your administrator.

The [Installing and managing applications](applications.html) section describes how to browse available versions, install, update, and delete applications.

The [Troubleshooting](troubleshooting.html) section describes how to diagnose and resolve problems with installed applications.
