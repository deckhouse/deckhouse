---
title: Marketplace
permalink: en/user/marketplace/
description: "Using Marketplace in Deckhouse Kubernetes Platform. Browse available application packages, install them into your namespace, and manage their lifecycle."
---

This section describes the ways to use Marketplace in the Deckhouse Kubernetes Platform (DKP).

Marketplace lets you install ready-made applications into your namespace from registries connected by the cluster administrator. Each application is installed as an [Application](../../reference/api/cr.html#application) resource and can exist in multiple independent instances — for example, separate Redis instances for caching and sessions in the same namespace.

{% alert level="info" %}
Marketplace are available starting from DKP version 1.76.
{% endalert %}

## Prerequisites

Before you can install an application, the cluster administrator must connect at least one package registry. Ask your administrator to check that [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) objects are available:

```bash
d8 k get apv
```

If the output is empty, the registry has not been scanned yet — contact your administrator.

The [Installing and managing applications](applications.html) section describes how to browse available versions, install, update, and delete applications.

The [Troubleshooting](troubleshooting.html) section covers how to diagnose and resolve problems with installed applications.
