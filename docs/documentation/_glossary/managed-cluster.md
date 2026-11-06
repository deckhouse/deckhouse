---
title: Managed cluster
description: A Deckhouse Platform cluster with the [`commander-agent`](/modules/commander-agent/) module installed. Its configuration and lifecycle are managed from a managing cluster.
lang: en
---

The [`commander-agent`](/modules/commander-agent/) module connects the cluster to the [`commander`](/modules/commander/) module of the [managing cluster](glossary.html#managing-cluster). Not to be confused with a Managed Kubernetes cluster, in which the control plane is provided by a cloud provider: see [cluster managed by Deckhouse Platform](glossary.html#dp-managed-cluster).

For details, see [Cluster management](architecture/deckhouse/commander.html).
