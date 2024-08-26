---
title: "The node-local-dns module: configuration"
---

{% include module-bundle.liquid %}

The module does not require any configuration (it works right out-of-the-box).

**Pay attention to the following:**
- The module works with all `CNIs`, but in order to work correctly with `cni-cilium`, a number of [conditions](../../../../../modules/021-cni-cilium/#limitations) must be met.
- By default for `cni-simple-bridge` and `cni-flannel`, the module **does not** serve `hostNetwork` requests (they are forwarded to `kube-dns`). In this case, you can specify the  `169.254.20.10` address in the Pod configuration yourself. However, if a `node-local-dns` will crash, you will not be able to get back to `kube-dns`.
