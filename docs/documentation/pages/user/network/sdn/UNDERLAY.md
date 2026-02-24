---
title: "Connecting physical network interfaces to DPDK application pods"
permalink: en/user/network/sdn/underlay.html
description: |
  Connecting physical network interfaces to pods via DRA for DPDK applications: Shared and Dedicated modes.
search: DPDK applications, UnderlayNetwork, physical interfaces, SR-IOV, VF PF
---

If your namespace (project) hosts high-performance workloads that require direct access to hardware (e.g., DPDK applications), you can use direct connection of physical network interfaces (Physical Functions and Virtual Functions) to pods via Kubernetes Dynamic Resource Allocation (DRA).

Physical network interfaces can be connected to pods in one of two modes:

- `Shared`: Virtual Functions (VF) are created from Physical Functions (PF) using SR-IOV, and multiple pods can share the same hardware.
- `Dedicated`: Each pod gets exclusive access to the entire PF.

For more information about the capabilities and features of working with Underlay networks in DKP, see the section [Configuring and connecting underlay networks for hardware device forwarding](../../../admin/configuration/network/sdn/cluster-preparing-and-sdn-enabling.html#configuring-and-connecting-underlay-networks-for-hardware-device-passthrough).

## Connecting physical network interfaces to pods

To use physical network interfaces (PF/VF) directly in pods for DPDK applications, you need to:

1. Ensure that your namespace has been [marked](../../../admin/configuration/network/sdn/cluster-preparing-and-sdn-enabling.html#preparing-namespaces-for-underlaynetwork-usage) by the administrator for use with UnderlayNetwork.
1. Create a pod with an annotation requesting the Underlay network device.

### Creating a pod with a device from the Underlay network

Create a pod that requests a device from an UnderlayNetwork. The pod annotation should specify:

* `type: "UnderlayNetwork"`: Indicates this is a physical device request
* `name: "underlay-network-name"`: The name of the UnderlayNetwork resource created by the administrator
* `bindingMode`: The binding mode for the device (VFIO-PCI, DPDK, or NetDev).

Example pod configuration for DPDK mode (universal mode that automatically selects the appropriate driver for the network adapter vendor):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: dpdk-app
  namespace: mydpdk
  annotations:
    network.deckhouse.io/networks-spec: |
      [
        {
          "type": "UnderlayNetwork",
          "name": "dpdk-shared-network",
          "bindingMode": "DPDK"
        }
      ]
spec:
  containers:
  - name: dpdk-container
    image: dpdk-app:latest
    securityContext:
      privileged: false
      capabilities:
        add:
        - NET_ADMIN
        - NET_RAW
        - IPC_LOCK
    volumeMounts:
    - mountPath: /hugepages
      name: hugepage
    resources:
      limits:
        hugepages-2Mi: 4Gi
        memory: 4Gi
        cpu: 4
      requests:
        cpu: 4
        memory: 4Gi
    command: ["/bin/sh", "-c", "sleep infinity"]
  volumes:
  - name: hugepage
    emptyDir:
      medium: HugePages
```

{% alert level="info" %}
For DPDK applications, it is important to:

* Configure `capabilities` (NET_ADMIN, NET_RAW, IPC_LOCK) to run in non-privileged mode instead of using `privileged: true`
* Mount hugepages volumes, as DPDK requires hugepages for efficient memory management.
{% endalert %}

{% alert level="info" %}
For VF devices in Shared mode, you can optionally specify a `vlanID` in the annotation to configure VLAN tagging on the VF:

```yaml
network.deckhouse.io/networks-spec: |
  [
    {
      "type": "UnderlayNetwork",
      "name": "dpdk-shared-network",
      "bindingMode": "VFIO-PCI",
      "vlanID": 100
    }
  ]
```

{% endalert %}

After creating the pod, verify that the device was allocated by checking the `network.deckhouse.io/networks-status` annotation:

```shell
d8 k -n mydpdk get pod dpdk-app -o jsonpath='{.metadata.annotations.network\.deckhouse\.io/networks-status}' | jq
```

You can also check the ResourceClaim that was automatically created:

```shell
d8 k -n mydpdk get resourceclaim
```

Example pod status with allocated UnderlayNetwork device:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: dpdk-app
  namespace: mydpdk
  annotations:
    network.deckhouse.io/networks-spec: |
      [
        {
          "type": "UnderlayNetwork",
          "name": "dpdk-shared-network",
          "bindingMode": "DPDK"
        }
      ]
    network.deckhouse.io/networks-status: |
      [
        {
          "type": "UnderlayNetwork",
          "name": "dpdk-shared-network",
          "bindingMode": "DPDK",
          "netDevInterfaces": [
            {
              "name": "ens1f0",
              "mac": "00:1b:21:bb:aa:cc"
            }
          ],
          "conditions": [
            {
              "type": "Configured",
              "status": "True",
              "reason": "InterfaceConfiguredSuccessfully",
              "message": "",
              "lastTransitionTime": "2025-01-15T10:35:00Z"
            },
            {
              "type": "Negotiated",
              "status": "True",
              "reason": "Up",
              "message": "",
              "lastTransitionTime": "2025-01-15T10:35:00Z"
            }
          ]
        }
      ]
status:
  phase: Running
  ...
```
