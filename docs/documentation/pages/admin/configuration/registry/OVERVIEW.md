---
title: DP components registry
permalink: en/admin/configuration/registry/
description: "DP component registry: configuring interaction and usage."
search: container registry, registry configuration, edition management, registry management, container images
---

This section describes the settings for interacting with the DP component registry.

This section covers configuring DP interaction with the registry in a running cluster. If you need information about working with the registry during cluster installation, go to the ["Platform installation"](../../../installing/) section.

The capabilities and processes for configuring the DP component registry depend on how the cluster is managed. In clusters fully managed by DP, configuration management is handled by the [`registry`](/modules/registry/) module (for more details, see the section ["Managing the registry in DP-managed clusters"](managing-interaction.html)). In Managed Kubernetes clusters, the `helper change-registry` is used; the `registry` module is not used (for more details, see the section ["Switching a Managed Kubernetes cluster to use third-party registry"](third-party.html)).
