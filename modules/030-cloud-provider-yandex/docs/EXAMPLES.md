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

## Managing zones

The [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) parameter defines the cluster-wide list of zones. Use this section when installing a cluster and when changing the zone list in a running cluster. Before changing a running cluster, complete the checks before changing the zone list.

In automatic mode, if [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) is not set, Deckhouse Platform uses `ru-central1-a`, `ru-central1-b`, `ru-central1-d`, and `ru-central1-e`. You can list one to four supported zones in a non-empty list. To use `ru-central1-f`, list it explicitly. For [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap), see [Using existing subnets](#using-existing-subnets).

### Selecting zones for the cluster

When it creates subnets automatically, Deckhouse Platform divides [`nodeNetworkCIDR`](cluster_configuration.html#yandexclusterconfiguration-nodenetworkcidr) into four equal parts. The `ru-central1-a` zone uses the first part, `ru-central1-b` uses the second, `ru-central1-e` uses the third, and `ru-central1-d` uses the fourth.

Select the tab that matches your zone list.

{% tabs zone_selection %}

{% tab "Default zones only" %}

When subnets are created automatically, if [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) is not set or contains only default zones, Deckhouse Platform creates or retains subnets in all four default zones. This also applies when the list contains only one zone. The zone list in a node group configuration determines which of these subnets receive nodes.

To use the default zones, do not set the [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) parameter. An empty list `[]` is allowed by the schema but differs from an unset parameter when the zone list for node groups is formed.

{% endtab %}

{% tab "The ru-central1-f zone" %}

You can list `ru-central1-f` without any default zones. For example, set the following list in the [YandexClusterConfiguration](cluster_configuration.html#yandexclusterconfiguration) resource:

```yaml
zones:
  - ru-central1-f
```

If [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap) is not set or empty, Deckhouse Platform creates a subnet only in `ru-central1-f`. It uses the part of the range released by one of the default zones. With only `ru-central1-f` in the list, it uses the part assigned to `ru-central1-a`. As long as `ru-central1-f` remains in the list, Deckhouse Platform preserves the CIDR of the existing subnet. If you remove the zone from the list, the subnet is removed with it.

When you replace a default zone with `ru-central1-f`, the new subnet receives the part of the range assigned to the excluded zone. For example, replacing `ru-central1-e` with `ru-central1-f` uses the third part of the range. Deckhouse Platform deletes the `ru-central1-e` subnet after nodes and other resources have been removed from it.

If [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) or [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) contains a zone list, add `ru-central1-f` to the groups that need nodes in this zone. For a [NodeGroup](/modules/node-manager/cr.html#nodegroup), use [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones).

{% endtab %}

{% endtabs %}

### Using existing subnets

If you pre-created subnets in Yandex Cloud and want cluster nodes to use them, map zones to subnet IDs in [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap). Deckhouse Platform uses these subnets but does not manage their lifecycle. You can specify one subnet ID for a zone.

The [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap) parameter can contain more than four zones. The four-zone limit applies only to an explicitly set [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) list. To select placement unambiguously, set [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) and include zones that are present in [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap).

The following configuration contains subnets for five zones and places nodes in two of them:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
nodeNetworkCIDR: 10.241.32.0/20
sshPublicKey: ssh-rsa AAAAB3Nza...
zones:
  - ru-central1-a
  - ru-central1-f
existingZoneToSubnetIDMap:
  ru-central1-a: <RU_CENTRAL1_A_SUBNET_ID>
  ru-central1-b: <RU_CENTRAL1_B_SUBNET_ID>
  ru-central1-d: <RU_CENTRAL1_D_SUBNET_ID>
  ru-central1-e: <RU_CENTRAL1_E_SUBNET_ID>
  ru-central1-f: <RU_CENTRAL1_F_SUBNET_ID>
```

In this example, the `ru-central1-b`, `ru-central1-d`, and `ru-central1-e` subnets remain in [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap) and in Yandex Cloud, but their zones are not selected for node placement. To use one of them, add its zone to [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) and to the zone lists of the required node groups.

If [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) is not set, Deckhouse Platform uses the subnets from [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap). For CloudEphemeral, the default zone list comes from discovery data. If the parameter contains default zones, this list contains only those zones. If the parameter contains only zones outside the default set, the list contains all zones specified in the parameter. For CloudPermanent groups without their own zone list, all subnets specified in the parameter are used. To limit placement unambiguously, set [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) explicitly and specify node group zone lists when needed.

To add a new zone, pre-create a subnet in it and add its ID to [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap). Then include the zone in [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) and in the zone lists of the node groups that should place nodes there. Keep entries for the other zones that you continue to use.

{% alert level="warning" %}
If you remove all entries from [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap), Deckhouse Platform switches to automatic subnet creation. Before removing the last entry, make sure that this is part of the intended infrastructure change.
{% endalert %}

### Changing the zone set in a running cluster

Changing the zone list does not move nodes, disks, or other resources between zones automatically. First update the zone lists of the node groups and move workloads, then update the cluster-wide [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) list. A [Yandex Cloud disk tied to a zone](https://yandex.cloud/en/docs/compute/concepts/disk) can only be attached to a node in the same zone. Move the data before changing zones by following the procedure for the relevant workload. For general information about disks and PersistentVolume, see [Yandex Cloud storage](/products/kubernetes-platform/documentation/v1/admin/integrations/public/yandex/storage.html).

{% alert level="warning" %}
A subnet cannot be deleted while nodes or other Yandex Cloud resources use it. Before changing zones, remove nodes from the excluded zone and detach other resources from the subnet to be deleted. See the [Yandex Cloud subnet deletion documentation](https://yandex.cloud/en/docs/vpc/operations/subnet-delete).
{% endalert %}

If [`withNATInstance.internalSubnetID`](cluster_configuration.html#yandexclusterconfiguration-withnatinstance-internalsubnetid) and [`withNATInstance.externalSubnetID`](cluster_configuration.html#yandexclusterconfiguration-withnatinstance-externalsubnetid) are not set in the [`WithNATInstance`](layouts.html#withnatinstance) layout, NAT instance placement is determined by [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones):

| Zone list | NAT instance placement |
| --- | --- |
| Contains only default zones | The NAT instance remains in `ru-central1-a` |
| Contains a dynamic zone and `ru-central1-a` | The NAT instance remains in `ru-central1-a` |
| Contains a zone outside the default set and excludes `ru-central1-a` | The NAT instance is recreated in the alphabetically first zone in [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) |

{% alert level="warning" %}
If [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap) is used without a `ru-central1-a` subnet, explicitly set [`withNATInstance.internalSubnetID`](cluster_configuration.html#yandexclusterconfiguration-withnatinstance-internalsubnetid) or [`withNATInstance.externalSubnetID`](cluster_configuration.html#yandexclusterconfiguration-withnatinstance-externalsubnetid). Otherwise, the NAT instance cannot determine a subnet in `ru-central1-a`.
{% endalert %}

{% alert level="warning" %}
Cluster egress is interrupted while the NAT instance is recreated.
{% endalert %}

{% alert level="warning" %}
According to Yandex Cloud information, creating and changing virtual machines in `ru-central1-b` will be unavailable from December 16, 2026. Before that date, remove nodes from the zone using one of the examples below. Retaining the subnet does not restore the ability to create or change virtual machines in the zone.
{% endalert %}

Before changing the zone list, complete the following checks:

1. Make sure that the installed Deckhouse Platform version supports the selected zones. The available [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) values are listed in the [YandexClusterConfiguration reference](cluster_configuration.html#yandexclusterconfiguration).
1. Check the cluster state and create an etcd backup. The procedure is described in [Backing up etcd](/products/kubernetes-platform/documentation/v1/admin/configuration/backup/backup-and-restore.html#etcd-backup).
1. Check the compute quota in the zones from the temporary and final node group lists. When replacing a single master node, allow resources for temporarily increasing the number of master nodes to three. The procedure for changing master nodes is described in [Managing master nodes](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#common-scaling-scenarios).
1. For CloudPermanent groups, check [`externalSubnetIDs`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-instanceclass-externalsubnetids) and [`externalIPAddresses`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-instanceclass-externalipaddresses). Nodes are assigned to zones in alphabetical order, repeating the zone list for subsequent nodes. After changing zones, make sure that each node's external subnet and external IP address match its zone.
1. For each CloudEphemeral node group, check the node management engine:

   ```shell
   d8 k get nodegroup -o custom-columns=NAME:.metadata.name,ENGINE:.status.engine
   ```

   The procedure below applies to groups with [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine) and to CloudPermanent groups. In a group with [`status.engine: CAPI`](/modules/node-manager/cr.html#nodegroup-v1-status-engine), old zonal MachineDeployments are not removed automatically. First follow the [Cluster API (CAPI) node group migration procedure](/products/kubernetes-platform/documentation/v1/faq.html#how-to-migrate-node-groups-to-cluster-api-capi), then change the cluster-wide [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) list after deleting the old group.
1. Find nodes in the excluded zone. For example, run the following command for `ru-central1-e`:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-e -o name
   ```

1. Make sure that workloads do not use disks bound to the excluded zone. Move data from those disks by following the procedure for the relevant workload.
1. Make sure that no other Yandex Cloud resources are attached to the excluded subnet.

#### Zone replacement procedure

The following example replaces `ru-central1-e` with `ru-central1-f` when subnets are created automatically. The YAML fragments show only the fields to change. Keep the other configuration fields.

{% alert level="warning" %}
The following procedure changes [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) for CloudEphemeral groups with [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine). Do not use this order for groups with [`status.engine: CAPI`](/modules/node-manager/cr.html#nodegroup-v1-status-engine). Old zonal MachineDeployments are not removed automatically, so first follow the migration procedure linked in the check step, then change the cluster-wide [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) list.
{% endalert %}

1. Set temporary zone lists without `ru-central1-e` for all node groups that use this zone.
   For CloudEphemeral groups with [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine), change [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in the relevant NodeGroups. For CloudPermanent, change [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) and [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) with `d8 system edit provider-cluster-configuration`. If a group has no zone list, set one explicitly so that its nodes leave the zone before you change the global list.

   ```yaml
   nodeGroups:
     - name: worker
       zones:
         - ru-central1-a
         - ru-central1-b
         - ru-central1-d
   masterNodeGroup:
     zones:
       - ru-central1-a
       - ru-central1-b
       - ru-central1-d
   ```

1. Apply the node group changes. For CloudPermanent, use `dhctl converge`. For CloudEphemeral groups with [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine), wait for the corresponding MachineDeployment and nodes to be removed. When changing master nodes, follow the [master node replacement procedure](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#common-scaling-scenarios).
1. Make sure that no nodes remain in `ru-central1-e`:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-e -o name
   ```

   The command must not return node names.
1. Run `d8 system edit provider-cluster-configuration` and replace `ru-central1-e` with `ru-central1-f` in [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones):

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-b
     - ru-central1-d
     - ru-central1-f
   ```

1. Apply the change with `dhctl converge`. Deckhouse Platform deletes the `ru-central1-e` subnet and creates a `ru-central1-f` subnet with the same CIDR slot.
1. Add `ru-central1-f` to the zone lists of the node groups that should place nodes there. Apply the node group changes using the method described in step 2.
1. Check that the nodes and subnets match the final configuration:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/zone
   yc vpc subnet list
   ```

If `dhctl converge` reports a CIDR overlap, wait for the old subnet deletion to finish and apply the change again. If the error persists, remove `ru-central1-f` from [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones), apply the change with `dhctl converge`, then set the final list with `ru-central1-f` and apply it again.

If the cluster already uses [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap), Deckhouse Platform does not create or delete subnets. In this case, pre-create the `ru-central1-f` subnet, add its ID to the parameter, and update [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones). Remove the `ru-central1-e` entry only after you stop using that subnet. Do not switch a cluster from automatic mode to existing subnets without a separate procedure.

#### Decommissioning the ru-central1-b zone

The following examples assume a cluster with four default zones and automatic subnet creation. If your cluster uses a different zone set, keep the required zones in the final list. For a NodeGroup, change [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones). For CloudPermanent, change [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) and [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) in YandexClusterConfiguration.

If a CloudEphemeral group uses CAPI, do not remove `ru-central1-b` by simply changing its zone list. Old zonal MachineDeployments are not removed automatically. First use the [Cluster API (CAPI) node group migration procedure](/products/kubernetes-platform/documentation/v1/faq.html#how-to-migrate-node-groups-to-cluster-api-capi), then change the cluster-wide [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) list after deleting the old group in the sequence required by your cluster configuration. For a group with [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine), change the zone list using the steps in the selected example.

Choose one of the examples. Before changing the cluster-wide zone list, remove nodes from `ru-central1-b` using the method for the node management engine.

{% tabs zones_b_migration %}

{% tab "Retain the ru-central1-b subnet" %}

Use this example if the `ru-central1-b` subnet must remain in Yandex Cloud but nodes are no longer needed in this zone.

1. Set temporary node group zone lists without `ru-central1-b` and apply the changes. For CloudPermanent, use `dhctl converge`. For CloudEphemeral groups with [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine), wait for the corresponding MachineDeployment and nodes to be removed.
1. Make sure that no nodes remain in `ru-central1-b`:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-b -o name
   ```

1. Run `d8 system edit provider-cluster-configuration` and list the remaining zones in [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones). For example:

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-d
     - ru-central1-e
   ```

1. Apply the change with `dhctl converge`. Because the list contains only default zones, Deckhouse Platform retains the subnets in all four default zones, including `ru-central1-b`.
1. If the cluster already uses [`existingZoneToSubnetIDMap`](cluster_configuration.html#yandexclusterconfiguration-existingzonetosubnetidmap), keep the `ru-central1-b` entry in the parameter. Deckhouse Platform will continue to use the mapped subnets without managing their deletion.

{% endtab %}

{% tab "Replace ru-central1-b with ru-central1-f" %}

Use this example if the final list must contain `ru-central1-f`. This example assumes automatic subnet creation.

1. Set temporary node group zone lists without `ru-central1-b` and apply the changes. For CloudPermanent, use `dhctl converge`. For CloudEphemeral groups with [`status.engine: MCM`](/modules/node-manager/cr.html#nodegroup-v1-status-engine), wait for the corresponding MachineDeployment and nodes to be removed.
1. Make sure that no nodes or other Yandex Cloud resources remain in `ru-central1-b`.
1. Run `d8 system edit provider-cluster-configuration` and replace `ru-central1-b` with `ru-central1-f` in [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones):

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-d
     - ru-central1-e
     - ru-central1-f
   ```

1. Apply the change with `dhctl converge`. Deckhouse Platform deletes the `ru-central1-b` subnet and creates a `ru-central1-f` subnet in the released CIDR slot.
1. Add `ru-central1-f` to the zone lists of the node groups that should place nodes there and apply the node group changes.
1. Check the final configuration:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/zone
   yc vpc subnet list
   ```

   No nodes from `ru-central1-b` must remain. The Yandex Cloud subnet list must contain a `ru-central1-f` subnet.

{% endtab %}

{% endtabs %}

If node removal does not complete, check the corresponding NodeGroup status and error messages. For CloudPermanent, check the `dhctl converge` output. For CloudEphemeral groups with [`status.engine: CAPI`](/modules/node-manager/cr.html#nodegroup-v1-status-engine), return to the [Cluster API (CAPI) migration procedure](/products/kubernetes-platform/documentation/v1/faq.html#how-to-migrate-node-groups-to-cluster-api-capi). Change the global [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) list only after the nodes have been removed.

Do not manually delete managed virtual machines or Machine objects. This can disrupt the node group change process.

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
