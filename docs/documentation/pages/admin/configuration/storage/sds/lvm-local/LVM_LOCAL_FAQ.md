---
title: "Managing local storage based on LVM"
permalink: en/admin/configuration/storage/sds/lvm-local-faq.html
---

## Selecting specific nodes for module usage

To restrict the module's usage to specific cluster nodes, you need to set labels in the [`nodeSelector`](/modules/sds-local-volume/configuration.html#parameters-datanodes-nodeselector) field in the module settings.

To display and edit the module settings, execute the command:

```shell
d8 k edit mc sds-local-volume
```

Configuration example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-local-volume
spec:
  enabled: true
  settings:
    dataNodes:
      nodeSelector:
        my-custom-label-key: my-custom-label-value
status:
  message: ""
  version: "1"
```

To view the current labels in the `nodeSelector` field, use the following command:

```shell
d8 k get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Example output:

```console
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

The module selects only those nodes that have all the labels specified in the `nodeSelector`. By modifying this field, you can control the list of nodes on which the module will run.

{% alert level=“warning” %}
You can specify multiple labels in the `nodeSelector`. However, for the module to work correctly, all these labels must be present on each node where you intend to run `sds-local-volume-csi-node`.
{% endalert %}

After configuring the labels, ensure that the `sds-local-volume-csi-node` Pods are running on the target nodes. You can check their presence with the command:

```shell
d8 k -n d8-sds-local-volume get pod -owide
```

## Verifying PVC creation on the selected node

Make sure that the `sds-local-volume-csi-node` Pod is running on the selected node. To do this, run the command:

```shell
d8 k -n d8-sds-local-volume get po -owide
```

If the Pod is absent, verify that all the labels specified in the module settings in the nodeSelector field are present on the node. For available solutions when Pods are missing from the target node, refer to [this section](#absence-of-component-service-pods-on-the-desired-node).

## Removing a node from the module management

To remove a node from module management, you need to delete the labels set in the [`nodeSelector`](/modules/sds-local-volume/configuration.html#parameters-datanodes-nodeselector) field in the `sds-local-volume` module settings.

To check the current labels, execute the command:

```shell
d8 k get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Example output:

```console
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Remove the specified labels from the nodes with the command:

```shell
d8 k label node %node-name% %label-from-selector%-
```

{% alert level="warning" %}
After the label key, you must specify a minus sign to remove it.
{% endalert %}

After this, the `sds-local-volume-csi-node` Pod should be removed from the node. Check its status with the command:

```shell
d8 k -n d8-sds-local-volume get po -owide
```

If the Pod remains after removing the label, ensure that the labels from the `d8-sds-local-volume-controller-config` config are actually removed. You can verify this using:

```shell
d8 k get node %node-name% --show-labels
```

If the labels are absent, check that the node does not have any [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resources being used by [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass) resources. More information on this check can be found in [this section](#verifying-dependent-lvmvolumegroup-resources-on-the-node).

{% alert level="warning" %}
Note that for [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) and [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass) resources that are the reason why the node cannot be removed from module management, the label `storage.deckhouse.io/sds-local-volume-candidate-for-eviction` will be assigned.

On the node itself, the label `storage.deckhouse.io/sds-local-volume-need-manual-eviction` will be present.
{% endalert %}

## Verifying dependent LVMVolumeGroup resources on the node

To check the dependent resources, follow these steps:

1. Display the available [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass) resources:

   ```shell
   d8 k get lsc
   ```

1. Check each of them for the list of used [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resources.

   If you want to list all [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass) resources at once, run the command:

   ```shell
   d8 k get lsc -oyaml
   ```

   Example [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass) resource:

   ```yaml
   apiVersion: v1
   items:
   - apiVersion: storage.deckhouse.io/v1alpha1
     kind: LocalStorageClass
     metadata:
       finalizers:
       - localstorageclass.storage.deckhouse.io
       name: test-sc
     spec:
       lvm:
         lvmVolumeGroups:
         - name: test-vg
         type: Thick
       reclaimPolicy: Delete
       volumeBindingMode: WaitForFirstConsumer
     status:
       phase: Created
   kind: List
   ```

   Pay attention to the `spec.lvm.lvmVolumeGroups` field. It specifies the used [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resources.

1. Display the list of existing [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resources.

   ```shell
   d8 k get lvg
   ```

   Example [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) output:

   ```text
   NAME              HEALTH        NODE            SIZE       ALLOCATED SIZE   VG        AGE
   lvg-on-worker-0   Operational   node-worker-0   40956Mi    0                test-vg   15d
   lvg-on-worker-1   Operational   node-worker-1   61436Mi    0                test-vg   15d
   lvg-on-worker-2   Operational   node-worker-2   122876Mi   0                test-vg   15d
   lvg-on-worker-3   Operational   node-worker-3   307196Mi   0                test-vg   15d
   lvg-on-worker-4   Operational   node-worker-4   307196Mi   0                test-vg   15d
   lvg-on-worker-5   Operational   node-worker-5   204796Mi   0                test-vg   15d
   ```

1. Ensure that the node you intend to remove from the module's control does not have any [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resources used in [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass) resources. To avoid unintentionally losing control over volumes already created using the module, the user needs to manually delete dependent resources by performing necessary operations on the volume.

## Remaining sds-local-volume-csi-node Pod after removing labels

If after removing the labels from the node the `sds-local-volume-csi-node` Pod continues to run, this is most likely due to the presence on the node of [LVMVolumeGroup](/modules/sds-node-configurator/cr.html#lvmvolumegroup) resources that are used by one of the [LocalStorageClass](/modules/sds-local-volume/cr.html#localstorageclass) resources. The verification process is described [above](#verifying-dependent-LVMVolumeGroup-resources-on-the-node).

## Absence of component service Pods on the desired node

The issue may be related to incorrectly set labels. The nodes used by the module are determined by the labels specified in the module settings in the [`nodeSelector`](/modules/sds-local-volume/configuration.html#parameters-datanodes-nodeselector) field. To view the current labels, run:

```shell
d8 k get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Example output:

```console
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Additionally, you can check the selectors used by the module in the secret configuration `d8-sds-local-volume-controller-config` in the `d8-sds-local-volume` namespace:

```shell
d8 k -n d8-sds-local-volume get secret d8-sds-local-volume-controller-config  -o jsonpath='{.data.config}' | base64 --decode
```

Example output:

```console
nodeSelector:
  kubernetes.io/os: linux
  my-custom-label-key: my-custom-label-value
```

This command output should list all the labels from the module settings `data.nodeSelector`, as well as `kubernetes.io/os: linux`.

Check the labels on the desired node:

```shell
d8 k get node %node-name% --show-labels
```

If necessary, add the missing labels to the desired node:

```shell
d8 k label node %node-name% my-custom-label-key=my-custom-label-value
```

If the labels are present, check for the presence of the label `storage.deckhouse.io/sds-local-volume-node=` on the node. If this label is missing, ensure that the `sds-local-volume-controller` is running, and review its logs:

```shell
d8 k -n d8-sds-local-volume get po -l app=sds-local-volume-controller
d8 k -n d8-sds-local-volume logs -l app=sds-local-volume-controller
```

## Data migration between PVCs

Copy the following script into a file named `migrate.sh` on any master node:

```shell
#!/bin/bash

ns=$1
src=$2
dst=$3

if [[ -z $3 ]]; then
  echo "You must give as args: namespace source_pvc_name destination_pvc_name"
  exit 1
fi

echo "Creating job yaml"
cat > migrate-job.yaml << EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: migrate-pv-$src
  namespace: $ns
spec:
  template:
    spec:
      containers:
      - name: migrate
        image: debian
        command: [ "/bin/bash", "-c" ]
        args:
          -
            apt-get update && apt-get install -y rsync &&
            ls -lah /src_vol /dst_vol &&
            df -h &&
            rsync -avPS --delete /src_vol/ /dst_vol/ &&
            ls -lah /dst_vol/ &&
            du -shxc /src_vol/ /dst_vol/
        volumeMounts:
        - mountPath: /src_vol
          name: src
          readOnly: true
        - mountPath: /dst_vol
          name: dst
      restartPolicy: Never
      volumes:
      - name: src
        persistentVolumeClaim:
          claimName: $src
      - name: dst
        persistentVolumeClaim:
          claimName: $dst
  backoffLimit: 1
EOF

d8 k create -f migrate-job.yaml
d8 k -n $ns get jobs -o wide
kubectl_completed_check=0

echo "Waiting for data migration to be completed"
while [[ $kubectl_completed_check -eq 0 ]]; do
   d8 k -n $ns get pods | grep migrate-pv-$src
   sleep 5
   kubectl_completed_check=`d8 k -n $ns get pods | grep migrate-pv-$src | grep "Completed" | wc -l`
done
echo "Data migration completed"
```

To use the script, run the following command:

```shell
migrate.sh NAMESPACE SOURCE_PVC_NAME DESTINATION_PVC_NAME
```

## Creating volume snapshots

You can read more about snapshots and associated resources in the [Kubernetes documentation](https://kubernetes.io/docs/concepts/storage/volume-snapshots/).

1. Enable the [`snapshot-controller`](/modules/snapshot-controller/) module:

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: snapshot-controller
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Now you can create volume snapshots. To do this, execute the following command with the necessary parameters:

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshot
   metadata:
     name: my-snapshot
     namespace: <name of the namespace where the PVC is located>
   spec:
     volumeSnapshotClassName: sds-local-volume-snapshot-class
     source:
       persistentVolumeClaimName: <name of the PVC to snapshot>
   EOF
   ```

   Note that `sds-local-volume-snapshot-class` is created automatically, and its `deletionPolicy` is set to `Delete`, which means that the VolumeSnapshotContent resource will be deleted when the associated VolumeSnapshot resource is deleted.

1. To check the status of the created snapshot, execute the command:

   ```shell
   d8 k get volumesnapshot
   ```

   This command will display a list of all snapshots and their current status.
