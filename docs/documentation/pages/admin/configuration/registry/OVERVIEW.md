---
title: Configuring DKP interaction with the container image registry
permalink: en/admin/configuration/registry/
description: "Configuring Deckhouse Kubernetes Platform interaction with the container image registry."
search: container registry, registry configuration, edition management, registry management, container images
---

This section describes the process of configuring Deckhouse Kubernetes Platform interaction with internal and external container image registries.

This section covers configuring DKP interaction with the registry in a running cluster. If you need information about working with the registry during cluster installation, go to the [Installing the platform](../../../installing/) section.

The section [Managing registry interaction in a cluster fully managed by DKP](managing-interaction.html) describes configuring DKP with the registry in a Managed Kubernetes cluster: interaction modes and switching between them (a detailed description of the modes is in the section [Architecture of registry interaction modes](../../../architecture/registry-modes.html)).

The section [Switching a running Managed Kubernetes cluster to a third-party container image registry](third-party.html) covers switching a running Managed Kubernetes cluster (in such a cluster, registry interaction settings cannot be managed via the `registry` module) to use a third-party registry.
