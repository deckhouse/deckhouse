---
title: Private environment
description: An environment without direct access to the public container registry of Deckhouse Platform and to external package repositories. Access goes through a proxy server or through a container registry deployed inside the perimeter.
lang: en
---

Proxy server parameters set in the cluster configuration during installation are propagated to the environment variables of cluster nodes and Deckhouse Platform components. User applications do not receive them automatically. Cluster nodes are usually reached through a [bastion host](glossary.html#bastion-host), and the private registry runs on a separate machine in the internal network. When access to external resources is disabled completely, no proxy is used, the required artefacts are brought into the perimeter by other means, and the cluster is an [air-gapped cluster](glossary.html#air-gapped-cluster).

For details, see [Installing DP in a private environment](guides/private-environment.html).
