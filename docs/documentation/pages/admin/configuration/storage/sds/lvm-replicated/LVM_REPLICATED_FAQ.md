---
title: "Managing DRBD‑based replicated storage"
permalink: en/admin/configuration/storage/sds/lvm-replicated-faq.html
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

There are two ways to obtain this information:

1. Via the Grafana dashboard:

   Go to "Dashboards" → "Storage" → "LINSTOR/DRBD". The current cluster space usage level is shown in the upper‑right corner.

   > **Warning.** The value reflects the state of all available space. When you create volumes with two replicas, divide the reported figures by two to estimate how many volumes can actually be placed.

1. Via the command line:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor storage-pool list
   ```

   > **Warning.** The value reflects the state of all available space. When you create volumes with two replicas, divide the reported figures by two to estimate how many volumes can actually be placed.

## Assigning the default StorageClass

To designate a StorageClass as the default, set the field `spec.isDefault` to `true` in the [ReplicatedStorageClass](/modules/sds-replicated-volume/cr.html#replicatedstorageclass) custom resource.

## Adding an existing LVMVolumeGroup

1. Assign the tag `storage.deckhouse.io/enabled=true` to the Volume Group:

   ```shell
   vgchange myvg-0 --add-tag storage.deckhouse.io/enabled=true
   ```

   After that the Volume Group will be discovered automatically and a corresponding [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resource will be created.

1. Reference this resource in [ReplicatedStoragePool](/modules/sds-replicated-volume/cr.html#replicatedstoragepool) parameters using `spec.lvmVolumeGroups[].name`.  
   If an LVMThin pool is used, additionally specify its name in `spec.lvmVolumeGroups[].thinPoolName`.

## Changing DRBD volume limits and cluster ports

The default port range for DRBD resources is TCP `7000`–`7999`. You can override it with the [`drbdPortRange`](/modules/sds-replicated-volume/stable/configuration.html#parameters-drbdportrange) setting by specifying the desired `minPort` and `maxPort` values.

{% alert level="warning" %}
After changing `drbdPortRange`, restart the LINSTOR controller so that the new settings take effect. Existing DRBD resources will retain their assigned ports.
{% endalert %}

## Correctly rebooting a node with DRBD resources

{% alert level="info" %}
To ensure stable module operation, avoid rebooting multiple nodes at the same time.
{% endalert %}

1. Drain the target node:

   ```shell
   d8 k drain test-node-1 --ignore-daemonsets --delete-emptydir-data
   ```

1. Make sure there are no faulty DRBD resources or resources in the `SyncTarget` state. To check this, run the following command and analyze the output:

   ```shell
   d8 k -n d8-sds-replicated-volume exec -t deploy/linstor-controller -- linstor r l --faulty
   ```

   If any resources are in the `SyncTarget` state, wait for the synchronization to complete or take corrective actions.

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

1. List the Storage Pools on the source node to identify which one is running low on free space:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor storage-pool list -n OLD_NODE
   ```

1. Determine which volumes are located in the overloaded Storage Pool to identify potential candidates for migration:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor volume list -n OLD_NODE
   ```

1. Get the list of resources that own these volumes so that you can proceed with moving them:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor resource list-volumes
   ```

1. Create copies of the selected resources on another node (no more than 1–2 at a time):

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor --yes-i-am-sane-and-i-understand-what-i-am-doing resource create NEW_NODE RESOURCE_NAME
   ```

1. Wait for the resource synchronization to complete to ensure the data has been replicated properly:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor resource-definition wait-sync RESOURCE_NAME
   ```

1. Remove the resource from the source node to free up space in the overloaded Storage Pool:

   ```shell
   d8 k exec -n d8-sds-replicated-volume deploy/linstor-controller -- linstor --yes-i-am-sane-and-i-understand-what-i-am-doing resource delete OLD_NODE RESOURCE_NAME
   ```

## Automatic management of replicas and monitoring of LINSTOR state

Replica management and state monitoring are automated in the `replicas_manager.sh` script.
It checks the availability of the LINSTOR controller, identifies faulty or corrupted resources, creates database backups, and manages disk replicas, including configuring `TieBreaker` for quorum.

To check the existence of the `replicas_manager.sh` script, run the following command on any master node:

   ```shell
   ls -l /opt/deckhouse/sbin/replicas_manager.sh
   ```

Upon execution, the script performs the following actions:

- Verifies the availability of the controller and connectivity to satellites.
- Identifies faulty or corrupted resources.
- Creates a backup of the database.
- Manages the number of disk replicas, adding new ones as needed.
- Configures TieBreaker for resources with two replicas.
- Logs all actions to a file named `linstor_replicas_manager_<date_time>.log`.
- Provides recommendations for resolving issues, such as stuck replicas.

Configuration variables for `replicas_manager.sh`:

- `NON_INTERACTIVE`: Enables non-interactive mode.
- `TIMEOUT_SEC`: Timeout between attempts, in seconds (default: 10).
- `EXCLUDED_RESOURCES_FROM_CHECK`: Regular expression to exclude resources from checks.
- `CHUNK_SIZE`: Chunk size for processing resources (default: 10).
- `NODE_FOR_EVICT`: The name of the node excluded from creating replicas.
- `LINSTOR_NAMESPACE`: Kubernetes namespace (default: `d8-sds-replicated-volume`).
- `DISKLESS_STORAGE_POOL`: Pool for diskless replicas (default: `DfltDisklessStorPool`).

## Evicting DRBD resources from a node

Eviction of DRBD resources from a node is performed using the `evict.sh` script. It can operate in two modes:

- Node removal: Additional replicas are created for every resource, after which the node is removed from LINSTOR and Kubernetes.
- Resource removal: Replicas are created for the resources, after which the resources themselves are removed from LINSTOR (the node remains in the cluster).

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

1. Ensure all Pods in the `d8-sds-replicated-volume` namespace are in the `Running` state:

   ```shell
   d8 k -n d8-sds-replicated-volume get pods | grep -v Running
   ```

### Example of removing a node from LINSTOR and Kubernetes

Run `evict.sh` on any master node in interactive mode, specifying `--delete-node`:

```shell
/opt/deckhouse/sbin/evict.sh --delete-node
```

For non‑interactive mode, add `--non-interactive` and specify the node name. The script will perform all actions without prompting:

```shell
/opt/deckhouse/sbin/evict.sh --non-interactive --delete-node --node-name "worker-1"
```

### Example of removing resources from a node

Run `evict.sh` on any master node in interactive mode, specifying `--delete-resources-only`:

```shell
/opt/deckhouse/sbin/evict.sh --delete-resources-only
```

For non‑interactive mode, add `--non-interactive` and specify the node name. In this mode, the script runs through all actions without asking for confirmation:

```shell
/opt/deckhouse/sbin/evict.sh --non-interactive --delete-resources-only --node-name "worker-1"
```

{% alert level="warning" %}
After the script completes, the node remains in `SchedulingDisabled` status and LINSTOR sets the property `AutoplaceTarget=false`. This blocks automatic creation of new resources on the node.
{% endalert %}

To allow resource placement again, run:

```shell
alias linstor='d8 k -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor node set-property "worker-1" AutoplaceTarget
d8 k uncordon "worker-1"
```

Check the `AutoplaceTarget` parameter on all nodes (the field will be empty on nodes where LINSTOR resource placement is allowed):

```shell
alias linstor='d8 k -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor node list -s AutoplaceTarget
```

### Parameters of the evict.sh script

- `--delete-node`: Remove a node from LINSTOR and Kubernetes after creating additional replicas for all resources on the node.
- `--delete-resources-only`: Remove resources from the node without deleting the node from LINSTOR and Kubernetes, after creating additional replicas.
- `--non-interactive`: Run the script in non‑interactive mode.
- `--node-name`: The name of the node to evict resources from. Mandatory when using `--non-interactive`.
- `--skip-db-backup`: Skip creating a LINSTOR DB backup before operations.
- `--ignore-advise`: Perform operations despite `linstor advise resource` warnings. Use if the script was interrupted and the number of replicas for some resources does not match the value specified in the `ReplicatedStorageClass`.
- `--exclude-resources-from-check`: Exclude resources listed with `|` from checks.

## Troubleshooting

Issues can arise at various component layers. The cheat sheet below helps diagnose volume failures in LINSTOR.

![Volume failure diagnostics in LINSTOR](../../../../images/storage/sds/lvm-replicated/linstor-debug-cheatsheet.svg)
<!-- Source: https://docs.google.com/drawings/d/19hn3nRj6jx4N_haJE0OydbGKgd-m8AUSr0IqfHfT6YA/edit -->

### Start error of linstor-node while loading the DRBD module

1. Check the `linstor-node` Pods:

   ```shell
   d8 k get pod -n d8-sds-replicated-volume -l app=linstor-node
   ```

1. If some Pods are stuck in `Init`, check the DRBD version and bashible logs on the node:

   ```shell
   cat /proc/drbd
   journalctl -fu bashible
   ```

Most likely causes:

- DRBDv8 is loaded instead of the required DRBDv9. Verify the version:

  ```shell
  cat /proc/drbd
  ```

  If the `/proc/drbd` file is missing, the module is not loaded and the problem lies elsewhere.

- Secure Boot is enabled. Because DRBD is built dynamically (similar to dkms) without a digital signature, the module is not supported when Secure Boot is enabled.

### FailedMount error when starting a Pod

#### Pod stuck in ContainerCreating

If the Pod is stuck in `ContainerCreating` and `d8 k describe pod` shows errors like the one below, the device is mounted on another node:

```console
rpc error: code = Internal desc = NodePublishVolume failed for pvc-b3e51b8a-9733-4d9a-bf34-84e0fee3168d: checking
for exclusive open failed: wrong medium type, check device health
```

Check where the device is in use:

```shell
alias linstor='d8 k -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor resource list -r pvc-b3e51b8a-9733-4d9a-bf34-84e0fee3168d
```

The `InUse` flag shows on which node the device is used; unmount the disk manually on that node.

#### Input/output error

Such errors usually occur during filesystem creation (mkfs). Check `dmesg` on the node where the Pod is starting:

```shell
dmesg | grep 'Remote failed to finish a request within'
```

If the output contains messages like `Remote failed to finish a request within …`, the disk subsystem may be too slow for DRBD to operate correctly.

## After the ReplicatedStoragePool resource is deleted, the corresponding Storage Pool remains in the backend

This is expected behavior. The [`sds-replicated-volume`](/modules/sds-replicated-volume/) module does not handle deletion operations for the [ReplicatedStoragePool](/modules/sds-replicated-volume/cr.html#replicatedstoragepool) resource.

## Restrictions on changing ReplicatedStorageClass spec

Only the `isDefault` field can be modified. All other parameters are immutable — this is expected behavior.

## Deleting a child StorageClass when deleting a ReplicatedStorageClass

If the StorageClass is in the `Created` status, it can be deleted. For other statuses you must restore the resource or delete the StorageClass manually.

## Errors when creating a Storage Pool or StorageClass

For temporary external issues (for example, when `kube‑apiserver` is unavailable) the module automatically retries the failed operation.

## Error "You're not allowed to change state of linstor cluster manually"

Operations requiring manual intervention are partially or fully automated in the [`sds-replicated-volume`](/modules/sds-replicated-volume/) module. Therefore, the module restricts the list of allowed LINSTOR commands. For example, creating a Tie‑Breaker is automated because LINSTOR sometimes does not create one for two‑replica resources. To see the list of allowed commands, run:

```shell
alias linstor='d8 k -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
linstor --help
```

## Restoring the database from a backup

Backend resource backups are stored in Secrets as YAML files split into segments for easier restoration. Backups are created automatically on a schedule.

A correctly formed backup looks like this:

```console
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
   linstor-20240425072413-backup-1              Opaque                           1      33m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072413
   linstor-20240425072413-backup-2              Opaque                           1      33m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072413
   linstor-20240425072413-backup-completed      Opaque                           0      33m     <none>
   linstor-20240425072510-backup-0              Opaque                           1      32m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072510
   linstor-20240425072510-backup-1              Opaque                           1      32m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072510
   linstor-20240425072510-backup-2              Opaque                           1      32m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072510
   linstor-20240425072510-backup-completed      Opaque                           0      32m     <none>
   linstor-20240425072634-backup-0              Opaque                           1      31m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072634
   linstor-20240425072634-backup-1              Opaque                           1      31m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072634
   linstor-20240425072634-backup-2              Opaque                           1      31m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072634
   linstor-20240425072634-backup-completed      Opaque                           0      31m     <none>
   linstor-20240425072918-backup-0              Opaque                           1      28m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072918
   linstor-20240425072918-backup-1              Opaque                           1      28m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072918
   linstor-20240425072918-backup-2              Opaque                           1      28m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425072918
   linstor-20240425072918-backup-completed      Opaque                           0      28m     <none>
   linstor-20240425074718-backup-0              Opaque                           1      10m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
   linstor-20240425074718-backup-1              Opaque                           1      10m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
   linstor-20240425074718-backup-2              Opaque                           1      10m     sds-replicated-volume.deckhouse.io/linstor-db-backup=20240425074718
   linstor-20240425074718-backup-completed      Opaque                           0      10m     <none>
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
   MOBJECTS=$(d8 k get rsmb -l "$LABEL_SELECTOR" --sort-by=.metadata.name -o jsonpath="{.items[*].metadata.name}")
   
   for MOBJECT in $MOBJECTS; do
     echo "Process: $MOBJECT"
     d8 k get rsmb "$MOBJECT" -o jsonpath="{.data}" | base64 --decode >> "$COMBINED"
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
   TMPDIR=$(mktemp -d)
   echo "Temporary directory: $TMPDIR"
   ```

    Example output:

   ```console
   ebsremotes.yaml                    layerdrbdvolumedefinitions.yaml        layerwritecachevolumes.yaml  propscontainers.yaml      satellitescapacity.yaml  secidrolemap.yaml         trackingdate.yaml
   files.yaml                         layerdrbdvolumes.yaml                  linstorremotes.yaml          resourceconnections.yaml  schedules.yaml           secobjectprotection.yaml  volumeconnections.yaml
   keyvaluestore.yaml                 layerluksvolumes.yaml                  linstorversion.yaml          resourcedefinitions.yaml  secaccesstypes.yaml      secroles.yaml             volumedefinitions.yaml
   layerbcachevolumes.yaml            layeropenflexresourcedefinitions.yaml  nodeconnections.yaml         resourcegroups.yaml       secaclmap.yaml           sectyperules.yaml         volumegroups.yaml
   layercachevolumes.yaml             layeropenflexvolumes.yaml              nodenetinterfaces.yaml       resources.yaml            secconfiguration.yaml    sectypes.yaml             volumes.yaml
   layerdrbdresourcedefinitions.yaml  layerresourceids.yaml                  nodes.yaml                   rollback.yaml             secdfltroles.yaml        spacehistory.yaml
   layerdrbdresources.yaml            layerstoragevolumes.yaml               nodestorpool.yaml            s3remotes.yaml            secidentities.yaml       storpooldefinitions.yaml
   ```

1. Restore the required entity by applying the corresponding YAML file:

   ```shell
   d8 k apply -f %something%.yaml
   ```

   Or bulk‑apply for a full restore:

   ```shell
   d8 k apply -f ./backup/
   ```

## Missing sds-replicated-volume service Pods on a selected node

The issue is most likely related to node labels.

- Check [`dataNodes.nodeSelector`](/modules/sds-local-volume/configuration.html#parameters-datanodes-nodeselector) in module settings:

  ```shell
  d8 k get mc sds-replicated-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
  ```

- Check the selectors used by `sds-replicated-volume-controller`:

  ```shell
  d8 k -n d8-sds-replicated-volume get secret d8-sds-replicated-volume-controller-config  -o jsonpath='{.data.config}' | base64 --decode
  ```

- The secret `d8-sds-replicated-volume-controller-config` must contain the selectors specified in module settings plus `kubernetes.io/os: linux`.

- Verify that the all labels from the `d8-sds-replicated-volume-controller-config` secret are present on the node:

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

Reasons for failed operations are shown in the `status.reason` field of [ReplicatedStoragePool](/modules/sds-replicated-volume/cr.html#replicatedstoragepool) and [ReplicatedStorageClass](/modules/sds-replicated-volume/cr.html#replicatedstorageclass) resources. If that information is insufficient, consult the `sds-replicated-volume-controller` logs.

## Migration from the linstor module to sds-replicated-volume

During migration the LINSTOR control plane and its CSI are temporarily unavailable, which can affect PV operations (creation, expansion, or deletion).

{% alert level="warning" %}
User data is not affected because the migration moves to a new namespace and adds components that manage volumes.
{% endalert %}

### Migration procedure

1. Ensure no faulty resources exist in the backend. The command should return an empty list:

   ```shell
   alias linstor='d8 k -n d8-linstor exec -ti deploy/linstor-controller -- linstor'
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

1. Create a ModuleConfig for [`sds-node-configurator`](/modules/sds-node-configurator/):

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

1. Create a ModuleConfig for [`sds-replicated-volume`](/modules/sds-replicated-volume/):

   > **Warning.** If `settings.dataNodes.nodeSelector` is not specified for `sds-replicated-volume`, its value will be taken from the `linstor` module. If it is absent there as well, it will remain empty and all cluster nodes will be considered data nodes.

   ```shell
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

1. Wait until all Pods in the `d8-sds-replicated-volume` and `d8-sds-node-configurator` namespaces are `Ready` or `Completed`:

   ```shell
   d8 k get po -n d8-sds-node-configurator
   d8 k get po -n d8-sds-replicated-volume
   ```

1. Update the `linstor` alias and check resources:

   ```shell
   alias linstor='d8 k -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
   linstor resource list --faulty
   ```

If no faulty resources are found, migration was successful.

### Migrating to ReplicatedStorageClass

StorageClasses in this module are managed via the [ReplicatedStorageClass](/modules/sds-replicated-volume/stable/cr.html#replicatedstorageclass) resource. StorageClasses must not be created manually.

When migrating from the LINSTOR module, delete old StorageClasses and create new ones via ReplicatedStorageClass according to the table below.

Note that in old StorageClasses you look at the option in the `parameters` section of the StorageClass itself, while when creating a new one you set the corresponding option in ReplicatedStorageClass.

| StorageClass parameter                               | ReplicatedStorageClass | Default | Notes                                                                                       |
|------------------------------------------------------|------------------------|---------|---------------------------------------------------------------------------------------------|
| linstor.csi.linbit.com/placementCount: "1"           | replication: "None"    |         | One data replica will be created                                                            |
| linstor.csi.linbit.com/placementCount: "2"           | replication: "Availability" |     | Two data replicas will be created                                                           |
| linstor.csi.linbit.com/placementCount: "3"           | replication: "ConsistencyAndAvailability" | Yes | Three data replicas will be created                                                         |
| linstor.csi.linbit.com/storagePool: "name"           | storagePool: "name"    |         | Name of the storage pool used for storage                                                   |
| linstor.csi.linbit.com/allowRemoteVolumeAccess: "false" | volumeAccess: "Local" |         | Remote Pod access to data volumes is forbidden (local disk access within the node only)     |

Additional parameters:

- `reclaimPolicy` (Delete, Retain): Corresponds to `reclaimPolicy` of the old StorageClass.
- `zones`: List of zones to place resources in (direct cloud zone names). Note that remote Pod access to the data volume is possible only within one zone.
- `volumeAccess` values: `Local` (access strictly within the node), `EventuallyLocal` (a data replica will synchronize to the node after the Pod starts), `PreferablyLocal` (remote Pod access allowed, `volumeBindingMode: WaitForFirstConsumer`), `Any` (remote Pod access allowed, `volumeBindingMode: Immediate`).
- If you need `volumeBindingMode: Immediate`, set `volumeAccess` in ReplicatedStorageClass to `Any`.

### Migrating to ReplicatedStoragePool

The [ReplicatedStoragePool](/modules/sds-replicated-volume/cr.html#replicatedstoragepool) resource allows you to create Storage Pools in the backend. It is recommended to create this resource even for existing Storage Pools and reference existing [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup). The controller will detect that the Storage Pools already exist and leave them unchanged, showing `Created` in `status.phase`.

## Migration from the sds-drbd module to sds-replicated-volume

During migration the module control plane and its CSI are unavailable. This prevents creation, expansion, or deletion of PVs and the creation or deletion of Pods that use DRBD PVs for the duration of the migration.

{% alert level="warning" %}
The migration will not affect user data, as it is performed in a new namespace and volume management will be handled by new components that will replace the functionality of the previous module.
{% endalert %}

### Migration procedure

1. Make sure there are no faulty DRBD resources in the cluster. The command should return an empty list:

   ```shell
   alias linstor='d8 k -n d8-sds-drbd exec -ti deploy/linstor-controller -- linstor'
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

1. Create a ModuleConfig for [`sds-replicated-volume`](/modules/sds-replicated-volume/):

   > **Warning.** If `settings.dataNodes.nodeSelector` is not specified for `sds-replicated-volume`, its value will be taken from the `sds-drbd` module. If it is absent there as well, it will remain empty and all cluster nodes will be considered data nodes.

   ```shell
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

1. Wait until all Pods in the `d8-sds-replicated-volume` namespace are `Ready` or `Completed`:

   ```shell
   d8 k get po -n d8-sds-replicated-volume
   ```

1. Update the `linstor` alias and check DRBD resources:

   ```shell
   alias linstor='d8 k -n d8-sds-replicated-volume exec -ti deploy/linstor-controller -- linstor'
   linstor resource list --faulty
   ```

If no faulty resources are found, migration was successful.

> **Warning.** DRBDStoragePool and DRBDStorageClass resources will be automatically migrated to ReplicatedStoragePool and ReplicatedStorageClass. No user action is required.

The logic of these resources remains unchanged. However, verify that no DRBDStoragePool or DRBDStorageClass resources remain. If they do, contact the [Deckhouse technical support](/tech-support/).

## Reasons to avoid RAID with sds-replicated-volume

Using DRBD with more than one replica already provides network‑level RAID functionality. Local RAID can cause the following issues:

- Significantly increases space overhead when using redundant RAID.  
  Example: [ReplicatedStorageClass](/modules/sds-replicated-volume/cr.html#replicatedstorageclass) with `replication` set to `ConsistencyAndAvailability`. DRBD will store three data replicas (one per node). If those nodes use RAID1, storing 1 GB of data will require 6 GB of disk space. Redundant RAID is reasonable only to simplify server maintenance when storage cost is irrelevant. RAID1 then allows replacing disks without moving data replicas off a "problem" disk.

- With RAID0 the performance gain is negligible because data replication occurs over the network and the bottleneck is likely the network. Additionally, reduced host storage reliability can lead to data unavailability since DRBD failover from a broken replica to a healthy one is not instantaneous.

## Recommendations for using local disks

DRBD uses the network for data replication. When using NAS, network load increases dramatically because nodes synchronize data not only with the NAS but also with each other. Latency for reads or writes also increases. NAS typically uses RAID on its side, adding further overhead.

## Manual trigger the certificate renewal process

Although the certificate renewal process is automated, manual renewal might still be necessary because it can be performed during a convenient maintenance window when it is acceptable to restart the module's objects. The automated renewal does not restart any objects.

To manually trigger the certificate renewal process, create a `ConfigMap` named `manualcertrenewal-trigger`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: manualcertrenewal-trigger
  namespace: d8-sds-replicated-volume
```

The system will stop all necessary module objects, update the certificates, and then restart them.

You can check the operation status using the following command:

```shell
d8 k -n d8-sds-replicated-volume get cm manualcertrenewal-trigger -ojsonpath='{.data.step}'
```

Possible statuses:

- `Prepared`: Health checks have passed successfully, and the downtime window has started.
- `TurnedOffAndRenewedCerts`: The system has been stopped and certificates have been renewed.
- `TurnedOn`: The system has been restarted.
- `Done`: The operation is complete and ready to be repeated.

Certificates are issued for a period of one year and are marked as expiring 30 days before their expiration date. The monitoring system alerts about expiring certificates using the `D8LinstorCertificateExpiringIn30d` alert.

To repeat the operation, simply remove the label from the trigger using the following command:

```shell
d8 k -n d8-sds-replicated-volume label cm manualcertrenewal-trigger storage.deckhouse.io/sds-replicated-volume-manualcertrenewal-completed-
```
