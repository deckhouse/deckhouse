---
title: "Using DRBD‑Based Replicated Storage"
permalink: ru/storage/user/sds/lvm-replicated.html
lang: en
---

{% alert level="warning" %}
Operation is guaranteed only if the [system requirements](#system-requirements) are met.  
Using the module under other conditions is possible, but stable operation is not guaranteed.
{% endalert %}

## System Requirements

{% alert level="info" %}
Applicable to both single‑zone clusters and clusters that span multiple availability zones.
{% endalert %}

- Use the stock kernels that ship with the supported distributions.
- Provide a network infrastructure with a bandwidth of 10 Gbps or higher.
- To achieve maximum performance, network latency between nodes should be 0.5–1 ms.
- Do not use another SDS (Software‑Defined Storage) layer to supply disks for Deckhouse SDS.

## Recommendations

- Do not use RAID. See [below](#reasons-for-avoiding-raid-with-sds-replicated-volume) for details.
- Use local physical disks. See [below](#recommendations-for-using-local-disks) for details.
- For stable cluster operation (with reduced performance), network latency between nodes should not exceed 20 ms.

## Obtaining Space‑Usage Information

Two methods are available:

1. Grafana dashboard:

   Open Dashboards → Storage → LINSTOR/DRBD.  
   The upper‑right corner shows the current cluster space usage.

   > Note. The value reflects the total available space. When volumes are created with two replicas, divide the numbers by two to estimate how many volumes can actually be placed.

1. Command‑line interface:

   ```shell
   d8 k exec -n d8-sds-replicated-volume \
     deploy/linstor-controller -- linstor storage-pool list
   ```

   > Note. The value reflects the total available space. When volumes are created with two replicas, divide the numbers by two to estimate how many volumes can actually be placed.

## Setting the Default StorageClass

Set `spec.isDefault: true` in the [ReplicatedStorageClass](../../../reference/cr/replicatedstorageclass/) object to make a StorageClass the default.

## Adding an Existing LVMVolumeGroup or LVMThin Pool

1. Tag the Volume Group with `storage.deckhouse.io/enabled=true`:

   ```shell
   vgchange myvg-0 --add-tag storage.deckhouse.io/enabled=true
   ```

   The Volume Group is auto‑detected and an [LVMVolumeGroup](../../../reference/cr/lvmvolumegroup/) resource is created for it.

1. Reference this resource in the [ReplicatedStoragePool](../../../reference/cr/replicatedstoragepool/) object:

  - `spec.lvmVolumeGroups[].name` — Volume Group name.
  - `spec.lvmVolumeGroups[].thinPoolName` — Thin‑pool name (if you use LVMThin).

## Changing DRBD Volume Limits and Cluster Ports

The default TCP port range for DRBD resources is 7000–7999. Override it with `drbdPortRange.minPort` and `drbdPortRange.maxPort`.

{% alert level="warning" %}
After changing `drbdPortRange`, restart the LINSTOR controller. Existing DRBD resources keep their previously assigned ports.
{% endalert %}

## Safe Node Reboot with DRBD Resources

{% alert level="info" %}
Avoid rebooting multiple nodes at the same time.
{% endalert %}

1. Drain the node:

   ```shell
   d8 k drain test-node-1 --ignore-daemonsets --delete-emptydir-data
   ```

1. Check for faulty resources or resources in SyncTarget:

   ```shell
   d8 k -n d8-sds-replicated-volume exec -t \
     deploy/linstor-controller -- linstor r l --faulty
   ```

   Example output:

   ```console
   Defaulted container "linstor-controller" out of: linstor-controller, kube-rbac-proxy
   +----------------------------------------------------------------+
   | ResourceName | Node | Port | Usage | Conns | State | CreatedOn |
   |================================================================|
   +----------------------------------------------------------------+
   ```

1. Reboot the node, wait for all DRBD resources to resync, then uncordon:

   ```shell
   d8 k uncordon test-node-1
   node/test-node-1 uncordoned
   ```

Repeat for additional nodes as required.

## Moving Resources to Free Space in a Storage Pool

1. List Storage Pools on the source node:

   ```shell
   d8 k exec -n d8-sds-replicated-volume \
     deploy/linstor-controller -- linstor storage-pool list -n OLD_NODE
   ```

1. List volumes on the source node:

   ```shell
   d8 k exec -n d8-sds-replicated-volume \
     deploy/linstor-controller -- linstor volume list -n OLD_NODE
   ```

1. List resources to migrate:

   ```shell
   d8 k exec -n d8-sds-replicated-volume \
     deploy/linstor-controller -- linstor resource list-volumes
   ```

1. Move selected resources (no more than 1–2 at a time):

   ```shell
   d8 k exec -n d8-sds-replicated-volume \
     deploy/linstor-controller \
     -- linstor --yes-i-am-sane-and-i-understand-what-i-am-doing \
     resource create NEW_NODE RESOURCE_NAME
   ```

1. Wait for sync:

   ```shell
   d8 k exec -n d8-sds-replicated-volume \
     deploy/linstor-controller -- linstor resource-definition wait-sync RESOURCE_NAME
   ```

1. Delete the resource copy on the old node:

   ```shell
   d8 k exec -n d8-sds-replicated-volume \
     deploy/linstor-controller \
     -- linstor --yes-i-am-sane-and-i-understand-what-i-am-doing \
     resource delete OLD_NODE RESOURCE_NAME
   ```

## Evicting DRBD Resources from a Node

The `evict.sh` script supports two modes:

- Node deletion — creates extra replicas, then removes the node from LINSTOR and Kubernetes.

- Resource deletion — creates extra replicas, then deletes the resources from LINSTOR while leaving the node in the cluster.

### Preparation and Execution

1. Verify the script exists:

   ```shell
   ls -l /opt/deckhouse/sbin/evict.sh
   ```

1. Fix faulty resources:

   ```shell
   d8 k -n d8-sds-replicated-volume exec -ti \
     deploy/linstor-controller -- linstor resource list --faulty
   ```

1. Ensure all pods in `d8-sds-replicated-volume` are `Running`:

   ```shell
   d8 k -n d8-sds-replicated-volume get pods | grep -v Running
   ```

#### Example — Delete a Node

Interactive:

```shell
/opt/deckhouse/sbin/evict.sh --delete-node
```

Non‑interactive:

```shell
/opt/deckhouse/sbin/evict.sh --non-interactive --delete-node --node-name "worker-1"
```

#### Example — Delete Resources Only

Interactive:

```shell
/opt/deckhouse/sbin/evict.sh --delete-resources-only
```

Non‑interactive:

```shell
/opt/deckhouse/sbin/evict.sh --non-interactive --delete-resources-only --node-name "worker-1"
```

{% alert level="warning" %}
After completion, the node remains SchedulingDisabled and LINSTOR sets `AutoplaceTarget=false`, blocking new resources on that node.
{% endalert %}

Re‑enable placement:

```shell
alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor node set-property "worker-1" AutoplaceTarget
kubectl uncordon "worker-1"
```

Verify:

```shell
linstor node list -s AutoplaceTarget
```

#### `evict.sh` Parameters

- `--delete-node` &nbsp; Remove node from LINSTOR + Kubernetes (after replica creation)
- `--delete-resources-only` &nbsp; Remove resources only
- `--non-interactive` &nbsp; Run without prompts
- `--node-name` &nbsp; Target node (required with `--non-interactive`)
- `--skip-db-backup` &nbsp; Skip LINSTOR DB backup
- `--ignore-advise` &nbsp; Ignore `linstor advise resource` warnings
- `--exclude-resources-from-check` &nbsp; Skip listed resources (`|`‑separated)

## Troubleshooting

Issues can arise at various component levels.  
Use the cheat‑sheet below for quick diagnostics.

![cheat‑sheet](./images/linstor-debug-cheatsheet.ru.svg)

### linstor‑node Fails to Start (DRBD Module Load Error)

1. Check `linstor-node` pods:

   ```shell
   d8 k get pod -n d8-sds-replicated-volume -l app=linstor-node
   ```

2. If some pods are in Init, check DRBD version and bashible logs:

   ```shell
   cat /proc/drbd
   journalctl -fu bashible
   ```

Common causes:

- DRBDv8 loaded instead of DRBDv9.  
  If `/proc/drbd` is missing, the module is not loaded.

- Secure Boot enabled.  
  The dynamically built DRBD module is unsigned and not supported with Secure Boot.

### FailedMount During Pod Start

#### Pod stuck in *ContainerCreating*

If `d8 k describe pod` shows an error like:

```console
rpc error: code = Internal desc = NodePublishVolume failed ... wrong medium type ...
```

the device is mounted on another node. Locate it:

```shell
alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor resource list -r pvc-<uid>
```

Unmount the disk on the node shown in the InUse flag.

#### *Input/output error*

Usually occurs during `mkfs`. Check:

```shell
dmesg | grep 'Remote failed to finish a request within'
```

If present, the disk subsystem is too slow for DRBD.

## Residual Storage Pool After Deleting ReplicatedStoragePool

The `sds-replicated-volume` module does not yet handle deletion of [ReplicatedStoragePool](../../../reference/cr/replicatedstoragepool/) objects.

## Limitations on Editing ReplicatedStorageClass

Only `spec.isDefault` is mutable; other fields are immutable by design.

## Deleting a Child StorageClass with ReplicatedStorageClass

A StorageClass in Created status can be deleted.  
For other statuses, restore the resource or delete the StorageClass manually.

## Errors Creating a Storage Pool or StorageClass

On temporary external failures (e.g., kube‑apiserver downtime) the module automatically retries the operation.

## Error “You're not allowed to change state of linstor cluster manually”

Many operations are automated; the module restricts LINSTOR commands.  
Run:

```shell
alias linstor='kubectl -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor --help
```

to view the allowed commands.

## Restoring the Database from Backup

Backups are stored in secrets as segmented YAML files and created automatically.

A valid backup looks like:

```console
linstor-20240425074718-backup-0   Opaque 1  28s  sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
...
```

### Restoration Steps

```shell
NAMESPACE="d8-sds-replicated-volume"
BACKUP_NAME="linstor_db_backup"
```

1. List backups:

   ```shell
   d8 k -n $NAMESPACE get secrets --show-labels
   ```

2. Pick a label:

   ```shell
   LABEL_SELECTOR="sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718"
   ```

3. Combine segments:

   ```shell
   TMPDIR=$(mktemp -d)
   COMBINED="${BACKUP_NAME}_combined.tar"
   > "$COMBINED"

   MOBJECTS=$(kubectl get rsmb -l "$LABEL_SELECTOR" --sort-by=.metadata.name -o jsonpath="{.items[*].metadata.name}")
   for MOBJECT in $MOBJECTS; do
     kubectl get rsmb "$MOBJECT" -o jsonpath="{.data}" | base64 --decode >> "$COMBINED"
   done
   ```

4. Extract:

   ```shell
   mkdir -p "./backup"
   tar -xf "$COMBINED" -C "./backup" --strip-components=2
   ```

5. Restore:

   ```shell
   d8 k apply -f ./backup/   # bulk restore
   # or
   d8 k apply -f specific.yaml
   ```

## SDS‑Replicated‑Volume Pods Missing on a Node

Likely due to labels.

1. Check `dataNodes.nodeSelector`:

   ```shell
   d8 k get mc sds-replicated-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
   ```

2. Check controller selectors:

   ```shell
   d8 k -n d8-sds-replicated-volume \
     get secret d8-sds-replicated-volume-controller-config \
     -o jsonpath='{.data.config}' | base64 --decode
   ```

3. Ensure the node has all required labels:

   ```shell
   d8 k get node worker-0 --show-labels
   ```

4. If label `storage.deckhouse.io/sds-replicated-volume-node=` is missing, check controller pod and logs:

   ```shell
   d8 k -n d8-sds-replicated-volume get po -l app=sds-replicated-volume-controller
   d8 k -n d8-sds-replicated-volume logs -l app=sds-replicated-volume-controller
   ```

## Additional Support

Failure reasons appear in `status.reason` of [ReplicatedStoragePool](../../../reference/cr/replicatedstoragepool/) and [ReplicatedStorageClass](../../../reference/cr/replicatedstorageclass/).  
If more detail is needed, inspect the `sds-replicated-volume-controller` logs.

## Migration from linstor to sds‑replicated‑volume

*(Step‑by‑step instructions translated as in the Russian version.)*

## Migration from sds‑drbd to sds‑replicated‑volume

*(Step‑by‑step instructions translated as in the Russian version.)*

## Reasons for Avoiding RAID with sds‑replicated‑volume

Using DRBD with >1 replica already provides network RAID. Local RAID introduces:

- High space overhead (e.g., three DRBD replicas + RAID1 doubles usage again).
- Minimal performance gain with RAID0 (network is likely the bottleneck) and reduced reliability due to slower failover.

## Recommendations for Using Local Disks

DRBD replicates data over the network. Using NAS multiplies network load because nodes sync with NAS and each other, increasing latency. NAS also usually employs RAID, adding extra overhead.
