---
title: "The documentation module"
description: "A web interface with documentation for the Deckhouse Kubernetes Platform."
webIfaces:
- name: documentation
---

The `documentation` module creates a documentation web UI for the Deckhouse version currently used in a cluster.

This can be useful when Deckhouse works in a network with limited Internet access.

To get the web interface address, in the [publicDomainTemplate](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) template of the Deckhouse global configuration parameter, replace the `%s` key with `documentation`.

For example, if `publicDomainTemplate` is set as `%s-kube.company.my`, then the documentation web interface will be available at `documentation-kube.company.my`.
