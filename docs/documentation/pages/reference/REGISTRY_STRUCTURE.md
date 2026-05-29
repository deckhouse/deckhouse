---
title: Container registry structure
permalink: en/reference/container-registry-structure.html
toc: false
search: container registry, image registry
---

This page provides a general overview of the container image registries used to install the Deckhouse Kubernetes Platform (DKP).

## Commercial editions container registry

The registry contains modules and components available in the commercial DKP editions:

- EE (address `registry.deckhouse.io/deckhouse/ee`).
- SE (address `registry.deckhouse.io/deckhouse/se`).
- SE+ (address `registry.deckhouse.io/deckhouse/se-plus`).
- BE (address `registry.deckhouse.io/deckhouse/be`).

Key features:

* implements functionality according to the edition;
* security testing is performed as part of the secure software development lifecycle;
* delivered electronically.

Images are provided with [technical support](https://deckhouse.io/tech-support/), including operational consultation.

Image build and update specifics:

* built from open-source software [included in DKP](./oss_info.html):
  * from upstream (as-is);
  * with deep modernization (fork).
* includes proprietary company developments;
* vulnerability remediation and package updates are performed only when upstream open-source packages are updated;
* updates are provided:
  * with new functionality;
  * with bug and vulnerability fixes;
  * with security incident resolution.
