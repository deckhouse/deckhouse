---
title: "The node-local-dns module: configuration"
---

{% include module-bundle.liquid %}

The module does not require any configuration (it works right out-of-the-box).

**Pay attention to the following:**
- The module supports the iptables `kube-proxy` mode only (the ipvs mode is not supported and not tested).
- By default, the module **does not** serve `hostNetwork` requests (they are forwarded to `kube-dns`). In this case, you can specify the  `169.254.20.10`address in the Pod configuration yourself. However, if a `node-local-dns` will crash, you will not be able to get back to `kube-dns`.
