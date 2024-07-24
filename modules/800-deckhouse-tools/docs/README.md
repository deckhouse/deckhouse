---
title: "The documentation module"
webIfaces:
- name: documentation
---

The module creates a web UI with links to download Deckhouse tools (currently d8-cli for all systems).

This can be useful, for example, when Deckhouse works in a network with limited Internet access.

The tools web UI address is formed as follows: the key `%s` of the [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) global Deckhouse configuration parameter is replaced by `tools`.

For example, if `publicDomainTemplate` is set as `%s-kube.company.my`, then the tools web interface will be available at `tools-kube.company.my`.
