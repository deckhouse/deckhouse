---
title: "Internal network configuration"
permalink: en/admin/configuration/network/internal/configuration.html
---

In Deckhouse Kubernetes Platform, networking is configured using CNI plugins.
The recommended option is Cilium, which suits most use cases.
Other supported CNIs include Flannel and Simple Bridge, typically used with cloud providers.

Network parameters are specified during the deployment of the DKP cluster:

- Address ranges for Pods and Services are defined.
- The network operation mode for Cilium is selected (this can be changed later).

## Internal network operation modes

The operation mode is set via the `tunnelMode` parameter in the [`cni-cilium`](/modules/cni-cilium/configuration.html) module settings.
Two modes are supported:

- Classic
- VXLAN tunnel

The appropriate mode depends on the cluster network setup:

- If all nodes are on the same network, either mode can be used.
- If nodes are on different networks, using VXLAN is recommended to create a virtual isolated network over the existing one.
  This allows communication across different network segments.

Performance differences between the modes are minimal. The table below outlines their characteristics:

| **Parameter**                  | **Classic mode**            | **VXLAN tunnel mode**         |
|-------------------------------|-----------------------------------|------------------------------------|
| **Network type**             | For nodes in the same network | Suitable for nodes in different networks |
| **Traffic isolation**          | Not provided                               | Provided                     |
| **Infrastructure usage** | Direct routing              | Virtual network over existing infrastructure |
| **Changes to MTU**             | No changes                     | MTU is reduced due to encapsulation  |
| **Additional routing** | Intermediate hop is added    | Not required                  |

### Mode configuration

The internal network mode is configured via the ModuleConfig resource named `cni-cilium`.

Example configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  version: 1
  enabled: true
  settings:
    tunnelMode: VXLAN
```

For more details on configuring the `cni-cilium` module, refer to the [module documentation](/modules/cni-cilium/configuration.html).

{% alert level="warning" %}
After changing the operation mode, reboot all nodes.
Otherwise, Pod availability issues may occur.
{% endalert %}

## Services

Services in DKP are implemented either using the selected CNI plugin or through `kube-proxy`.

They provide access to groups of Pods performing the same function and balance traffic between them.
Each Service object defines:

- A logical group of endpoints (Pods).
- A routing method for reaching them.

### Supported service types

- **ClusterIP**: Creates an internal IP address selected from the Service network.
  All traffic sent to this IP is forwarded to the Pods specified in the selector.
  This ensures load balancing.

  You can also disable IP assignment in this service type.
  In that case, the service will not be assigned an IP address from the service network, and it will resolve to the Pod IPs.
  Load balancing will be done using DNS (round-robin).
- **NodePort**: Opens the specified port on each cluster node and forwards incoming traffic to the Pods.
  For security reasons, DKP only listens on the node's internal IP.
  This behavior can be overridden by adding the annotation `node.deckhouse.io/nodeport-bind-internal-ip: "false"` to a node group.
- **LoadBalancer**: Created in the cloud provider where DKP is deployed and accepts external traffic.
  In bare-metal clusters, similar functionality is implemented using the [`metalLb`](/modules/metallb/configuration.html) module.
- **ExternalName**: A DNS record for accessing the service (acting as a CNAME).

### Configuring the service network with kube-proxy

When using Flannel or Simple Bridge, the `kube-proxy` module is required for services to function.
It replaces the default kubeadm components (DaemonSet, ConfigMap, RBAC) with its own configuration.

By default, for security reasons, when using services of NodePort type, connections are only accepted on the node's InternalIP.
This behavior can be changed by adding the following annotation to the node: `node.deckhouse.io/nodeport-bind-internal-ip: "false"`.

Example annotation for a NodeGroup:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: myng
spec:
  nodeTemplate:
    annotations:
      node.deckhouse.io/nodeport-bind-internal-ip: "false"
```

{% alert level="warning" %}
After adding, removing, or changing this annotation, you must manually restart all kube-proxy Pods.

When the [`cni-cilium`](/modules/cni-cilium/) module is enabled, the `kube-proxy` module is automatically disabled.
{% endalert %}
