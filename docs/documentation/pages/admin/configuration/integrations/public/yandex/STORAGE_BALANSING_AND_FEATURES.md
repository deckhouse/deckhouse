---
title: Storage and load balancing
permalink: en/admin/integrations/public/yandex/storage.html
---

This section covers additional aspects of Deckhouse Kubernetes Platform (DKP) integration with Yandex Cloud:

- Attaching cloud disks via CSI.
- Automatic StorageClass creation.
- Use of load balancers.
- Specifics of applying changes.
- Working with CloudStatic nodes and bastion hosts.

## Storage (CSI and StorageClass)

DKP integrates with Yandex Cloud block storage via the Container Storage Interface (CSI) component.
This allows DKP clusters to automatically provision and attach disks
and use standard Kubernetes PersistentVolumeClaim resources for working with the storage.

DKP automatically creates StorageClass resources for all supported Yandex Cloud disk types,
enabling immediate storage usage without the need to define classes manually.

The following disk types are supported:

| Disk type                 | StorageClass name          | Notes              |
|--------------------------|---------------------------|--------------------------|
| `network-hdd`            | `network-hdd`             | —                        |
| `network-ssd`            | `network-ssd`             | —                        |
| `network-ssd-nonreplicated` | `network-ssd-nonreplicated` | Size must be a multiple of 93 GB |
| `network-ssd-io-m3`      | `network-ssd-io-m3`       | Size must be a multiple of 93 GB      |

{% alert level="info" %}
The sizes of `network-ssd-nonreplicated` and `network-ssd-io-m3` disks must be multiples of 93 GB.
Otherwise, volume provisioning will fail.
{% endalert %}

### Excluding unnecessary StorageClasses

If certain disk types won’t be used in the cluster,
you can disable automatic creation of the corresponding StorageClass objects
using the [`settings.storageClass.exclude`](/modules/cloud-provider-yandex/configuration.html#parameters-storageclass-exclude) parameter in the ModuleConfig resource:

```yaml
settings:
  storageClass:
    exclude:
    - network-ssd-.*
    - network-hdd
```

In this example, DKP will not create StorageClass resources for any `network-ssd` or `network-hdd` disks.

### Setting the default StorageClass

By default, DKP determines the StorageClass using the `storageclass.kubernetes.io/is-default-class=true` annotation.

To explicitly set a different default StorageClass, use the global DKP parameter [`global.defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass).
You can modify it with the following command:

```shell
kubectl edit mc global
```

If `defaultClusterStorageClass` is not specified, DKP determines the default StorageClass in the following order:

- A StorageClass with the annotation `storageclass.kubernetes.io/is-default-class='true'` (if it exists in the cluster).
- The first StorageClass in alphabetical order auto-created by the cloud provider.
- The default value of `defaultClusterStorageClass` is an empty string (`""`).

## Load balancing

### External LoadBalancer

DKP automatically watches for Kubernetes Service objects of type LoadBalancer.
When such a service is created, DKP creates the following resources:

- **NetworkLoadBalancer**: A Yandex Cloud network load balancer.
- **TargetGroup**: A group of endpoints for traffic distribution.

These resources allow Kubernetes LoadBalancer services to receive traffic from the internet or internal networks,
depending on the configuration.

For more details on the architecture, refer to the [Kubernetes Cloud Controller Manager for Yandex Cloud documentation](https://github.com/flant/yandex-cloud-controller-manager).

### Internal LoadBalancer

To create an internal load balancer, you must explicitly specify the subnet where the load balancer listener should be created.

To do this, add the following annotation to the Service object:

```yaml
metadata:
  annotations:
    yandex.cpi.flant.com/listener-subnet-id: <SubnetID>
```

`SubnetID` refers to the ID of the subnet where the internal listener for the Yandex LoadBalancer will be created.
This allows you to control the load balancer’s network exposure and limit it to internal addresses only.

> The default behavior (external or internal LB) depends on the cluster configuration. To explicitly choose the type, use the `yandex.cpi.flant.com/loadbalancer-external` annotation.

#### Service annotations

Default values are configured in the cluster for placing load balancer resources (the network for the Target Group and the subnet for the Listener). These values are set automatically during cluster setup and can be overridden with annotations at the individual Service level.

The following annotations are supported by Yandex Cloud Controller Manager:

1. `yandex.cpi.flant.com/target-group-network-id` — specifies the NetworkID in which the Target Group for this Service will be created. Overrides the corresponding default value.
1. `yandex.cpi.flant.com/listener-subnet-id` — sets the SubnetID for the Listeners of the LB created for this Service. Overrides the corresponding default value.
1. `yandex.cpi.flant.com/listener-address-ipv4` — sets a predefined IPv4 address for the Listeners (supported for both internal and external LBs).
1. `yandex.cpi.flant.com/loadbalancer-external` — enables creation of an external LB for this Service (use it when you need to explicitly create an external load balancer). Overrides the default behavior.
1. `yandex.cpi.flant.com/target-group-name-prefix` — sets a Target Group name prefix in the format `<annotation value><Yandex cluster name><NetworkID>` (for a Service). A similar annotation can be set on a node to include the node in a non-standard Target Group (Target Groups will be created with names `<annotation value><Yandex cluster name><network id of the instance’s interfaces>`).

If separate Target Groups are created for the control plane or master nodes, add the label `node.kubernetes.io/exclude-from-external-load-balancers: ""` to the master nodes. This prevents the controller from automatically adding master nodes to new Target Groups for load balancers.
If you create your own load balancer for master nodes and want YCC to also be able to place its load balancers on master nodes, pre-create a Target Group with a name matching the pattern `${CLUSTER-NAME}${VPC.ID}`.

#### Target Group health checks

Health check parameters (for LB Target Groups created by the controller):

1. `yandex.cpi.flant.com/healthcheck-interval-seconds` — how often to run the check, in seconds (default: 2).
1. `yandex.cpi.flant.com/healthcheck-timeout-seconds` — how long to wait for an endpoint response, in seconds. If no response is received within this time, the check is considered failed (default: 1).
1. `yandex.cpi.flant.com/healthcheck-unhealthy-threshold` — how many consecutive failed checks are required to mark an endpoint as unhealthy and exclude it from load balancing (default: 2).
1. `yandex.cpi.flant.com/healthcheck-healthy-threshold` — how many consecutive successful checks are required to return an endpoint to healthy status and include it back in load balancing (default: 2).

## Applying changes

DKP does not recreate existing Machine objects when configuration parameters change.
Node recreation occurs only when:

- [NodeGroup](/modules/node-manager/cr.html#nodegroup) section parameters change.
- [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) parameters change.

This behavior helps prevent unnecessary operations and node idling, but it means you must manually recreate VMs if needed.

If you change the [YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) resource (for example, change provider settings, layout, subnets, etc.),
run the following command to apply the changes:

```shell
dhctl converge
```

This command reconciles the cluster state with the configuration defined in the resources.

## Integrating manually created VMs

DKP allows you to connect existing VMs in Yandex Cloud to the Kubernetes cluster as nodes.
These nodes are called CloudStatic and they are not directly managed by the [`node-manager`](/modules/node-manager/) module
but can still be used in the cluster.

To manually connect a VM as a CloudStatic node, follow these steps:

1. Retrieve the current `nodeNetworkCIDR` value from the cluster:

   ```shell
   kubectl -n kube-system get secret d8-provider-cluster-configuration -o json | \
     jq --raw-output '.data."cloud-provider-cluster-configuration.yaml"' | base64 -d | grep '^nodeNetworkCIDR'
   ```

   Expected output:

   ```console
   nodeNetworkCIDR: 192.168.12.13/24
   ```

   Copy this value and use it in the VM metadata as `value`.

1. Set the `node-network-cidr` parameter in the VM metadata:

   ```yaml
   key: node-network-cidr
   value: <nodeNetworkCIDR from the cluster>
   ```

   The `node-network-cidr` parameter must match the value specified in the YandexClusterConfiguration resource under [`nodeNetworkCIDR`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-nodenetworkcidr).
