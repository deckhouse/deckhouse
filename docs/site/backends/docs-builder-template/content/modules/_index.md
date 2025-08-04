---
title: Modules overview
url: modules/
layout: modules
---

Deckhouse Kubernetes Platform has a modular structure. A module can either be built into Deckhouse (embedded) or be plug-in (using the [ModuleSource](/products/kubernetes-platform/documentation/v1/cr.html#modulesource) resource).

The distinguishing feature between an embedded Deckhouse module and a plug-in one is that the embedded module is delivered as part of the Deckhouse Kubernetes Platform and shares a common Deckhouse release cycle. For more information on embedded Deckhouse modules, read the [Deckhouse documentation](/products/kubernetes-platform/documentation/v1/).

Deckhouse modules that are plugged in using the [ModuleSource](/products/kubernetes-platform/documentation/v1/cr.html#modulesource) resource have a release cycle independent of Deckhouse and can be updated separately from Deckhouse versions. Deckhouse plug-in modules may be developed by a team of developers not associated with the Deckhouse development team.

To determine whether a module is built-in or plug-in, you can check the value of the `SOURCE` field in the output of the `kubectl get modules` command. For built-in modules, this field will show `Embedded`, while for plug-in modules, it will display the name of the [ModuleSource](/products/kubernetes-platform/documentation/v1/cr.html#modulesource) (the source of the modules from which the module is installed).

Example of the output:

```console
$ kubectl get modules
NAME                STAGE    SOURCE      PHASE        ENABLED   READY
cni-cilium                   Embedded    Ready        True      True
commander                    deckhouse   Available    False     False
```

This section provides information on Deckhouse modules that can be plugged in from the module source. The modules have undergone preliminary compatibility testing and are approved for use in conjunction with Deckhouse.
