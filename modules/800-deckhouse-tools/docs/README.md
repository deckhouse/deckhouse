---
title: "The deckhouse-tools module"
webIfaces:
- name: deckhouse-tools
description: "The deckhouse-tools Deckhouse module provides a web interface in the cluster for downloading Deckhouse utilities (Deckhouse CLI)"
---

The module creates a web UI with links to download Deckhouse tools (currently [Deckhouse CLI](../../deckhouse-cli/) for various operating systems).

The web interface address is composed based on the [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) template of the Deckhouse global configuration parameter (the `%s` key is replaced with `tools`).

For example, if `publicDomainTemplate` is set to `%s-kube.company.my`, the tools web interface will be exposed at `tools-kube.company.my`.
