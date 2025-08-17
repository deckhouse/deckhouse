---
title: "The node-local-dns module: configuration"
---

{% alert level="warning" %}
- The module works with all `CNIs`, but in order to work correctly with `cni-cilium`, a number of [conditions](../cni-cilium/#limitations) must be met.
- By default, when used together with the `cni-simple-bridge` or `cni-flannel` modules, the `node-local-dns` module **does not work** for requests from `hostNetwork`. In this case, all requests go to the `kube-dns` module. You can specify the address `169.254.20.10` in the pod configuration, but if `node-local-dns` module crashes, there will be no *fallback* to the `kube-dns` module.
{% endalert %}

<!-- SCHEMA -->
