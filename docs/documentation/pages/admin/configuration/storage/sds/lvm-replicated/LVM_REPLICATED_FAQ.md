---
title: "Managing DRBD‑based replicated storage"
permalink: en/admin/storage/sds/lvm-replicated-faq.html
---

{% alert level="warning" %}
Functionality is guaranteed only if the [system requirements](#system-requirements) are met. Using it under other conditions is possible, but stable operation is not guaranteed.
{% endalert %}

## System requirements

{% alert level="info" %}
Applicable to both single‑zone clusters and clusters that use multiple availability zones.
{% endalert %}

- Use stock kernels shipped with the supported distributions.
- For network connectivity, use infrastructure with bandwidth of 10 Gbps or higher.
- To achieve maximum performance, network latency between nodes should be within 0.5–1 ms.
- Do not use another SDS (Software‑defined storage) to provide disks for Deckhouse SDS.

## Recommendations

- Do not use RAID. See [below](#reasons-to-avoid-raid-with-sds-replicated-volume) for details.
- Use local physical disks. See [below](#recommendations-for-using-local-disks).
- For stable cluster operation (with reduced performance), allowable network latency between nodes must not exceed 20 ms.

## Retrieving space‑usage information

There are two ways:

1. Via the Grafana dashboard:

   Go to “Dashboards” → “Storage” → “LINSTOR/DRBD”. The current cluster space usage level is shown in the upper‑right corner.

   > **Warning.** The value reflects the state of all available space. When you create volumes with two replicas, divide the reported figures by two to estimate how many volumes can actually be placed.

1. Via the command line:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor storage-pool list
   ```

   > **Warning.** The value reflects the state of all available space. When you create volumes with two replicas, divide the reported figures by two to estimate how many volumes can actually be placed.

## Assigning the default StorageClass

To designate a StorageClass as the default, set the field `spec.isDefault` to `true` in the [ReplicatedStorageClass](../../../reference/cr/replicatedstorageclass/) custom resource.

## Adding an existing LVMVolumeGroup or LVMThin pool

1. Tag the Volume Group with `storage.deckhouse.io/enabled=true`:

   ```shell
   vgchange myvg-0 --add-tag storage.deckhouse.io/enabled=true
   ```

   After that the Volume Group will be discovered automatically and a corresponding [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) resource will be created.

1. Reference this resource in [ReplicatedStoragePool](../../../reference/cr/replicatedstoragepool/) parameters using `spec.lvmVolumeGroups[].name`.  
   If an LVMThin pool is used, additionally specify its name in `spec.lvmVolumeGroups[].thinPoolName`.

## Changing DRBD volume limits and cluster ports

The default port range for DRBD resources is TCP 7000–7999. You can override it with the `drbdPortRange` setting by specifying the desired `minPort` and `maxPort` values.

{% alert level="warning" %}
After changing `drbdPortRange`, restart the LINSTOR controller so that the new settings take effect. Existing DRBD resources will retain their assigned ports.
{% endalert %}

## Correctly rebooting a node with DRBD resources

{% alert level="info" %}
To ensure stable module operation, avoid rebooting multiple nodes at the same time:
{% endalert %}

1. Drain the desired node:

   ```shell
   d8 k drain test-node-1 --ignore-daemonsets --delete-emptydir-data
   ```

1. Check for faulty DRBD resources or resources in the `SyncTarget` state. If any are found, wait for synchronization to finish or take corrective actions:

   ```shell
   d8 k -n d8-sds-replicated-volume exec -t deploy/linstor-controller -- linstor r l --faulty
   ```

   Example output:

   ```console
   Defaulted container "linstor-controller" out of: linstor-controller, kube-rbac-proxy
   +----------------------------------------------------------------+
   | ResourceName | Node | Port | Usage | Conns | State | CreatedOn |
   |================================================================|
   +----------------------------------------------------------------+
   ```

1. Reboot the node and wait for all DRBD resources to resynchronize, then run `uncordon`:

   ```shell
   d8 k uncordon test-node-1
   node/test-node-1 uncordoned
   ```

If you need to reboot another node, repeat the procedure.

## Moving resources to free up space in a Storage Pool

1. List Storage Pools:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor storage-pool list -n OLD_NODE
   ```

1. Determine the location of volumes:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor volume list -n OLD_NODE
   ```

1. Get a list of resources to move:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor resource list-volumes
   ```

1. Move the selected resources to another node (no more than 1–2 at a time):

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor --yes-i-am-sane-and-i-understand-what-i-am-doing resource create NEW_NODE RESOURCE_NAME
   ```

1. Wait for synchronization:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor resource-definition wait-sync RESOURCE_NAME
   ```

1. Delete the resource from the original node:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor --yes-i-am-sane-and-i-understand-what-i-am-doing resource delete OLD_NODE RESOURCE_NAME
   ```

## Evicting DRBD resources from a node

The `evict.sh` script performs eviction in two modes:

- **Node removal** — additional replicas are created for every resource, after which the node is removed from LINSTOR and Kubernetes.
- **Resource removal** — replicas are created for the resources, after which the resources themselves are removed from LINSTOR (the node remains in the cluster).

### Preparing and running the script

Before eviction:

1. Make sure the script is present on the master node:

   ```shell
   ls -l /opt/deckhouse/sbin/evict.sh
   ```

1. Fix faulty resources:

   ```shell
   d8 k -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor resource list --faulty
   ```

1. Ensure all pods in the `d8-sds-replicated-volume` namespace are in the `Running` state:

   ```shell
   d8 k -n d8-sds-replicated-volume get pods | grep -v Running
   ```

### Example: removing a node from LINSTOR and Kubernetes

Run `evict.sh` on any master node in interactive mode, specifying `--delete-node`:

```shell
/opt/deckhouse/sbin/evict.sh --delete-node
```

For non‑interactive mode, add `--non-interactive` and specify the node name. The script will perform all actions without prompting:

```shell
/opt/deckhouse/sbin/evict.sh --non-interactive --delete-node --node-name "worker-1"
```

### Example: removing resources from a node

Run `evict.sh` on any master node in interactive mode, specifying `--delete-resources-only`:

```shell
/opt/deckhouse/sbin/evict.sh --delete-resources-only
```

For non‑interactive mode, add `--non-interactive` and specify the node name:

```shell
/opt/deckhouse/sbin/evict.sh --non-interactive --delete-resources-only --node-name "worker-1"
```

{% alert level="warning" %}
After the script completes, the node remains in `SchedulingDisabled` status and LINSTOR sets the property `AutoplaceTarget=false`. This blocks automatic creation of new resources on the node.
{% endalert %}

To allow resource placement again, run:

```shell
alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor node set-property "worker-1" AutoplaceTarget
kubectl uncordon "worker-1"
```

Check the `AutoplaceTarget` parameter on all nodes (the field will be empty on nodes where LINSTOR resource placement is allowed):

```shell
alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor node list -s AutoplaceTarget
```

### `evict.sh` script parameters

- `--delete-node` — remove a node from LINSTOR and Kubernetes after creating additional replicas for all resources on the node;
- `--delete-resources-only` — remove resources from the node without deleting the node, after creating additional replicas;
- `--non-interactive` — run the script in non‑interactive mode;
- `--node-name` — the name of the node to evict resources from. Mandatory when using `--non-interactive`;
- `--skip-db-backup` — skip creating a LINSTOR DB backup before operations;
- `--ignore-advise` — perform operations despite `linstor advise resource` warnings;
- `--exclude-resources-from-check` — exclude resources listed with `|` from checks;

## Troubleshooting

Issues can arise at various component layers. The cheat sheet below helps diagnose volume failures in LINSTOR.

![cheat sheet](../../../images/storage/sds/lvm-replicated/linstor-debug-cheatsheet.svg)
<!-- Source: https://docs.google.com/drawings/d/19hn3nRj6jx4N_haJE0OydbGKgd-m8AUSr0IqfHfT6YA/edit -->

### linstor-node start error while loading the DRBD module

1. Check the `linstor-node` pods:

   ```shell
   d8 k get pod -n d8-sds-replicated-volume -l app=linstor-node
   ```

1. If some pods are stuck in `Init`, check the DRBD version and bashible logs on the node:

   ```shell
   cat /proc/drbd
   journalctl -fu bashible
   ```

Most likely causes:

- DRBDv8 is loaded instead of the required DRBDv9. Verify the version (if `/proc/drbd` is missing, the module is not loaded):

  ```shell
  cat /proc/drbd
  ```

  If the file is missing, the module is not loaded and the problem lies elsewhere.

- Secure Boot is enabled. Because DRBD is built dynamically (similar to dkms) without a digital signature, the module is not supported when Secure Boot is enabled.

### FailedMount error when starting a pod

#### Pod stuck in ContainerCreating

If the pod is stuck in `ContainerCreating` and `d8 k describe pod` shows errors like the one below, the device is mounted on another node:

```console
rpc error: code = Internal desc = NodePublishVolume failed for pvc-b3e51b8a-9733-4d9a-bf34-84e0fee3168d: checking
for exclusive open failed: wrong medium type, check device health
```

Check where the device is in use:

```shell
alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor resource list -r pvc-b3e51b8a-9733-4d9a-bf34-84e0fee3168d
```

The `InUse` flag shows on which node the device is used; unmount the disk manually on that node.

#### Input/output error

Such errors usually occur during filesystem creation (mkfs). Check `dmesg` on the node where the pod is starting:

```shell
dmesg | grep 'Remote failed to finish a request within'
```

If the output contains messages like *Remote failed to finish a request within …*, the disk subsystem may be too slow for DRBD to operate correctly.

## Removing a leftover Storage Pool after deleting a ReplicatedStoragePool

The `sds-replicated-volume` module does not handle deletion operations for the [ReplicatedStoragePool](../../../reference/cr/replicatedstoragepool/) resource.

## Restrictions on changing ReplicatedStorageClass spec

Only the `isDefault` field can be modified. All other parameters are immutable — this is expected behavior.

## Deleting a child StorageClass when deleting a ReplicatedStorageClass

If the StorageClass is in the `Created` status, it can be deleted. For other statuses you must restore the resource or delete the StorageClass manually.

## Errors when creating a Storage Pool or StorageClass

For temporary external issues (e.g., kube‑apiserver unavailable) the module automatically retries the failed operation.

## Error "You're not allowed to change state of linstor cluster manually"

Operations requiring manual intervention are partially or fully automated in the `sds-replicated-volume` module. Therefore, the module restricts the list of allowed LINSTOR commands. For example, creating a Tie‑Breaker is automated because LINSTOR sometimes does not create one for two‑replica resources. To see the list of allowed commands, run:

```shell
alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor --help
```

## Restoring the database from a backup

Backend resource backups are stored in Secrets as YAML files split into segments for easier restoration. Backups are created automatically on a schedule.

A correctly formed backup looks like this:

```shell
linstor-20240425074718-backup-0              Opaque                           1      28s     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
linstor-20240425074718-backup-1              Opaque                           1      28s     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
linstor-20240425074718-backup-2              Opaque                           1      28s     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
linstor-20240425074718-backup-completed      Opaque                           0      28s     <none>
```

The backup is stored in encoded segments in Secrets named `linstor-%date_time%-backup-{0..2}`. The Secret `linstor-%date_time%-backup-completed` contains no data and serves as a marker that the backup process completed successfully.

### Backup restoration procedure

1. Set environment variables:

   ```shell
   NAMESPACE="d8-sds-replicated-volume"
   BACKUP_NAME="linstor_db_backup"
   ```

1. List available backups:

   ```shell
   d8 k -n $NAMESPACE get secrets --show-labels
   ```

   Example output:

   ```shell
   linstor-20240425072413-backup-0              Opaque                           1      33m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072413
   ...
   ```

1. Each backup has its own creation‑time label. Choose the desired one and save it to an environment variable. In this example we use the most recent label:

   ```shell
   LABEL_SELECTOR="sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718"
   ```

1. Create a temporary directory to store archive parts:

   ```shell
   TMPDIR=$(mktemp -d)
   echo "Temporary directory: $TMPDIR"
   ```

1. Create an empty archive and combine Secret data into one file:

   ```shell
   COMBINED="${BACKUP_NAME}_combined.tar"
   > "$COMBINED"
   ```

1. Get the list of Secrets by label, decode the data, and append it to the archive:

   ```shell
   MOBJECTS=$(kubectl get rsmb -l "$LABEL_SELECTOR" --sort-by=.metadata.name -o jsonpath="{.items[*].metadata.name}")
   
   for MOBJECT in $MOBJECTS; do
     echo "Process: $MOBJECT"
     kubectl get rsmb "$MOBJECT" -o jsonpath="{.data}" | base64 --decode >> "$COMBINED"
   done
   ```

1. Extract the archive to get backup files:

   ```shell
   mkdir -p "./backup"
   tar -xf "$COMBINED" -C "./backup" --strip-components=2
   ```

1. Check the backup contents:

   ```shell
   ls ./backup
   ```

1. Restore the required entity by applying the corresponding YAML file:

   ```shell
   d8 k apply -f %something%.yaml
   ```

   Or bulk‑apply for a full restore:

   ```shell
   d8 k apply -f ./backup/
   ```

## Missing sds-replicated-volume service pods on a selected node

The issue is most likely related to node labels.

- Check [dataNodes.nodeSelector](./configuration.html#parameters-datanodes-nodeselector) in module settings:

  ```shell
  d8 k get mc sds-replicated-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
  ```

- Check the selectors used by `sds-replicated-volume-controller`:

  ```shell
  d8 k -n d8-sds-replicated-volume get secret d8-sds-replicated-volume-controller-config  -o jsonpath='{.data.config}' | base64 --decode
  ```

- The secret `d8-sds-replicated-volume-controller-config` must contain the selectors specified in module settings plus `kubernetes.io/os: linux`.

- Verify that the required labels are present on the node:

  ```shell
  d8 k get node worker-0 --show-labels
  ```

- If the labels are missing, add them via NodeGroup templates or directly to the node.

- If the labels exist, check whether the node has the label `storage.deckhouse.io/sds-replicated-volume-node=`.  
  If not, check whether `sds-replicated-volume-controller` is running and inspect its logs:

  ```shell
  d8 k -n d8-sds-replicated-volume get po -l app=sds-replicated-volume-controller
  d8 k -n d8-sds-replicated-volume logs -l app=sds-replicated-volume-controller
  ```

## Additional support

Reasons for failed operations are shown in the `status.reason` field of [ReplicatedStoragePool](../../../reference/cr/replicatedstoragepool/) and [ReplicatedStorageClass](../../../reference/cr/replicatedstorageclass/) resources. If that information is insufficient, consult the `sds-replicated-volume-controller` logs.

## Migration from the linstor module to sds-replicated-volume

During migration the LINSTOR control plane and its CSI are temporarily unavailable, which can affect PV operations (creation, expansion, or deletion).

{% alert level="warning" %}
User data is not affected because the migration moves to a new namespace and adds components that manage volumes.
{% endalert %}

### Migration procedure

1. Ensure no faulty resources exist in the backend. The command should return an empty list:

   ```shell
   alias linstor='kubectl -n d8-linstor exec -ti deploy/linstor-controller -- linstor'
   linstor resource list --faulty
   ```

   > **Warning.** All resources must be healthy before migration.

1. Disable the `linstor` module:

   ```shell
   d8 k patch moduleconfig linstor --type=merge -p '{"spec": {"enabled": false}}'
   ```

1. Wait until the `d8-linstor` namespace is deleted:

   ```shell
   d8 k get namespace d8-linstor
   ```

1. Create a ModuleConfig for `sds-node-configurator`:

   ```yaml
   d8 k apply -f -<<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-node-configurator
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Wait until `sds-node-configurator` reaches `Ready`:

   ```shell
   d8 k get moduleconfig sds-node-configurator
   ```

1. Create a ModuleConfig for `sds-replicated-volume`:

   > **Warning.** If `settings.dataNodes.nodeSelector` is not specified for `sds-replicated-volume`, its value will be taken from the `linstor` module. If it is absent there as well, it will remain empty and all cluster nodes will be considered data nodes.

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-replicated-volume
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Wait until `sds-replicated-volume` reaches `Ready`:

   ```shell
   d8 k get moduleconfig sds-replicated-volume
   ```

1. Check the `sds-replicated-volume` settings:

   ```shell
   d8 k get moduleconfig sds-replicated-volume -oyaml
   ```

1. Wait until all pods in the `d8-sds-replicated-volume` and `d8-sds-node-configurator` namespaces are `Ready` or `Completed`:

   ```shell
   d8 k get po -n d8-sds-node-configurator
   d8 k get po -n d8-sds-replicated-volume
   ```

1. Update the `linstor` alias and check resources:

```shell
alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor resource list --faulty
```

If no faulty resources are found, migration was successful.

### Migrating to ReplicatedStorageClass

StorageClasses in this module are managed via the `ReplicatedStorageClass` resource. StorageClasses must not be created manually.

When migrating from the LINSTOR module, delete old StorageClasses and create new ones via `ReplicatedStorageClass` according to the table below.

Note that in old StorageClasses you look at the option in the `parameters` section of the StorageClass itself, while when creating a new one you set the corresponding option in `ReplicatedStorageClass`.

| StorageClass parameter                               | ReplicatedStorageClass | Default | Notes                                                                                       |
|------------------------------------------------------|------------------------|---------|---------------------------------------------------------------------------------------------|
| linstor.csi.linbit.com/placementCount: "1"           | replication: "None"    |         | One data replica will be created                                                            |
| linstor.csi.linbit.com/placementCount: "2"           | replication: "Availability" |     | Two data replicas will be created                                                           |
| linstor.csi.linbit.com/placementCount: "3"           | replication: "ConsistencyAndAvailability" | yes | Three data replicas will be created                                                         |
| linstor.csi.linbit.com/storagePool: "name"           | storagePool: "name"    |         | Name of the storage pool used for storage                                                   |
| linstor.csi.linbit.com/allowRemoteVolumeAccess: "false" | volumeAccess: "Local" |         | Remote pod access to data volumes is forbidden (local disk access within the node only)     |

Additional parameters:

- `reclaimPolicy` (Delete, Retain) — corresponds to `reclaimPolicy` of the old StorageClass.
- `zones` — list of zones to place resources in (direct cloud zone names). Note that remote pod access to the data volume is possible only within one zone.
- `volumeAccess` values: `Local` (access strictly within the node), `EventuallyLocal` (a data replica will synchronize to the node after the pod starts), `PreferablyLocal` (remote pod access allowed, `volumeBindingMode: WaitForFirstConsumer`), `Any` (remote pod access allowed, `volumeBindingMode: Immediate`).
- If you need `volumeBindingMode: Immediate`, set `volumeAccess` in ReplicatedStorageClass to `Any`.

### Migrating to ReplicatedStoragePool

The [ReplicatedStoragePool](../../../reference/cr/replicatedstoragepool/) resource allows you to create Storage Pools in the backend. It is recommended to create this resource even for existing Storage Pools and reference existing [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/). The controller will detect that the Storage Pools already exist and leave them unchanged, showing `Created` in `status.phase`.

## Migration from the sds-drbd module to sds-replicated-volume

During migration the module control plane and its CSI are unavailable. This prevents creation, expansion, or deletion of PVs and the creation or deletion of pods that use DRBD PVs for the duration of the migration.

{% alert level="warning" %}
> **Important.** User data is not affected because migration occurs to a new namespace and new components are added to provide future volume‑management functionality.
{% endalert %}

### Migration procedure

1. Make sure there are no faulty DRBD resources in the cluster. The command should return an empty list:

   ```shell
   alias linstor='kubectl -n d8-sds-drbd exec -ti deploy/linstor-controller -- linstor'
   linstor resource list --faulty
   ```

   > **Warning.** All DRBD resources must be healthy before migration.

1. Disable the `sds-drbd` module:

   ```shell
   d8 k patch moduleconfig sds-drbd --type=merge -p '{"spec": {"enabled": false}}'
   ```

1. Wait until the `d8-sds-drbd` namespace is deleted:

   ```shell
   d8 k get namespace d8-sds-drbd
   ```

1. Create a ModuleConfig for `sds-replicated-volume`:

   > **Warning.** If `settings.dataNodes.nodeSelector` is not specified for `sds-replicated-volume`, its value will be taken from the `sds-drbd` module. If it is absent there as well, it will remain empty and all cluster nodes will be considered data nodes.

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-replicated-volume
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Wait until `sds-replicated-volume` reaches `Ready`:

   ```shell
   d8 k get moduleconfig sds-replicated-volume
   ```

1. Check the `sds-replicated-volume` settings:

   ```shell
   d8 k get moduleconfig sds-replicated-volume -oyaml
   ```

1. Wait until all pods in the `d8-sds-replicated-volume` namespace are `Ready` or `Completed`:

   ```shell
   d8 k get po -n d8-sds-replicated-volume
   ```

1. Update the `linstor` alias and check DRBD resources:

   ```shell
   alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
   linstor resource list --faulty
   ```

If no faulty resources are found, migration was successful.

> **Warning.** DRBDStoragePool and DRBDStorageClass resources will be automatically migrated to ReplicatedStoragePool and ReplicatedStorageClass. No user action is required.

The logic of these resources remains unchanged. However, verify that no DRBDStoragePool or DRBDStorageClass resources remain; if they do, please contact our technical support.

## Reasons to avoid RAID with sds-replicated-volume

Using DRBD with more than one replica already provides network‑level RAID functionality. Local RAID can cause the following issues:

- Significantly increases space overhead when using redundant RAID.  
  Example: [ReplicatedStorageClass](../../../reference/cr/replicatedstorageclass/) with `replication` set to `ConsistencyAndAvailability`. DRBD will store three data replicas (one per node). If those nodes use RAID1, storing 1 GB of data will require 6 GB of disk space. Redundant RAID is reasonable only to simplify server maintenance when storage cost is irrelevant. RAID1 then allows replacing disks without moving data replicas off a “problem” disk.

- With RAID0 the performance gain is negligible because data replication occurs over the network and the bottleneck is likely the network. Additionally, reduced host storage reliability can lead to data unavailability since DRBD failover from a broken replica to a healthy one is not instantaneous.

## Recommendations for using local disks

DRBD uses the network for data replication. When using NAS, network load increases dramatically because nodes synchronize data not only with the NAS but also with each other. Latency for reads or writes also increases. NAS typically uses RAID on its side, adding further overhead.
