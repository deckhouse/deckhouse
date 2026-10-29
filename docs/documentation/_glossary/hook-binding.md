---
title: Hook binding
description: An event on which a hook runs, specified in the hook configuration. A hook can run on Kubernetes events, on schedule, at startup, before or after the Helm templates are applied.
lang: en
---

Bindings of a [module hook](glossary.html#module-hook) are `onStartup`, `beforeHelm`, `afterHelm`, `kubernetes` and `schedule`. Global hooks also have `beforeAll` and `afterAll`. The hooks of a [Marketplace package](glossary.html#marketplace-package) use the same kinds of bindings, with the `Queue` field that sets a separate queue for the `Schedule` and `Kubernetes` bindings. Hooks with the same binding run in the order of their `Order` value.

For details, see [Hooks](architecture/marketplace/hooks.html).
