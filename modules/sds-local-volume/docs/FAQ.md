---
title: "The sds-local-volume module: FAQ"
description: "The sds-local-volume module: FAQ"
---

## What is difference between LVM and LVMThin?

- LVM is simpler and has high performance that is similar to that of native disk drives;
- LVMThin allows overprovisioning; however, it is slower than LVM.

{{< alert level="warning" >}}
Overprovisioning in LVMThin should be used with caution, monitoring the availability of free space in the pool (The cluster monitoring system generates separate events when the free space in the pool reaches 20%, 10%, 5%, and 1%).

In case of no free space in the pool, degradation in the module's operation as a whole will be observed, and there is a real possibility of data loss!
{{< /alert >}}

## How do I set the default StorageClass?

Add the annotation `storageclass.kubernetes.io/is-default-class: "true"` to the corresponding StorageClass resource:

```shell
kubectl annotate storageclasses.storage.k8s.io <storageClassName> storageclass.kubernetes.io/is-default-class=true
```

## I don't want the module to be used on all nodes of the cluster. How can I select the desired nodes?

The nodes that will be involved with the module are determined by special labels specified in the `nodeSelector` field in the module settings.

To display and edit the module settings, you can execute the command:

```shell
kubectl edit mc sds-local-volume
```

The approximate output of the command would be:

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

To display existing labels specified in the `nodeSelector` field, you can execute the command:

```shell
kubectl get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

The approximate output of the command would be:

```yaml
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Nodes whose labels include the set specified in the settings are selected by the module as targets for usage. Therefore, by changing the `nodeSelector` field, you can influence the list of nodes that the module will use.

{{< alert level="warning" >}}
The `nodeSelector` field can contain any number of labels, but it's crucial that each of the specified labels is present on the node you intend to use for working with the module. It's only when all the specified labels are present on the selected node that the `sds-local-volume-csi-node` pod will be launched.
{{< /alert >}}

After adding labels to the nodes, the `sds-local-volume-csi-node` pods should be started. You can check their presence using the command:

```shell
 kubectl -n d8-sds-local-volume get pod -owide
 ```

## Why can't I create a PVC on the selected node using the module?

Please verify that the pod `sds-local-volume-csi-node` is running on the selected node.

```shell
kubectl -n d8-sds-local-volume get po -owide
```

If the pod is missing, please ensure that all labels specified in the module settings in the `nodeSelector` field are present on the selected node. More details about this can be found [here](#service-pods-for-the-sds-local-volume-components-are-not-being-created-on-the-node-i-need-why-is-that).

## How do I take a node out of the module's control?

To take a node out of the module's control, you need to remove the labels specified in the `nodeSelector` field in the module settings for `sds-local-volume`.

You can check the presence of existing labels in the `nodeSelector` using the command:

```shell
kubectl get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

The approximate output of the command would be:

```yaml
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Remove the labels specified in `nodeSelector` from the desired nodes.

```shell
kubectl label node %node-name% %label-from-selector%-
```

{{< alert level="warning" >}}
To remove a label, you need to add a hyphen immediately after its key instead of its value.
{{< /alert >}}

As a result, the `sds-local-volume-csi-node` pod should be deleted from the desired node. You can check its status using the command:

```shell
kubectl -n d8-sds-local-volume get po -owide
```

If the `sds-local-volume-csi-node` pod remains on the node after removing the `nodeSelector` label, please ensure that the labels specified in the `nodeSelector` field of the `d8-sds-local-volume-controller-config` in the config have indeed been successfully removed from the selected node.

You can verify this using the command:

```shell
kubectl get node %node-name% --show-labels
```

If the labels from `nodeSelector` are not present on the node, ensure that this node does not own any `LVMVolumeGroup` resources used by `LocalStorageClass` resources. More details about this check can be found [here](#how-to-check-if-there-are-dependent-resources-lvmvolumegroup-on-the-node).

{{< alert level="warning" >}}
On the `LVMVolumeGroup` and `LocalStorageClass` resources, which prevent the node from being taken out of the module's control, the label `storage.deckhouse.io/sds-local-volume-candidate-for-eviction` will be displayed.
On the node itself, the label `storage.deckhouse.io/sds-local-volume-need-manual-eviction` will be present.
{{< /alert  >}}

## How to check if there are dependent resources `LVMVolumeGroup` on the node?

To check for such resources, follow these steps:
1. Display the existing `LocalStorageClass` resources

   ```shell
   kubectl get lsc
   ```

2. Check each of them for the list of used `LVMVolumeGroup` resources.

   > If you want to list all `LocalStorageClass` resources at once, run the command:
   >
   > ```shell
   > kubectl get lsc -oyaml
   > ```

   ```shell
   kubectl get lsc %lsc-name% -oyaml
   ```

   An approximate representation of `LocalStorageClass` could be:

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

   > Please pay attention to the `spec.lvm.lvmVolumeGroups` field - it specifies the used `LVMVolumeGroup` resources.

3. Display the list of existing `LVMVolumeGroup` resources.

   ```shell
   kubectl get lvg
   ```

   An approximate representation of `LVMVolumeGroup` could be:

   ```text
   NAME              HEALTH        NODE                         SIZE       ALLOCATED SIZE   VG        AGE
   lvg-on-worker-0   Operational   node-worker-0   40956Mi    0                test-vg   15d
   lvg-on-worker-1   Operational   node-worker-1   61436Mi    0                test-vg   15d
   lvg-on-worker-2   Operational   node-worker-2   122876Mi   0                test-vg   15d
   lvg-on-worker-3   Operational   node-worker-3   307196Mi   0                test-vg   15d
   lvg-on-worker-4   Operational   node-worker-4   307196Mi   0                test-vg   15d
   lvg-on-worker-5   Operational   node-worker-5   204796Mi   0                test-vg   15d
   ```

4. Ensure that the node you intend to remove from the module's control does not have any `LVMVolumeGroup` resources used in `LocalStorageClass` resources.

   To avoid unintentionally losing control over volumes already created using the module, the user needs to manually delete dependent resources by performing necessary operations on the volume.

## I removed the labels from the node, but the `sds-local-volume-csi-node` pod is still there. Why did this happen?

Most likely, there are `LVMVolumeGroup` resources present on the node, which are used in one of the `LocalStorageClass` resources.

To avoid unintentionally losing control over volumes already created using the module, the user needs to manually delete dependent resources by performing necessary operations on the volume."

The process of checking for the presence of the aforementioned resources is described [here](#how-to-check-if-there-are-dependent-resources-lvmvolumegroup-on-the-node).

## Service pods for the `sds-local-volume` components are not being created on the node I need. Why is that?

With a high probability, the issues are related to the labels on the node.

Nodes to be used by the module are determined by special labels specified in the `nodeSelector` field in the module settings.

To display the existing labels specified in the `nodeSelector` field, you can execute the command:

```shell
kubectl get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

The approximate output of the command would be:

```yaml
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Nodes whose labels include the set specified in the settings are chosen by the module as targets for usage.

You can also additionally check the selectors used by the module in the configuration of the secret `d8-sds-local-volume-controller-config` in the namespace `d8-sds-local-volume`.

```shell
kubectl -n d8-sds-local-volume get secret d8-sds-local-volume-controller-config  -o jsonpath='{.data.config}' | base64 --decode
```

The approximate output of the command would be:

```yaml
nodeSelector:
  kubernetes.io/os: linux
  my-custom-label-key: my-custom-label-value
```

The output of this command should include all labels from the settings of the `data.nodeSelector` module, as well as `kubernetes.io/os: linux`.

Check the labels on the node you need:

```shell
kubectl get node %node-name% --show-labels
```

If necessary, add the missing labels to the desired node:

```shell
kubectl label node %node-name% my-custom-label-key=my-custom-label-value
```

If the labels are present, it's necessary to check for the existence of the label `storage.deckhouse.io/sds-local-volume-node=` on the node. If the label is absent, it's advisable to verify whether `sds-local-volume-controller` is functioning properly. If it is, then check the logs:

```shell
kubectl -n d8-sds-local-volume get po -l app=sds-local-volume-controller
kubectl -n d8-sds-local-volume logs -l app=sds-local-volume-controller
```

## How to move data from one PVC to another?

Use the following script. Place it to file migrate.sh on any master node.
Usage: migrate.sh NAMESPACE SOURCE_PVC_NAME DESTINATION_PVC_NAME

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

kubectl create -f migrate-job.yaml
kubectl -n $ns get jobs -o wide
kubectl_completed_check=0

echo "Waiting for data migration to be completed"
while [[ $kubectl_completed_check -eq 0 ]]; do
   kubectl -n $ns get pods | grep migrate-pv-$src
   sleep 5
   kubectl_completed_check=`kubectl -n $ns get pods | grep migrate-pv-$src | grep "Completed" | wc -l`
done
echo "Data migration completed"
```

## How to Create Volume Snapshots?

You can read more about snapshots [here](https://kubernetes.io/docs/concepts/storage/volume-snapshots/).

### Step 1: Enabling the snapshot-controller

First, you need to enable the snapshot-controller:

```shell
kubectl apply -f -<<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: snapshot-controller
spec:
  enabled: true
  version: 1
EOF

```

### Step 2: Creating a Volume Snapshot

Now you can create volume snapshots. To do this, execute the following command with the necessary parameters:

```shell
kubectl apply -f -<<EOF
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

Note that `sds-local-volume-snapshot-class` is created automatically, and it's `deletionPolicy` is `Delete`, which means that `VolumeSnapshotContent` should be deleted when its bound `VolumeSnapshot` is deleted.


### Step 3: Checking the Snapshot Status

To check the status of the created snapshot, execute the command:

```shell
kubectl get volumesnapshot
```

This command will display a list of all snapshots and their current status.
