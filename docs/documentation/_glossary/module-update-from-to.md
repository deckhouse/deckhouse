---
title: Skipping intermediate module releases
description: The from-to mechanism that lets Deckhouse Platform skip step-by-step module updates. If the installed module version falls into a from-to range declared in module.yaml, Deckhouse Platform installs the latest version within to.
lang: en
---

Transition rules are listed in the `update.versions` section of `module.yaml`. A release that contains them is called a constrained release. The rule is read from the release to move to, and its `to` must match the version of that release. If the installed version is lower than `from`, the update proceeds sequentially. When several rules match, the one with the largest `to` is chosen.

For details, see [Module development and debugging](architecture/module-development/development/).
