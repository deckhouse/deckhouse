---
title: bashible-api-server
description: A Kubernetes extension API server on the master nodes that generates bashible scripts from templates stored in custom resources.
lang: en
---

Storing every variant of the scripts for all supported operating systems in etcd is not feasible because of the key size limit and the load, so kube-apiserver forwards requests for resources with [bashible bundles](glossary.html#bashible-bundle) to bashible-api-server and returns the generated result. It serves the second phase of the [bootstrap script](glossary.html#bootstrap-script), the `bashible.sh` script and the rendered bundle of a [node group](glossary.html#node-group).

For details, see [Bashible](architecture/cluster-and-infrastructure/node-management/bashible.html).
