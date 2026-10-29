---
title: In-cluster image cache
description: A container image registry deployed on the master nodes of the cluster, from which all nodes pull the images of Deckhouse Platform components.
lang: en
---

The cache is enabled by the `storage.cache` parameter of the [`registry`](/modules/registry/) [ModuleConfig](reference/api/cr.html#moduleconfig) in the `Managed` [registry management mode](glossary.html#registry-mode). While the cache is being filled, the upstream registry stays a fallback path. In an [isolated cluster](glossary.html#air-gapped-cluster), the cache is the only source of images and is filled with the `d8 mirror push` command.
