---
title: Static cluster
description: |
  A cluster with the [clusterType: Static](reference/api/cr.html#clusterconfiguration-clustertype). Its nodes are physical servers or virtual machines that Deckhouse Platform does not create or manage through a cloud API.
lang: en
---

The administrator prepares the servers, and the [installer](glossary.html#installer) configures them, deploys Kubernetes and installs Deckhouse Platform. The nodes of such a cluster are [static nodes](glossary.html#static-node). A static cluster can be extended with nodes of a cloud provider into a [hybrid cluster](glossary.html#hybrid-cluster).

For details, see [Platform installation](installing/).
