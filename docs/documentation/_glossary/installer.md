---
title: Installer
description: The tool that deploys a Kubernetes cluster and installs Deckhouse Platform in it. It is available as the `dhctl` CLI installer in a container image and as the graphical installer.
lang: en
---

The `dhctl` CLI installer deploys a cluster in a supported cloud, on bare-metal servers, including [hybrid clusters](glossary.html#hybrid-cluster), or installs Deckhouse Platform into an existing Kubernetes cluster. In a [cloud cluster](glossary.html#cloud-cluster) it creates the required infrastructure: virtual machines, networks and other resources. In a running cluster the `dhctl converge` command applies configuration changes of [CloudPermanent nodes](glossary.html#cloud-permanent-node) and changes of [protected parameters](glossary.html#protected-parameters).

The second form is the graphical installer available on the Deckhouse Platform website.

For details, see [Platform installation](installing/).
