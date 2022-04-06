---
title: "The linstor module: configuration examples"
---

<div class="docs__information warning active">
The module is actively developed, and it might significantly change in the future.
</div>

We're working hard to make Deckhouse easy to use and don't want to overwhelm you with too much information. Therefore, we decided to provide a simple and familiar interface for configuring LINSTOR. Namely, just create LVM group or thin pool with certain tag. Such pools will be automatically added to LINSTOR and will be available for use as a StorageClass in Kubernetes.

This simple configuration is currently under active development. In the meantime, you can set up LINSTOR using [advanced configuration](advanced_usage.html).
