---
title: Conditional module dependency
description: A dependency on another module that applies only if that module is enabled. The module works on its own, but if the other module is present its version must match the constraint.
lang: en
---

A conditional dependency is a kind of [module dependency](glossary.html#module-dependencies) and is available in Deckhouse Platform starting with version 1.73. It may affect the ability to enable, disable and update both modules: the dependent one and the one it depends on. A [Marketplace package](glossary.html#marketplace-package) lists such modules in `requirements.modules.conditional`.

For details, see [Dependencies of the Deckhouse Platform modules](architecture/module-development/dependencies/).
