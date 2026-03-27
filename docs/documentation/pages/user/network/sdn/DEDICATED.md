---
title: "Additional networks for use in application pods"
permalink: en/user/network/sdn/dedicated.html
description: |
  Creating and connecting additional software-defined networks for pods: cluster networks and project networks.
search: additional networks, project network, cluster network, Network, NetworkClass
---

DKP implements the ability to use additional software-defined networks (hereinafter referred to as additional networks) for application workloads (pods, virtual machines). You can use the following types of networks:

- Cluster (public) — a network that is publicly available in each project, configured and managed by the administrator. An example is a public WAN network or a shared network for traffic exchange between projects. To create such a network and use it for application pods, contact the cluster administrator.
- Project network (user network) — a network accessible within a namespace, created and managed by the user using the NetworkClass manifest provided by the administrator.

For more information about additional software-defined networks, see [Configuring and connecting additional virtual networks for use in application pods](../../../admin/configuration/network/sdn/configure.html#configuring-and-connecting-additional-virtual-networks-for-use-in-application-pods).

## Creating a project network (user network)

To create a network for a specific project, use the [Network](/modules/sdn/cr.html#network) and [NetworkClass](/modules/sdn/cr.html#networkclass) custom resources provided to you by the administrator:

1. Create and apply the Network manifest by specifying the name of the NetworkClass obtained from the administrator in the `spec.networkClass` field:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: Network
   metadata:
     name: my-network
     namespace: my-namespace
   spec:
     networkClass: my-network-class # The name of the NetworkClass obtained from the administrator.
   ```

   > Static identification of the VLAN ID number from the pool assigned by the cluster or network administrator is supported. If the value of the `spec.vlan.id` field is not specified, the VLAN ID will be assigned dynamically.

1. After creating the Network object you can check its status:

   ```shell
   d8 k -n my-namespace get network my-network -o yaml
   ```

   Example of the status of a Network object:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: Network
   metadata:
   ...
   status:
     bridgeName: d8-br-600
     conditions:
     - lastTransitionTime: "2025-09-29T14:51:26Z"
       message: All node interface attachments are ready
       reason: AllNodeInterfaceAttachmentsAreReady
       status: "True"
       type: AllNodeAttachementsAreReady
     - lastTransitionTime: "2025-09-29T14:51:26Z"
       message: Network is operational
       reason: NetworkReady
       status: "True"
       type: Ready
     nodeAttachementsCount: 1
     observedGeneration: 1
     readyNodeAttachementsCount: 1
     vlanID: 600
   ```

After creating a network, you can [connect it to pods](#connecting-additional-networks-to-pods).

## Connecting additional networks to pods

You can connect cluster networks and project networks to pods. To do this, use the pod annotation, specifying the parameters of the additional networks to be connected.

Example of a pod manifest with two additional networks added (the cluster network `my-cluster-network` and the project network `my-network`):

> The `ifName` field (optional) specifies the name of the TAP interface within the subnet. The `mac` field (optional) specifies the MAC address to be assigned to the TAP interface.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app-with-additional-networks
  namespace: my-namespace
  annotations:
    network.deckhouse.io/networks-spec: |
      [
        {
          "type": "Network",
          "name": "my-network",
          "ifName": "veth_mynet",
          "mac": "aa:bb:cc:dd:ee:ff"
        },
        {
          "type": "ClusterNetwork",
          "name": "my-cluster-network",
          "ifName": "veth_public"
        }
      ]
spec:
  containers:
    - name: app
    # other parameters...
```

## IPAM for additional networks (IP allocation and assignment)

The IPAM (IP Address Management) mechanism allows you to allocate IP addresses (IPv4 addresses are supported) from pools and assign them to additional network interfaces of pods connected to cluster networks and project networks.

The cluster administrator is responsible for allocating IP addresses for connection to cluster networks. They enable and configure IPAM for networks and define the IP address pool for them. Users can assign addresses and configure IPAM in project networks.

### Features of using IPAM in a DKP cluster

IPAM in a DKP cluster has the following usage features:

- IPAM is enabled **at the network level** via the `spec.ipam.ipAddressPoolRef` parameter (for ClusterNetwork IPAM, the cluster administrator enables it).
- IP address assignment to the pod interface is described in the annotation `network.deckhouse.io/networks-spec` added to the pod through the following fields:
- `ipAddressNames` — a list of [IPAddress](/modules/sdn/cr.html#ipaddress) objects to be assigned to this interface (if the parameter is not specified, IPAddress can be created automatically).
  - `skipIPAssignment` — control of IPAddress reservation/tracking. If `skipIPAssignment: true`, IPAddress reservation/tracking is enabled, but the IP address is **not assigned** to the interface within the pod (advanced usage option).
- **Only IPv4** is supported.

{% alert level="warning" %}
If multiple additional networks with IPAM enabled are connected to a single pod, it is recommended to [explicitly specify `ipAddressNames`](#manual-explicit-creation-of-ipaddress-with-type-auto) for each interface (creating separate IPAddresses). Automatically created `IPAddress` is bound to the pod and may not be suitable for multiple IPAM networks at the same time.
{% endalert %}

### Allocating a pool of IP addresses for the project network and enabling IPAM

> To allocate a pool of addresses for the project network ([Network](/modules/sdn/cr.html#network)), create an [IPAddressPool](/modules/sdn/cr.html#ipaddresspool) resource **in the same namespace** as the project network (pods connected to the network).

To allocate a pool of addresses and assign them to network interfaces of pods connected to the project network, perform the following steps:

1. Create an address pool. To do this, use the [IPAddressPool](/modules/sdn/cr.html#ipaddresspool) resource.

   Example:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: IPAddressPool
   metadata:
     name: my-net-pool
     namespace: my-namespace
   spec:
     leaseTTL: 1h
     pools:
       - network: 192.168.10.0/24
         ranges:
           - 192.168.10.50-192.168.10.200
         routes:
           - destination: 10.10.0.0/16
             via: 192.168.10.1
   ```

   > The [`spec.pools[].ranges`](/modules/sdn/cr.html#ipaddresspool-v1alpha1-spec-pools-ranges) parameter is optional. If it is not specified, the entire CIDR from [`spec.pools[].network`](/modules/sdn/cr.html#ipaddresspool-v1alpha1-spec-pools-network) is considered available (except for network/broadcast addresses, see the behavior of `/31` and `/32`).

1. Enable IPAM on the network. To do this, specify the parameters of the IPAddressPool created in the previous step in the [`spec.ipam.ipAddressPoolRef`](/modules/sdn/cr.html#network-v1alpha1-spec-ipam-ipaddresspoolref) parameter of the Network resource.

   Example:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: Network
   metadata:
     name: my-network
     namespace: my-namespace
   spec:
     networkClass: my-network-class
     ipam:
       ipAddressPoolRef:
         kind: IPAddressPool
         name: my-net-pool
   ```

After allocating a pool of IP addresses for the project network, they can be assigned to the network interfaces of the pods connected to this network.

### Assigning IP addresses to network interfaces of pods connected to an additional network

DKP implements [automatic IP address assignment](#automatic-ip-address-assignment) for additional pod interfaces, as well as the ability to [manually assign specific static IP addresses](#manually-assigning-a-static-ip-address-to-an-additional-pod-interface) to additional pod interfaces.

#### Automatic IP address assignment

To have the IP address for the additional network interface pod selected automatically from the pool, add the annotation `network.deckhouse.io/networks-spec` to the pod. In this annotation, specify the network parameters with IPAM enabled.

Example (the IP address will be automatically selected from the pool created for the `my-network` network and assigned to the `net1` interface):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app-with-ipam
  namespace: my-namespace
  annotations:
    network.deckhouse.io/networks-spec: |
      [
        {
          "type": "Network",
          "name": "my-network",
          "ifName": "net1"
        }
      ]
spec:
  containers:
    - name: app
      image: nginx
```

In this case, an [IPAddress](/modules/sdn/cr.html#ipaddress) object (type `Auto`) will be automatically created, and an IP address will be automatically selected from the pool attached to the additional network (in the example, `my-network`) and assigned to the pod network interface.

##### Manual (explicit) creation of IPAddress with type `Auto`

You can also manually create an [IPAddress](/modules/sdn/cr.html#ipaddress) object with `spec.type: Auto` (without specifying the `static.ip` parameter). In this case, the controller will allocate a free address from the pool attached to the additional network (in the example, `my-network`), and you can bind it to a specific interface using the `ipAddressNames` parameter in the `network.deckhouse.io/networks-spec annotation`.

Example:

1. Create an IPAddress object:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: IPAddress
   metadata:
     name: app-net1-auto
     namespace: my-namespace
   spec:
     networkRef:
       kind: Network          # or ClusterNetwork
       name: my-network
     type: Auto
   ```

1. Assign an IP address from the pool to the pod interface:

   ```yaml
   apiVersion: v1
   kind: Pod
   metadata:
     name: app-with-manual-auto-ip
     namespace: my-namespace
     annotations:
       network.deckhouse.io/networks-spec: |
         [
           {
             "type": "Network",
             "name": "my-network",
             "ifName": "net1",
             "ipAddressNames": ["app-net1-auto"]
           }
         ]
   spec:
     containers:
       - name: app
         image: nginx
   ```

#### Manually assigning a static IP address to an additional pod interface

To assign a specific static IP address to an additional pod interface, follow these steps:

1. Create an IPAddress in the pod namespace and specify which network it is intended for and which IP address is required:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: IPAddress
   metadata:
     name: app-net1-static
     namespace: my-namespace
   spec:
     networkRef:
       kind: Network
       name: my-network
     type: Static
     static:
       ip: 192.168.10.50
   ```

1. Connect the network to the pod and specify the created IPAddress in `ipAddressNames`:

   ```yaml
   apiVersion: v1
   kind: Pod
   metadata:
     name: app-with-static-ip
     namespace: my-namespace
     annotations:
       network.deckhouse.io/networks-spec: |
         [
           {
             "type": "Network",
             "name": "my-network",
             "ifName": "net1",
             "ipAddressNames": ["app-net1-static"]
           }
         ]
   spec:
     containers:
       - name: app
         image: nginx
   ```

### Verification of IP address assignment to an Interface

To verify that an IP address is assigned to the interface, follow these steps:

1. Check the allocated address and phase at `IPAddress` (the phase should be `Allocated`):

   ```shell
   d8 k -n my-namespace get ipaddress app-net1-static -o yaml
   ```

   Output example:

   ```console
   NAME               TYPE   KIND      NAME    ADDRESS        NETWORK           PHASE       AGE
   ipaddress-auto-1   Auto   Network   mynet   192.168.12.1   192.168.12.0/24   Allocated   4d1h
   ipaddress-auto-2   Auto   Network   mynet   192.168.12.2   192.168.12.0/24   Allocated   4d1h
   ```

1. Check the pod annotation `network.deckhouse.io/networks-status` (including `ipAddressConfigs` and routes):

   ```shell
   d8 k -n my-namespace get pod app-with-static-ip -o jsonpath='{.metadata.annotations.network\.deckhouse\.io/networks-status}   ' | jq
   ```

   Output example:

   ```json
   [
     {
       "type": "Network",
       "name": "mynet",
       "ifName": "aabbcc",
       "mac": "ae:1c:68:7a:00:8f",
       "vlanID": 0,
       "ipAddressConfigs": [
         {
           "name": "ipaddress-auto-1",
           "address": "192.168.12.1",
           "network": "192.168.12.0/24"
         }
       ],
       "conditions": [
         {
           "type": "Configured",
           "status": "True",
           "lastTransitionTime": "2026-02-26T10:06:49Z",
           "reason": "InterfaceConfiguredSuccessfully",
           "message": ""
         },
         {
           "type": "Negotiated",
           "status": "True",
           "lastTransitionTime": "2026-02-26T10:06:49Z",
           "reason": "Up",
           "message": ""
         }
       ]
     }
   ]
   ```
