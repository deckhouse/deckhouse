---
title: Managing cluster
description: A Deckhouse Platform cluster with the [`commander`](/modules/commander/) module installed. It creates standardised Deckhouse Platform clusters and manages their configuration and lifecycle.
lang: en
---

The [`commander`](/modules/commander/) module implements a web application for cluster management and needs a PostgreSQL instance, deployed inside or outside the cluster. Clusters under its management run the [`commander-agent`](/modules/commander-agent/) module and are called [managed clusters](glossary.html#managed-cluster).

For details, see [Cluster management](architecture/deckhouse/commander.html).
