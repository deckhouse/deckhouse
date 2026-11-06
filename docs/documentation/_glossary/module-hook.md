---
title: Module hook
description: An executable file of a module or a Marketplace package that Deckhouse Platform runs in response to an event. Hooks prepare values for templates, create and change objects in the cluster and validate settings.
lang: en
---

A hook runs when one of its [bindings](glossary.html#hook-binding) fires: at startup, before or after the Helm templates are applied, on schedule or when watched Kubernetes objects change. The hooks of a module are located in the `/hooks/batch` directory. The hooks of a [Marketplace package](glossary.html#marketplace-package) are written in Go with module-sdk and compiled into a single binary. Module hooks and global hooks exist. The [Deckhouse controller](glossary.html#deckhouse-controller) runs them as tasks of the [addon-operator queues](glossary.html#addon-operator-queue).

For details, see [Module structure](architecture/module-development/structure/).
