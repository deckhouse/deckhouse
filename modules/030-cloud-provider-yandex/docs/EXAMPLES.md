---
title: "Cloud provider — Yandex Cloud: examples"
---

Below is an example of the Yandex Cloud cloud provider configuration.

## An example of the module configuration

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 1
  enabled: true
  settings:
    additionalExternalNetworkIDs:
    - enp6t4snovl2ko4p15em
```

## Using the ru-central1-f zone

To place nodes in `ru-central1-f`, explicitly list it in [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) in place of one of the default zones. You can list up to four zones in `zones`. If the parameter is not set, Deckhouse Platform uses `ru-central1-a`, `ru-central1-b`, `ru-central1-d`, and `ru-central1-e`.

1. Select one of the default zones to replace with `ru-central1-f`. The following example excludes `ru-central1-e`.
1. List `ru-central1-f` and the other selected zones in the [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) parameter of the [YandexClusterConfiguration](cluster_configuration.html#yandexclusterconfiguration) resource:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: YandexClusterConfiguration
   layout: Standard
   nodeNetworkCIDR: 10.241.32.0/20
   sshPublicKey: ssh-rsa AAAAB3Nza...
   zones:
     - ru-central1-a
     - ru-central1-b
     - ru-central1-d
     - ru-central1-f
   ```

   Deckhouse Platform creates a subnet in `ru-central1-f` using the part of [`nodeNetworkCIDR`](cluster_configuration.html#yandexclusterconfiguration-nodenetworkcidr) assigned to the excluded zone. In a running cluster with the default zones, replacing `ru-central1-e` with `ru-central1-f` deletes the subnet in `ru-central1-e` and creates a new one in `ru-central1-f`. Subnets in the other selected zones are retained.

### Using existing subnets

If [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap) is set, Deckhouse Platform uses the subnets specified by the parameter without creating or deleting them. The four-zone limit does not apply to `existingZoneToSubnetIDMap`. If [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) is not set, nodes are placed in all zones specified in `existingZoneToSubnetIDMap`, including zones outside the default set.

### Changing zones in a running cluster

To add `ru-central1-f`, update [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones).

{% alert level="info" %}
When you add `ru-central1-f`, Deckhouse Platform deletes subnets in the excluded default zones. Replacing `[ru-central1-a, ru-central1-b]` with `[ru-central1-a, ru-central1-b, ru-central1-f]` deletes the subnets in `ru-central1-d` and `ru-central1-e`. A subnet in `ru-central1-f` is created using the address range of one of the deleted subnets.
{% endalert %}

Changing the zone list may cause an address range conflict. This occurs if a subnet in a default zone you add needs to use the address range of the `ru-central1-f` subnet. Deckhouse Platform rejects such a change with an error. To resolve the conflict, apply the changes in two stages:

1. Remove `ru-central1-f` from [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) and apply the changes. Deckhouse Platform deletes the subnet in `ru-central1-f`.
1. Set the final zone list including `ru-central1-f` and apply the changes again.

For example, this conflict may occur when replacing `ru-central1-d` with `ru-central1-e` while keeping `ru-central1-f` in the list.

If there is no conflict, you can change the zone list directly. The `ru-central1-f` subnet retains its address range.

If NAT subnets are not explicitly specified in the [`WithNATInstance`](layouts.html#withnatinstance) layout, NAT instance placement is determined by [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones):

| Zone list | NAT instance placement |
| --- | --- |
| Contains only default zones | The NAT instance remains in `ru-central1-a` |
| Contains a zone outside the default set and excludes `ru-central1-a` | The NAT instance is recreated in the alphabetically first zone in `zones` |

{% alert level="warning" %}
Cluster egress is interrupted while the NAT instance is recreated.
{% endalert %}

## An example of the `YandexInstanceClass` custom resource

```yaml
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: test
spec:
  cores: 4
  memory: 8192
```

## LoadBalancer

### Service annotations

Default values are configured in the cluster for placing load balancer resources (the network for the target group and the subnet for the Listener). These values are set automatically during cluster setup and can be overridden with annotations at the individual Service level.

> The default behavior (external or internal LB) depends on the cluster configuration. To explicitly choose the type, use the `yandex.cpi.flant.com/loadbalancer-external` annotation.

The following annotations are supported by Yandex Cloud Controller Manager:

1. `yandex.cpi.flant.com/target-group-network-id` — specifies the NetworkID in which the target group for this Service will be created. Overrides the corresponding default value.
1. `yandex.cpi.flant.com/listener-subnet-id` — sets the SubnetID for the Listeners of the LB created for this Service. Overrides the corresponding default value.
1. `yandex.cpi.flant.com/listener-address-ipv4` — sets a predefined IPv4 address for the Listeners (supported for both internal and external LBs).
1. `yandex.cpi.flant.com/loadbalancer-external` — enables creation of an external LB for this Service (use it when you need to explicitly create an external load balancer). Overrides the default behavior.
1. `yandex.cpi.flant.com/target-group-name-prefix` — specifies the name prefix of the target group used by the LoadBalancer. The annotation on the Service identifies the target group but does not select nodes. To include nodes in the target group, set the same annotation value in [`spec.nodeTemplate.annotations`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetemplate-annotations) in the NodeGroup. Yandex Cloud Controller Manager includes all suitable nodes carrying this annotation value, regardless of their NodeGroup. The target group name is formed as `<ANNOTATION_VALUE><YANDEX_CLOUD_CLUSTER_NAME><NETWORK_ID>`. For details, see [Using a separate target group for a NodeGroup](faq.html#using-a-separate-target-group-for-a-nodegroup).

If separate target groups are created for the control plane or master nodes, add the label `node.kubernetes.io/exclude-from-external-load-balancers: ""` to the master nodes. This prevents the controller from automatically adding master nodes to new target groups for load balancers.

If you create your own load balancer for master nodes and want YCC to also be able to place its load balancers on master nodes, pre-create a target group with a name matching the pattern `${CLUSTER-NAME}${VPC.ID}`.

### Target group health checks

Health check parameters (for LB target groups created by the controller):

1. `yandex.cpi.flant.com/healthcheck-interval-seconds` — how often to run the check, in seconds (default: 2).
1. `yandex.cpi.flant.com/healthcheck-timeout-seconds` — how long to wait for an endpoint response, in seconds. If no response is received within this time, the check is considered failed (default: 1).
1. `yandex.cpi.flant.com/healthcheck-unhealthy-threshold` — how many consecutive failed checks are required to mark an endpoint as unhealthy and exclude it from load balancing (default: 2).
1. `yandex.cpi.flant.com/healthcheck-healthy-threshold` — how many consecutive successful checks are required to return an endpoint to healthy status and include it back in load balancing (default: 2).
