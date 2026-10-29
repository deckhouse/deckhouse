---
title: Installation configuration file
description: A set of YAML documents with Deckhouse Platform settings and manifests of the cluster objects to be created after installation. It is passed to the CLI installer with the --config parameter.
lang: en
---

The file contains [InitConfiguration](reference/api/cr.html#initconfiguration) (required), [ClusterConfiguration](reference/api/cr.html#clusterconfiguration) (required unless Deckhouse Platform is installed into an existing cluster), [StaticClusterConfiguration](reference/api/cr.html#staticclusterconfiguration) or the provider cluster configuration, and optionally [ModuleConfig](reference/api/cr.html#moduleconfig), [NodeGroup](/modules/node-manager/cr.html#nodegroup) and other objects. The [CLI installer](glossary.html#installer) reads it during [preflight checks](glossary.html#preflight-checks) and the cluster bootstrap.

For details, see [Installation](installing/).
