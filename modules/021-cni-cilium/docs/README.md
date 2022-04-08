---
title: "The cni-cilium module"
---

This module is responsible for providing a network between multiple nodes in a cluster using the [cilium](https://cilium.io/) module.

## Limitations

1. This module currently supports only direct-routing mode.
2. Service types `NodePort` and `LoadBalancer` do not work with hostNetwork endpoints in the `DSR` LB mode.
