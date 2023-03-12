---
title: "The deckhouse-web module"
webIfaces:
- name: deckhouse
---

The module creates a documentation web UI for the Deckhouse version currently used in a cluster.

This can be useful, for example, when Deckhouse works in a network with limited Internet access.

The documentation web UI address is formed as follows: the key `%s` of the [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) global Deckhouse configuration parameter is replaced by `deckhouse`.

For example, if `publicDomainTemplate` is set as `%s-kube.company.my`, then the documentation web interface will be available at `deckhouse-kube.company.my`.
