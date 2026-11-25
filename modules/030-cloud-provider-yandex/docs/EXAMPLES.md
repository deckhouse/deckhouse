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
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        cloudID: b1g0ldjdpa8rtm2ru3s4
        folderID: b1gq1vn5jqk0tmh4u7bp
    nodes:
      parameters:
        layout: Standard
        nodeNetworkCIDR: 10.241.32.0/20
        sshPublicKey: ssh-rsa AAAAB3Nza...
        zones:
          - ru-central1-a
          - ru-central1-b
        labels:
          billing: prod
        externalSubnetIDs:
          worker:
            - e2lu8r1tbbtryhdpa9ro
        externalIPAddresses:
          worker:
            - Auto
    storage:
      parameters:
        excludedStorageClasses:
          - network-hdd
        provisionedStorageClasses:
          - name: network-ssd-64k
            type: network-ssd
            blockSize: 64Ki
    ccm:
      parameters:
        additionalExternalNetworkIDs:
          - enp6t4snovl2ko4p15em
        additionalInternalNetworkIDs:
          - enpmhcdmtvs6ptdq3mkt
```

The service account key is not stored in this resource, it is set in a [separate Secret](environment.html#credentials-secret).

## Managing zones

The [`zones`](configuration.html#parameters-nodes-parameters-zones) parameter sets the cluster-wide zone list. If [`zones`](configuration.html#parameters-nodes-parameters-zones) is unset or empty, automatic subnet creation uses `ru-central1-a`, `ru-central1-b`, `ru-central1-d`, and `ru-central1-e` by default. An explicitly set non-empty list can contain one to four supported zones.

In the current set of supported zones, `ru-central1-f` is not in the default list. To use it in a four-zone list, replace one of the default zones. An example that replaces `ru-central1-e` with `ru-central1-f` is provided in ["Zone replacement procedure"](#zone-replacement-procedure). To use fewer than four zones, list only the zones where nodes should be placed.

### Selecting zones for the cluster

Before installing a cluster or changing the zone list of a running cluster, check which subnets Deckhouse Platform will create, keep, or delete.
To change zones in a running cluster, follow the [step-by-step instructions](#changing-the-zone-set-in-a-running-cluster).

To create subnets automatically, Deckhouse Platform divides [`nodeNetworkCIDR`](configuration.html#parameters-nodes-parameters-nodenetworkcidr) into four equal parts called CIDR slots. Each default zone has a fixed CIDR slot. If [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) is unset or empty, the subnets created depend on the value of [`zones`](configuration.html#parameters-nodes-parameters-zones). Select the appropriate tab. If [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) contains entries, see ["Using existing subnets"](#using-existing-subnets).

{% tabs zone_selection %}

{% tab "Default zones only" %}

If [`zones`](configuration.html#parameters-nodes-parameters-zones) is unset or empty, or contains only default zones, Deckhouse Platform creates or keeps the subnets in all four default zones, even if the list contains only one zone.
Node placement depends on the provider and node group settings.

{% endtab %}

{% tab "Not only default zones" %}

If [`zones`](configuration.html#parameters-nodes-parameters-zones) includes a zone outside the default set, such as `ru-central1-f`, Deckhouse Platform creates subnets in the selected zones and deletes subnets in the excluded default zones. For a new subnet, Deckhouse Platform uses the CIDR slot of one of the unselected default zones.

You can specify only `ru-central1-f`, without any default zones. For example, the following ModuleConfig fragment sets this list:

```yaml
spec:
  settings:
    nodes:
      parameters:
        zones:
          - ru-central1-f
```

In this case, Deckhouse Platform creates or keeps a node subnet only in `ru-central1-f`. A new subnet uses the first CIDR slot, assigned to `ru-central1-a`, which is one quarter of [`nodeNetworkCIDR`](configuration.html#parameters-nodes-parameters-nodenetworkcidr). If the `ru-central1-f` subnet already exists, it keeps its CIDR.

To place all nodes only in `ru-central1-f`, set any explicitly configured node group zone lists to this zone. In NodeGroup, change [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones). For permanent groups in YandexClusterConfiguration, change [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) and [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones).

{% alert level="warning" %}
Before changing the zone list of a running cluster, disconnect all resources using the excluded subnets. Otherwise, Deckhouse Platform cannot delete these subnets.

To keep only `ru-central1-f` in a running cluster, first [add this zone](#zone-replacement-procedure) and migrate nodes and data to it. Remove the other zones after the migration is complete.
{% endalert %}

{% endtab %}

{% endtabs %}

### Using existing subnets

If you have created subnets in Yandex Cloud and want to use them for cluster nodes, map their zones to subnet IDs in [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap). Deckhouse Platform uses these subnets but does not manage them. You can specify only one subnet ID per zone.

If [`zones`](configuration.html#parameters-nodes-parameters-zones) is unset or empty, the zones in [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) form the cluster-wide zone list, including zones outside the default set. To restrict this list, set [`zones`](configuration.html#parameters-nodes-parameters-zones) explicitly and select zones from [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap).

In the following ModuleConfig fragment, [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) contains subnets in five zones, while [`zones`](configuration.html#parameters-nodes-parameters-zones) restricts the cluster-wide list to `ru-central1-a` and `ru-central1-f`:

```yaml
spec:
  settings:
    nodes:
      parameters:
        existingZoneToSubnetIDMap:
          ru-central1-a: <RU_CENTRAL1_A_SUBNET_ID>
          ru-central1-b: <RU_CENTRAL1_B_SUBNET_ID>
          ru-central1-d: <RU_CENTRAL1_D_SUBNET_ID>
          ru-central1-e: <RU_CENTRAL1_E_SUBNET_ID>
          ru-central1-f: <RU_CENTRAL1_F_SUBNET_ID>
        zones:
          - ru-central1-a
          - ru-central1-f
```

Replace `<RU_CENTRAL1_A_SUBNET_ID>`, `<RU_CENTRAL1_B_SUBNET_ID>`, `<RU_CENTRAL1_D_SUBNET_ID>`, `<RU_CENTRAL1_E_SUBNET_ID>`, and `<RU_CENTRAL1_F_SUBNET_ID>` with subnet IDs from the corresponding zones.

The `ru-central1-b`, `ru-central1-d`, and `ru-central1-e` subnets remain in [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) and are kept in Yandex Cloud. In this configuration, their zones are outside the cluster-wide zone list for node placement.

To place new nodes in one of these subnets, include its zone in [`zones`](configuration.html#parameters-nodes-parameters-zones), keeping at most four zones in the list. If the relevant node groups have explicitly configured zone lists, add the same zone to those lists.

If [`zones`](configuration.html#parameters-nodes-parameters-zones) is unset or empty, the cluster-wide list includes all five zones from [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap).

To add a new zone, create a subnet in that zone in advance and add its ID to [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap). Keep entries for the other zones. If the [`zones`](configuration.html#parameters-nodes-parameters-zones) list is non-empty, include the new zone in it.

Then add the new zone to the settings of node groups that should create nodes in that zone. For NodeGroup, change [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones). For permanent node groups in YandexClusterConfiguration, change [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) or [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones).

{% alert level="warning" %}
Keep at least one entry in [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap). Removing the parameter or all its entries switches Deckhouse Platform to automatic subnet creation.
{% endalert %}

The four-zone limit applies to an explicit [`zones`](configuration.html#parameters-nodes-parameters-zones) list. It does not limit the number of zones in [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap).

### Changing the zone set in a running cluster

Before changing zones, review the effects and perform the checks below. Then proceed to the [zone replacement procedure](#zone-replacement-procedure).

Deckhouse Platform does not move resources between zones. When a worker NodeGroup's zones change, Deckhouse Platform removes nodes from the excluded zones. The number of nodes in the remaining zones is determined by the group settings and, for CloudEphemeral, also depends on autoscaling. A zonal volume uses a Yandex Cloud disk that can be [attached only to a node in the same zone](https://yandex.cloud/en/docs/compute/concepts/disk). Before changing zones, migrate workloads and volume data using the procedure for the corresponding application. For general information about disks and PersistentVolumes, see the [Yandex Cloud storage section](/products/kubernetes-platform/documentation/v1/admin/integrations/public/yandex/load-balancing-and-storage.html).

{% alert level="warning" %}
A subnet cannot be deleted while other Yandex Cloud resources use it. Before applying the change, remove nodes from the excluded zone, migrate volume data, and disconnect other resources from the subnets being deleted. For the required checks, see the [Yandex Cloud subnet deletion instructions](https://yandex.cloud/en/docs/vpc/operations/subnet-delete).
{% endalert %}

{% alert level="warning" %}
In the [`WithNATInstance`](layouts.html#withnatinstance) layout, check the NAT instance zone before deleting subnets. The subnet zone from [`withNATInstance.internalSubnetID`](configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetid) determines the NAT instance zone. If this parameter is unset, the subnet zone from [`withNATInstance.externalSubnetID`](configuration.html#parameters-nodes-parameters-withnatinstance-externalsubnetid) is used. If both IDs are unset, automatic subnet creation with `ru-central1-f` uses the alphabetically first zone in [`zones`](configuration.html#parameters-nodes-parameters-zones); otherwise, `ru-central1-a` is used.

If the NAT instance is in an excluded zone, migrate it before deleting subnets. To use an existing subnet in the target zone, set [`withNATInstance.internalSubnetID`](configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetid) to its ID and remove [`withNATInstance.internalSubnetCIDR`](configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetcidr) if it is set. Review the other [NAT instance settings](configuration.html#parameters-nodes-parameters-withnatinstance). The external subnet must be in the same zone as the NAT instance. Check that explicitly set [`natInstanceInternalAddress`](configuration.html#parameters-nodes-parameters-withnatinstance-natinstanceinternaladdress) and [`natInstanceExternalAddress`](configuration.html#parameters-nodes-parameters-withnatinstance-natinstanceexternaladdress) values match the new subnets and zone. Apply the changes with `dhctl converge`.

If the NAT instance internal IP address changes, update the next hop for the `0.0.0.0/0` route in the [route table](https://yandex.cloud/en/docs/vpc/cli-ref/route-table/update), keeping the other routes. Migrating the NAT instance interrupts egress until the migration and route update are complete.
{% endalert %}

If `dhctl converge` fails to create a subnet because its CIDR overlaps with a subnet being deleted, check that deletion has completed and run `dhctl converge` again. This can occur during any subnet replacement, including restoring a CIDR slot and replacing `ru-central1-b` with `ru-central1-f`.

Before changing the zone list, perform the following checks:

1. Make sure the installed Deckhouse Platform version supports the selected zones. The available values of [`zones`](configuration.html#parameters-nodes-parameters-zones) are listed in the [`cloud-provider-yandex` module settings](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-zones).
1. Check the cluster state and back up etcd. For the procedure, see the [etcd backup section](/products/kubernetes-platform/documentation/v1/admin/configuration/backup/backup-and-restore.html#backing-up-etcd).
1. Check the [available Yandex Cloud quotas](https://yandex.cloud/en/docs/compute/concepts/limits) and resource availability for creating nodes in every zone in the temporary and final node group zone lists, including zones already in use.
   In the example replacing `ru-central1-e` with `ru-central1-f`, resources are needed first in `ru-central1-a`, `ru-central1-b`, and `ru-central1-d`, then in the new `ru-central1-f` zone.
   When replacing the only master node, `dhctl converge` temporarily increases the number of master nodes to three. Allow resources for two additional master nodes in the zones from the new `master` group list.
1. Find nodes in the zone to be excluded. For example, run the following command for `ru-central1-e`:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-e -o name
   ```

   Example output:

   ```console
   node/cluster-worker-e-0
   node/cluster-worker-e-1
   ```

1. Check the placement constraints of PersistentVolumes:

   ```shell
   d8 k get pv -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.nodeAffinity}{"\n"}{end}'
   ```

   Find the name of the excluded zone in the output.
1. Migrate data from volumes available only in the excluded zone using the corresponding application's tools.
   For general information about disks and PersistentVolumes, see the [Yandex Cloud storage section](/products/kubernetes-platform/documentation/v1/admin/integrations/public/yandex/load-balancing-and-storage.html).
1. Determine the temporary and final zone lists for each node group. Use zones from the cluster-wide [`zones`](configuration.html#parameters-nodes-parameters-zones) list where nodes should be placed at each stage.
   For NodeGroups of type `CloudEphemeral`, set the list in [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) with any provider configuration method.
   For groups of type `CloudPermanent` with the provider configured through ModuleConfig, use the same NodeGroup parameter. With the legacy configuration method through YandexClusterConfiguration, change [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) and [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones).
   At the first stage, remove the zone to be excluded from the lists of groups using it. Add a new zone to groups after its subnet is created. Step-by-step examples follow below.
1. For node groups of type `CloudPermanent`, check [`externalSubnetIDs`](configuration.html#parameters-nodes-parameters-externalsubnetids) and [`externalIPAddresses`](configuration.html#parameters-nodes-parameters-externalipaddresses). Deckhouse Platform distributes these nodes across zones alphabetically, repeating the zone list for subsequent nodes. After changing zones, make sure each node's external subnet and external IP address match its zone.

#### Zone replacement procedure

First change the node group zone lists and wait for nodes to be removed from the excluded zone. Then change the cluster-wide zone list in the provider configuration.
The following example replaces `ru-central1-e` with `ru-central1-f` using automatic subnet creation. Adjust the example values for a different zone set.
The YAML fragments show only the fields to change. Keep the other settings of the existing resources.
Select the tab matching your cluster configuration method.

{% tabs zones_configuration %}

{% tab "ModuleConfig" %}

To replace a zone in a cluster configured through ModuleConfig, follow these steps:

1. Change [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in each worker [NodeGroup](/modules/node-manager/cr.html#nodegroup) that uses `ru-central1-e`.
   Temporarily keep the zones that are currently available and will remain in the final list:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-b
         - ru-central1-d
   ```

1. Remove `ru-central1-e` from [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in the `master` [NodeGroup](/modules/node-manager/cr.html#nodegroup).

1. Wait for the node group changes to complete.
   For `CloudEphemeral`, the [`node-manager`](/modules/node-manager/) module applies the change automatically.
   For groups of type `CloudPermanent`, including `master`, apply the changes using `dhctl converge`.
   The command recreates master nodes whose placement settings have changed, one at a time. Review the plan before confirming changes.
   After each master node replacement, perform the [etcd and control plane component checks](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#in-a-cloud-cluster).

1. Make sure there are no cluster nodes left in `ru-central1-e`:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-e -o name
   ```

   The command must not print any node names.
   Separately check that no other Yandex Cloud resources use the subnet to be deleted.

1. Open the module configuration with `d8 k edit mc cloud-provider-yandex` and change [`zones`](configuration.html#parameters-nodes-parameters-zones) by replacing `ru-central1-e` with `ru-central1-f`:

   ```yaml
   spec:
     settings:
       nodes:
         parameters:
           zones:
             - ru-central1-a
             - ru-central1-b
             - ru-central1-d
             - ru-central1-f
   ```

1. Apply the changes with `dhctl converge`.
   When subnets are created automatically, `ru-central1-f` receives the CIDR slot of `ru-central1-e`.
   If the CIDR slot of an existing `ru-central1-f` subnet is assigned to a default zone, follow the CIDR slot release procedure below.

1. Add `ru-central1-f` to [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in the groups that should use the new zone:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-b
         - ru-central1-d
         - ru-central1-f
   ```

   For CloudPermanent, apply the changes with `dhctl converge` again.
   For CloudEphemeral, wait until the `node-manager` module applies the changes.

{% endtab %}

{% tab "YandexClusterConfiguration" %}

To replace a zone in a cluster using the legacy configuration method through YandexClusterConfiguration, follow these steps:

1. Change [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in worker [NodeGroup](/modules/node-manager/cr.html#nodegroup) resources of type CloudEphemeral.
   Temporarily exclude `ru-central1-e`:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-b
         - ru-central1-d
   ```

   Wait until the [`node-manager`](/modules/node-manager/) module applies the changes.

1. Open the provider configuration with `d8 system edit provider-cluster-configuration`.
   Change [`zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) in the corresponding [`nodeGroups[]`](cluster_configuration.html#yandexclusterconfiguration-nodegroups) entries.
   For example, for a permanent `worker` node group, set:

   ```yaml
   nodeGroups:
     - name: worker
       zones:
         - ru-central1-a
         - ru-central1-b
         - ru-central1-d
   ```

1. Remove `ru-central1-e` from [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) if master nodes are in that zone:

   ```yaml
   masterNodeGroup:
     zones:
       - ru-central1-a
       - ru-central1-b
       - ru-central1-d
   ```

1. Wait for the node group changes to complete.
   For `CloudEphemeral`, the [`node-manager`](/modules/node-manager/) module applies the change automatically.
   For groups of type `CloudPermanent`, including `master`, apply the changes using `dhctl converge`.
   The command recreates master nodes whose placement settings have changed, one at a time. Review the plan before confirming changes.
   After each master node replacement, perform the [etcd and control plane component checks](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#in-a-cloud-cluster).

1. Make sure there are no cluster nodes left in `ru-central1-e`:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-e -o name
   ```

   The command must not print any node names.
   Separately check that no other Yandex Cloud resources use the subnet to be deleted.

1. Run `d8 system edit provider-cluster-configuration` again and change the global [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) list:

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-b
     - ru-central1-d
     - ru-central1-f
   ```

1. Apply the changes with `dhctl converge`.
   When subnets are created automatically, `ru-central1-f` receives the CIDR slot of `ru-central1-e`.
   If the CIDR slot of an existing `ru-central1-f` subnet is assigned to a default zone, follow the CIDR slot release procedure below.

1. Add `ru-central1-f` to the zone lists of groups that should use the new zone.
   For CloudEphemeral, change [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in the NodeGroup.
   For permanent groups, change [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) or [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) and apply the changes with `dhctl converge` again.
   In this example, the final group list contains `ru-central1-a`, `ru-central1-b`, `ru-central1-d`, and `ru-central1-f`.

If the cluster still uses YandexClusterConfiguration, read about [migrating the provider to ModuleConfig](/products/kubernetes-platform/documentation/v1/faq.html#how-to-migrate-a-cloud-provider-to-moduleconfig-based-configurat).

{% endtab %}

{% endtabs %}

To restore a default zone to the list when its CIDR slot is occupied by the `ru-central1-f` subnet, recreate that subnet using the procedure below.
Use this procedure if Deckhouse Platform creates the subnets and you want to keep `ru-central1-f` in the final list.
For example, when changing from `[ru-central1-a, ru-central1-b, ru-central1-d, ru-central1-f]` to `[ru-central1-a, ru-central1-b, ru-central1-e, ru-central1-f]`, first release the CIDR slot of `ru-central1-e`.
Select the tab matching your cluster configuration method.

{% tabs zones_cidr_restore %}

{% tab "ModuleConfig" %}

To release the CIDR slot and recreate the `ru-central1-f` subnet, follow these steps:

1. Remove `ru-central1-f` and `ru-central1-d` from [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in worker NodeGroups:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-b
   ```

1. Remove `ru-central1-f` and `ru-central1-d` from [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in the `master` [NodeGroup](/modules/node-manager/cr.html#nodegroup).
   When you next apply the changes using `dhctl converge`, the command recreates master nodes whose placement settings have changed, one at a time. Review the plan before confirming changes and perform the [etcd and control plane component checks](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#in-a-cloud-cluster) after each replacement.

1. Wait until the `node-manager` module applies the CloudEphemeral changes.
   For CloudPermanent, apply the changes with `dhctl converge`.
   Make sure that nodes no longer use `ru-central1-f` and `ru-central1-d` and no other Yandex Cloud resources use their subnets.

1. Run `d8 k edit mc cloud-provider-yandex` and temporarily set [`zones`](configuration.html#parameters-nodes-parameters-zones) to a list without `ru-central1-f`:

   ```yaml
   spec:
     settings:
       nodes:
         parameters:
           zones:
             - ru-central1-a
             - ru-central1-b
             - ru-central1-e
   ```

1. Apply the changes with `dhctl converge`.
   Wait until the `ru-central1-f` subnet is deleted and the `ru-central1-e` subnet is created.
   Check the subnet list:

   ```shell
   yc vpc subnet list
   ```

1. Add `ru-central1-f` back to the global [`zones`](configuration.html#parameters-nodes-parameters-zones) list:

   ```yaml
   spec:
     settings:
       nodes:
         parameters:
           zones:
             - ru-central1-a
             - ru-central1-b
             - ru-central1-e
             - ru-central1-f
   ```

1. Apply the changes with `dhctl converge`.
   In this example, Deckhouse Platform creates the `ru-central1-f` subnet in the CIDR slot of `ru-central1-d`.

1. Add `ru-central1-e` and `ru-central1-f` to [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in the groups that should use these zones.
   For CloudPermanent, apply the changes with `dhctl converge` again. For CloudEphemeral, wait until `node-manager` applies them.

{% endtab %}

{% tab "YandexClusterConfiguration" %}

To release the CIDR slot and recreate the `ru-central1-f` subnet, follow these steps:

1. Remove `ru-central1-f` and `ru-central1-d` from [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in worker CloudEphemeral NodeGroups:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-b
   ```

1. Run `d8 system edit provider-cluster-configuration` and change [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) to keep `ru-central1-a` and `ru-central1-b`.
   For master nodes, set the same list in [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones).
   When you next apply the changes using `dhctl converge`, the command recreates master nodes whose placement settings have changed, one at a time. Review the plan before confirming changes and perform the [etcd and control plane component checks](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#in-a-cloud-cluster) after each replacement.

1. Apply the permanent node group changes with `dhctl converge`.
   Wait until the `node-manager` module applies the CloudEphemeral changes.
   Make sure that nodes no longer use `ru-central1-f` and `ru-central1-d` and no other Yandex Cloud resources use their subnets.

1. Run `d8 system edit provider-cluster-configuration` again and temporarily set [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) to a list without `ru-central1-f`:

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-b
     - ru-central1-e
   ```

1. Apply the changes with `dhctl converge`.
   Wait until the `ru-central1-f` subnet is deleted and the `ru-central1-e` subnet is created.
   Check the subnet list:

   ```shell
   yc vpc subnet list
   ```

1. Add `ru-central1-f` back to the global [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) list:

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-b
     - ru-central1-e
     - ru-central1-f
   ```

1. Apply the changes with `dhctl converge`.
   In this example, Deckhouse Platform creates the `ru-central1-f` subnet in the CIDR slot of `ru-central1-d`.

1. Add `ru-central1-e` and `ru-central1-f` to the zone lists of groups that should use these zones.
   For CloudEphemeral, change [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in the NodeGroup.
   For permanent groups, change [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) or [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) and apply the changes with `dhctl converge` again.

{% endtab %}

{% endtabs %}

After completing the selected procedure, check nodes and subnets:

```shell
d8 k get nodes -L topology.kubernetes.io/zone
yc vpc subnet list
```

Make sure that nodes use only zones from the final list.
When subnets are created automatically, check that the `ru-central1-f` subnet uses the CIDR slot of the excluded default zone.

#### Decommissioning the ru-central1-b zone

To stop placing nodes in `ru-central1-b`, choose the final zone list and follow the appropriate instructions below.

{% alert level="warning" %}

From December 16, 2026, Yandex Cloud restricts creating and modifying virtual machines in the `ru-central1-b` zone. Before this date, remove the zone from node group configurations and migrate the required data to other zones. Otherwise, Deckhouse Platform will be unable to create or recreate nodes in `ru-central1-b`.

{% endalert %}

If [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) is used, first remove nodes from `ru-central1-b` by excluding it from node group zone lists.
For CloudPermanent, apply the node group changes with `dhctl converge`; for CloudEphemeral, wait for the `node-manager` module to apply them.
Then set the global [`zones`](configuration.html#parameters-nodes-parameters-zones) list to the remaining zones, even if it was previously unset, and apply the changes with `dhctl converge`.
You can keep the `ru-central1-b` entry in [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap).
Remove it only after evacuating nodes and keep entries for the other zones, as described in ["Using existing subnets"](#using-existing-subnets).

The final list can contain any combination of one to four supported zones.
In these examples, the cluster initially uses all four default zones, and Deckhouse Platform creates the subnets.
The YAML fragments show only the fields to change. Keep the other parameters of the existing resources.
In each example, select the tab matching your cluster configuration method.

{% tabs zones_b_migration %}

{% tab "Exclude ru-central1-b without adding a new zone" %}

Choose this example to continue operating in the remaining default zones, for example if quota in `ru-central1-f` is not yet available.
The final list contains `ru-central1-a`, `ru-central1-d`, and `ru-central1-e`.

{% tabs zones_b_keep_configuration %}

{% tab "ModuleConfig" %}

For a cluster configured through ModuleConfig, follow these steps:

1. Remove `ru-central1-b` from [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in each worker [NodeGroup](/modules/node-manager/cr.html#nodegroup) using this zone:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
   ```

1. Set [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in the `master` [NodeGroup](/modules/node-manager/cr.html#nodegroup) to the same zone list, without `ru-central1-b`.

1. Wait for the node group changes to complete.
   For `CloudEphemeral`, the [`node-manager`](/modules/node-manager/) module applies the change automatically.
   For groups of type `CloudPermanent`, including `master`, apply the changes using `dhctl converge`.
   The command recreates master nodes whose placement settings have changed, one at a time. Review the plan before confirming changes.
   After each master node replacement, perform the [etcd and control plane component checks](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#in-a-cloud-cluster).

1. Make sure there are no cluster nodes left in `ru-central1-b`:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-b -o name
   ```

   The command must not print any node names.

1. Open ModuleConfig with `d8 k edit mc cloud-provider-yandex` and change [`zones`](configuration.html#parameters-nodes-parameters-zones) by removing `ru-central1-b`:

   ```yaml
   spec:
     settings:
       nodes:
         parameters:
           zones:
             - ru-central1-a
             - ru-central1-d
             - ru-central1-e
   ```

1. Apply the changes using `dhctl converge`.
   Deckhouse Platform keeps the subnets in all four default zones, including `ru-central1-b`, because [`zones`](configuration.html#parameters-nodes-parameters-zones) contains only default zones.

1. Check the result:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/zone
   yc vpc subnet list
   ```

   The node list must not contain `ru-central1-b`. The subnet for this zone must remain in the subnet list.

{% endtab %}

{% tab "YandexClusterConfiguration" %}

For a cluster using the legacy configuration method through YandexClusterConfiguration, follow these steps:

1. Remove `ru-central1-b` from [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in each `CloudEphemeral` worker [NodeGroup](/modules/node-manager/cr.html#nodegroup) using this zone:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
   ```

1. Run `d8 system edit provider-cluster-configuration` and remove `ru-central1-b` from [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) for permanent worker groups and [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones). For example, set the following lists for the `worker` group and master nodes:

   ```yaml
   nodeGroups:
     - name: worker
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
   masterNodeGroup:
     zones:
       - ru-central1-a
       - ru-central1-d
       - ru-central1-e
   ```

1. Wait for the node group changes to complete.
   For `CloudEphemeral`, the [`node-manager`](/modules/node-manager/) module applies the change automatically.
   For groups of type `CloudPermanent`, including `master`, apply the changes using `dhctl converge`.
   The command recreates master nodes whose placement settings have changed, one at a time. Review the plan before confirming changes.
   After each master node replacement, perform the [etcd and control plane component checks](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#in-a-cloud-cluster).

1. Make sure there are no cluster nodes left in `ru-central1-b`:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-b -o name
   ```

   The command must not print any node names.

1. Run `d8 system edit provider-cluster-configuration` again and change the global [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) list by removing `ru-central1-b`:

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-d
     - ru-central1-e
   ```

1. Apply the changes using `dhctl converge`.
   Deckhouse Platform keeps the subnets in all four default zones, including `ru-central1-b`, because [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) contains only default zones.

1. Check the result:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/zone
   yc vpc subnet list
   ```

   The node list must not contain `ru-central1-b`. The subnet for this zone must remain in the subnet list.

{% endtab %}

{% endtabs %}

{% endtab %}

{% tab "Replace ru-central1-b with ru-central1-f" %}

Choose this example if quota in `ru-central1-f` is available and you need the new zone.
The final list contains `ru-central1-a`, `ru-central1-d`, `ru-central1-e`, and `ru-central1-f`.

{% tabs zones_b_replace_configuration %}

{% tab "ModuleConfig" %}

For a cluster configured through ModuleConfig, follow these steps:

1. Remove `ru-central1-b` from [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in each worker [NodeGroup](/modules/node-manager/cr.html#nodegroup) using this zone:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
   ```

1. Set [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in the `master` [NodeGroup](/modules/node-manager/cr.html#nodegroup) to the same zone list, without `ru-central1-b`.

1. Wait for the node group changes to complete.
   For `CloudEphemeral`, the [`node-manager`](/modules/node-manager/) module applies the change automatically.
   For groups of type `CloudPermanent`, including `master`, apply the changes using `dhctl converge`.
   The command recreates master nodes whose placement settings have changed, one at a time. Review the plan before confirming changes.
   After each master node replacement, perform the [etcd and control plane component checks](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#in-a-cloud-cluster).

1. Make sure there are no cluster nodes left in `ru-central1-b`:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-b -o name
   ```

   The command must not print any node names.
   Separately check that no other Yandex Cloud resources use the `ru-central1-b` subnet.

1. Open ModuleConfig with `d8 k edit mc cloud-provider-yandex` and change [`zones`](configuration.html#parameters-nodes-parameters-zones) by replacing `ru-central1-b` with `ru-central1-f`:

   ```yaml
   spec:
     settings:
       nodes:
         parameters:
           zones:
             - ru-central1-a
             - ru-central1-d
             - ru-central1-e
             - ru-central1-f
   ```

1. Apply the changes using `dhctl converge`.
   Deckhouse Platform deletes the `ru-central1-b` subnet and creates the `ru-central1-f` subnet in the released CIDR slot.

1. Add `ru-central1-f` to [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in groups that should create nodes in the new zone:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
         - ru-central1-f
   ```

   For groups of type `CloudPermanent`, apply the changes using `dhctl converge`. For `CloudEphemeral`, wait for the `node-manager` module to apply the changes.

1. Check the result:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/zone
   yc vpc subnet list
   ```

   The node list must not contain `ru-central1-b`.
   The subnet list must contain the `ru-central1-f` subnet and must not contain the `ru-central1-b` subnet.

{% endtab %}

{% tab "YandexClusterConfiguration" %}

For a cluster using the legacy configuration method through YandexClusterConfiguration, follow these steps:

1. Remove `ru-central1-b` from [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in each `CloudEphemeral` worker [NodeGroup](/modules/node-manager/cr.html#nodegroup) using this zone:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
   ```

1. Run `d8 system edit provider-cluster-configuration` and remove `ru-central1-b` from [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) for permanent worker groups and [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones). For example, set the following lists for the `worker` group and master nodes:

   ```yaml
   nodeGroups:
     - name: worker
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
   masterNodeGroup:
     zones:
       - ru-central1-a
       - ru-central1-d
       - ru-central1-e
   ```

1. Wait for the node group changes to complete.
   For `CloudEphemeral`, the [`node-manager`](/modules/node-manager/) module applies the change automatically.
   For groups of type `CloudPermanent`, including `master`, apply the changes using `dhctl converge`.
   The command recreates master nodes whose placement settings have changed, one at a time. Review the plan before confirming changes.
   After each master node replacement, perform the [etcd and control plane component checks](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#in-a-cloud-cluster).

1. Make sure there are no cluster nodes left in `ru-central1-b`:

   ```shell
   d8 k get nodes -l topology.kubernetes.io/zone=ru-central1-b -o name
   ```

   The command must not print any node names.
   Separately check that no other Yandex Cloud resources use the `ru-central1-b` subnet.

1. Run `d8 system edit provider-cluster-configuration` again and change the global [`zones`](cluster_configuration.html#yandexclusterconfiguration-zones) list by replacing `ru-central1-b` with `ru-central1-f`:

   ```yaml
   zones:
     - ru-central1-a
     - ru-central1-d
     - ru-central1-e
     - ru-central1-f
   ```

1. Apply the changes using `dhctl converge`.
   Deckhouse Platform deletes the `ru-central1-b` subnet and creates the `ru-central1-f` subnet in the released CIDR slot.

1. Add `ru-central1-f` to [`spec.cloudInstances.zones`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones) in `CloudEphemeral` worker NodeGroups that should create nodes in the new zone:

   ```yaml
   spec:
     cloudInstances:
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
         - ru-central1-f
   ```

   Wait for the `node-manager` module to apply the changes.

1. Add `ru-central1-f` to [`nodeGroups[].zones`](cluster_configuration.html#yandexclusterconfiguration-nodegroups-zones) and [`masterNodeGroup.zones`](cluster_configuration.html#yandexclusterconfiguration-masternodegroup-zones) for groups that should create nodes in the new zone:

   ```yaml
   nodeGroups:
     - name: worker
       zones:
         - ru-central1-a
         - ru-central1-d
         - ru-central1-e
         - ru-central1-f
   masterNodeGroup:
     zones:
       - ru-central1-a
       - ru-central1-d
       - ru-central1-e
       - ru-central1-f
   ```

   Apply the changes using `dhctl converge`.

1. Check the result:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/zone
   yc vpc subnet list
   ```

   The node list must not contain `ru-central1-b`.
   The subnet list must contain the `ru-central1-f` subnet and must not contain the `ru-central1-b` subnet.

{% endtab %}

{% endtabs %}

{% endtab %}

{% endtabs %}

If node deletion does not complete, check the NodeGroup status and error messages. For CloudPermanent, check the `dhctl converge` output. Change the global zone list only after the nodes have been deleted.

Do not manually delete managed virtual machines or Machine objects, as this can disrupt node group changes.

## An example of the YandexInstanceClass custom resource

YandexInstanceClass describes the virtual machines that Deckhouse Platform provisions for a node group. Only two parameters are required, [`cores`](cr.html#yandexinstanceclass-v1-spec-cores) and [`memory`](cr.html#yandexinstanceclass-v1-spec-memory). The other parameters are optional.

The example below also sets the OS image, the disk type and size, the share of a vCPU, and the network interface mode:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: worker
spec:
  cores: 4
  memory: 8192
  coreFraction: 50
  imageID: fd8bhu2b2bsbnqkhhd2k
  diskType: network-ssd
  diskSizeGB: 50
  networkType: SoftwareAccelerated
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
1. `yandex.cpi.flant.com/target-group-name-prefix` — specifies the name prefix of the target group used by the LoadBalancer. The annotation on the Service identifies the target group but does not select nodes. To include nodes in the target group, set the same annotation value in [`spec.nodeTemplate.annotations`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetemplate-annotations) in the NodeGroup. Yandex Cloud Controller Manager includes all suitable nodes carrying this annotation value, regardless of their NodeGroup. The target group name is formed as `<ANNOTATION_VALUE><YANDEX_CLOUD_CLUSTER_NAME><NETWORK_ID>`. For details, see ["Using a separate target group for a NodeGroup"](faq.html#using-a-separate-target-group-for-a-nodegroup).

If separate target groups are created for the control plane or master nodes, add the label `node.kubernetes.io/exclude-from-external-load-balancers: ""` to the master nodes. This prevents the controller from automatically adding master nodes to new target groups for load balancers.

If you create your own load balancer for master nodes and want YCC to also be able to place its load balancers on master nodes, pre-create a target group with a name matching the pattern `${CLUSTER-NAME}${VPC.ID}`.

### Target group health checks

Health check parameters (for LB target groups created by the controller):

1. `yandex.cpi.flant.com/healthcheck-interval-seconds` — how often to run the check, in seconds (default: 2).
1. `yandex.cpi.flant.com/healthcheck-timeout-seconds` — how long to wait for an endpoint response, in seconds. If no response is received within this time, the check is considered failed (default: 1).
1. `yandex.cpi.flant.com/healthcheck-unhealthy-threshold` — how many consecutive failed checks are required to mark an endpoint as unhealthy and exclude it from load balancing (default: 2).
1. `yandex.cpi.flant.com/healthcheck-healthy-threshold` — how many consecutive successful checks are required to return an endpoint to healthy status and include it back in load balancing (default: 2).
