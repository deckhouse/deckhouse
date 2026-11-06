---
title: Module dependencies
description: A set of conditions (requirements) that must be met for Deckhouse Platform to run a module. They are declared in the requirements section of module.yaml.
lang: en
---

A module can depend on the Deckhouse Platform version, on the Kubernetes version and on the versions of other [modules](glossary.html#module). Dependencies on other modules affect the module startup order together with the [module weight](glossary.html#module-weight), and a dependency may be [conditional](glossary.html#module-conditional-dependency). A [Marketplace package](glossary.html#marketplace-package) declares the same kind of requirements in `package.yaml`, including groups of alternative and incompatible modules.

For details, see [Dependencies of the Deckhouse Platform modules](architecture/module-development/dependencies/).
