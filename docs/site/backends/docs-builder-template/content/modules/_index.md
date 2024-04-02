---
title: Modules overview
url: modules/readme.html
layout: modules
---

Deckhouse Kubernetes Platform has a modular structure. Modules can be embedded in Deckhouse or connected using the `ModuleSource` resource.

The distinguishing feature of an _embedded_ Deckhouse module is that it is delivered as part of the Deckhouse Kubernetes Platform and shares a common Deckhouse release cycle. For more information on embedded Deckhouse modules, see the [Deckhouse documentation](/documentation/v1/).

Deckhouse modules (connected using the `ModuleSource` resource) have a release cycle that is independent of Deckhouse, i. e. they can be updated independently of Deckhouse versions. Deckhouse modules may be developed by a team that is not part of the Deckhouse development team. The operation of a particular module may affect the stability of Deckhouse, but we strive to ensure that such an impact will not have serious consequences for the platform as a whole.

This section provides information on Deckhouse modules that have passed preliminary compatibility testing and have been approved for use with Deckhouse.
