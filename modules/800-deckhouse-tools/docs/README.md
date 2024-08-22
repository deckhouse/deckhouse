---
title: "The deckhouse-tools module"
webIfaces:
- name: deckhouse-tools
description: "The deckhouse-tools Deckhouse module provides a web interface in the cluster for downloading Deckhouse utilities (Deckhouse CLI)"
---

The module creates a web UI with links to download Deckhouse tools (currently [Deckhouse CLI](../../deckhouse-cli/) for different OS).

The tools web UI address is formed as follows: the key `%s` of the [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) global Deckhouse configuration parameter is replaced by `tools`.

For example, if `publicDomainTemplate` is set as `%s-kube.company.my`, then the tools web interface will be available at `tools-kube.company.my`.
