---
title: CSI driver
description: A plugin that manages persistent storage volumes in Deckhouse Platform. It is part of the cloud-provider-* and csi-* modules, and all implementations share one reference architecture.
lang: en
---

Container Storage Interface (CSI) is a standard interface that unifies access to storage systems. Every [cloud provider](glossary.html#cloud-provider) and [external storage](glossary.html#external-storage) system has its own implementation of the specification. The implementations differ in the supported capabilities and components, but not in architecture.

For details, see [CSI driver](architecture/storage/csi-drivers/csi-driver.html).
