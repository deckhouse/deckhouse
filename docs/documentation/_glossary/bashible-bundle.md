---
title: Bashible bundle
description: A collection of bashible scripts. Four bundles exist, bashible installation scripts, bootstrap scripts for the first node, node configuration scripts of a cloud provider and common scripts. An individual script of a bundle is a step.
lang: en
---

[bashible](glossary.html#bashible) scripts are Go templates, so a bundle is rendered for a specific [node group](glossary.html#node-group), and every step must be safe to run again after a failure. Rendered bundles are served by [bashible-api-server](glossary.html#bashible-api-server). The steps cover the main node configuration stages: access for [NodeUser](/modules/node-manager/cr.html#nodeuser), CA certificates, binaries in `/opt/deckhouse/bin`, packages, containerd, [kubernetes-api-proxy](glossary.html#kubernetes-api-proxy), kubelet and the bashible service itself.

For details, see [Bashible](architecture/cluster-and-infrastructure/node-management/bashible.html).
