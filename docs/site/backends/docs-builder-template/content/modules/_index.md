---
title: Modules overview
url: modules/readme.html
layout: modules
---

Deckhouse Platform has a modular structure. Modules can be embedded in Deckhouse or connected using the `ModuleSource` resource.

The main difference of the _embedded_ Deckhouse module is that the embedded module is delivered as part of the Deckhouse platform and shares a common release cycle with Deckhouse. Documentation for Deckhouse embedded modules can be found in the [Deckhouse documentation](/documentation/v1/) section.

Deckhouse modules (connected using the `ModuleSource` resource) have a release cycle independent of Deckhouse, i.e. they can be updated independently of Deckhouse versions. Deckhouse modules can be developed by a development team separate from the Deckhouse development team. The operation of a particular module may impact Deckhouse stability, although we strive to ensure that this impact does not have serious consequences for the entire platform.

This section provides information on Deckhouse modules that have passed preliminary compatibility testing and are approved for use in conjunction with Deckhouse.
