---
title: "Connecting additional networks to pods"
permalink: en/user/network/sdn/pod-connecting-dedicated-networks.html
---

To connect additional networks to the pod, use the pod annotation, specifying the parameters of the additional networks to be connected:

```yaml
network.deckhouse.io/networks-spec: |
  [
    {
      "type": "Network", # Connecting the my-network project network.
      "name": "my-network",
      "ifName": "veth_mynet",    # TAP interface name inside the pod (optional).
      "mac": "aa:bb:cc:dd:ee:ff" # MAC address to assign to the TAP interface (optional).
    },
    {
      "type": "ClusterNetwork", # Connecting to the public network my-cluster-network.
      "name": "my-cluster-network",
      "ifName": "veth_public",
    }
  ]
```
