---
title: Modules overview
url: modules/readme.html
layout: modules
---

Deckhouse Platform has a modular structure. Modules can be embedded in Deckhouse or connected using the [module Source](#) resource.

The main difference between _embedded_ Deckhouse modules and _plug-in_ modules is that embedded modules are delivered as part of the Deckhouse platform, undergo deeper testing, and share a common release cycle with Deckhouse. Documentation for Deckhouse embedded modules can be found in the [Deckhouse documentation](../../) section.

Deckhouse modules have a release cycle independent of Deckhouse, i.e. they can be updated independently of Deckhouse versions. Deckhouse modules can be developed by a development team separate from the Deckhouse development team. The operation of a particular plugin **may** impact Deckhouse stability, although we strive to ensure that this impact does not have serious consequences for the entire platform.

This section provides information on Deckhouse modules that have passed preliminary compatibility testing and are approved for use in conjunction with Deckhouse. For each module, information about the module's authors, how to get technical support, and the terms of use of the module is also available.

