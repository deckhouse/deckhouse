---
title: DKP component registry
permalink: en/admin/configuration/registry/
description: "DKP component registry: configuring interaction and usage."
search: container registry, registry configuration, edition management, registry management, container images
---

This section describes the settings for interacting with the DKP component registry.

This section covers configuring DKP interaction with the registry in a running cluster. If you need information about working with the registry during cluster installation, go to the [Installing the platform](../../../installing/) section.

The capabilities and processes for configuring the DKP component registry depend on how the cluster is managed. In clusters fully managed by DKP, configuration management is handled by the [`registry`](/modules/registry/) module (for more details, see the section ["Configurations in a DKP-Managed cluster"](managing-interaction.html)). In Managed Kubernetes clusters, the `helper change-registry` is used; the `registry` module is not used (for more details, see the section ["Configurations in a Managed Kubernetes cluster"](third-party.html)).
