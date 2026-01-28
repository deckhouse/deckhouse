---
title: "Direct Mode Architecture"
permalink: en/architecture/registry-direct-mode.html
---

In Direct mode, registry requests are processed directly, without intermediate caching.

CRI requests to the registry are redirected based on its configuration, which is defined in the `containerd` configuration.

For components such as [operator-trivy](/modules/operator-trivy/), `image-availability-exporter`, `deckhouse-controller`, and others that access the registry directly, requests will go through the in-cluster proxy located on the master nodes.

<!--- Source: mermaid code from docs/internal/DIRECT.md --->
![direct](../images/registry-module/direct-en.png)

For more information about the `Direct` mode, see the section [Using the internal container image registry](../admin/configuration/registry/internal.html).
