---
title: Container registry structure
permalink: en/reference/container-registry-structure.html
toc: false
search: container registry, image registry
---

## Commercial editions container registry

The registry for commercial DKP editions is available at `registry.deckhouse.io/deckhouse/ee`, `registry.deckhouse.io/deckhouse/se`, and `registry.deckhouse.io/deckhouse/ce`. It contains modules and components available in the EE, SE, and CE editions.

Key features:

* implements functionality according to the edition;
* for modules analogous to those in the CSE edition, security testing is performed as part of the secure software development lifecycle;
* delivered electronically.

Images are provided with [technical support](https://deckhouse.io/tech-support/), including operational consultation.

Image build and update specifics:

* built from open-source software [included in DKP](./oss_info.html):
  * from upstream (as-is);
  * with modifications for use in DKP CSE;
  * with deep modernization (fork).
* includes proprietary company developments;
* vulnerability remediation and package updates are performed only when upstream open-source packages are updated;
* updates are provided:
  * with new functionality;
  * with bug and vulnerability fixes;
  * with security incident resolution.
