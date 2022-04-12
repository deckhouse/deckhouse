---
title: "The metallb module: configuration"
---


This module is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  metallbEnabled: "true"
```

## Parameters

The module has the following parameters in the `deckhouse` ConfigMap:

* `speaker` — parameters of the `speaker` component that announces services (using `bgp` or `layer2` (LVS) routing protocol) and routes application traffic to its node.
    * `nodeSelector` — selects nodes where the `speaker` DaemonSet is active.
        * A mandatory parameter.
    * `tolerations` — allow (but do not require) the pods to schedule onto nodes with matching `taints`.
        * An optional parameter.
* `bgpPeers` — a list of external BGP routers to use with the module.
    * Format — a data array similar to that of [MetalLB's](https://metallb.universe.tf/configuration/#bgp-configuration). Main parameters:
        * `peer-address` — the IP address of the external BGP router.
        * `peer-asn` — the AS number on the external BGP router.
        * `my-asn` — the AS number in the cluster.
        * `source-address` - source IP address for outbound connections.
        * `hold-time` — the timeout after which the neighboring BGP peer is considered lost. This value is divided by three to get the keep-alive interval.
            * The recommended value is `3s` (i.e., keep-alive packets are sent once per second). Note that the BGP protocol does not support lower values.
            * By default, the parameter is set to `90s` (i.e., keep-alive packets are sent every 30 seconds).
        * `node-selector` — the additional pseudo-selector implemented by the speaker application. It selects nodes allowed to connect to external BGP routers. Do not confuse it with `speaker.nodeSelector` and  `nodeSelector`.
            * An optional parameter.
            * The format is [`matchLabels` or `matchExpressions`](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#resources-that-support-set-based-requirements).
    * The parameter is optional if only the `layer2` mode is used to announce services.
* `addressPools` — a list of IP ranges to assign to services.
    * Format — a data array similar to that of [MetalLB's](https://metallb.universe.tf/configuration/#advanced-address-pool-configuration). Main parameters:
        * `name` — the name of the pool (you can specify it using the service annotation as follows: `metallb.universe.tf/address-pool: <name>`).
        * `protocol` —  the protocol used by the speaker to announce services. Possible values:
            * `bgp`.
            * `layer2` — use the L2 LVS mode.
        * `addresses` — a list of ranges, where each range can look like a subnet/mask or a numeric address range (with a "-" delimiter).
* `nodeSelector` — a selector for the main controller. It is the same as the pods' `spec.nodeSelector` parameter in Kubernetes.
    * If the parameter is omitted or `false`, it will be determined [automatically](../../#advanced-scheduling).
* `tolerations` — tolerations for the main controller. They are the same as the pods' `spec.tolerations` parameter in Kubernetes.
    * If the parameter is omitted or `false`, it will be determined [automatically](../../#advanced-scheduling).
