---
title: "The deckhouse-tools module"
webIfaces:
- name: deckhouse-tools
description: "A web interface for downloading Deckhouse CLI (d8) for various operating systems."
---

The module creates a web UI with links to download [Deckhouse CLI]({% if site.mode != 'module' %}{{ site.canonical_url_prefix_documentation }}{% endif %}/cli/d8/) tool for various operating systems.

The web interface address is composed based on the [publicDomainTemplate](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) template of the Deckhouse global configuration parameter (the `%s` key is replaced with `tools`).

For example, if `publicDomainTemplate` is set to `%s-kube.company.my`, the tools web interface will be exposed at `tools-kube.company.my`.
