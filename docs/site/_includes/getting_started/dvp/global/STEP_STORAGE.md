{% alert level="warning" %}
At this stage, an example of configuring software-defined storage based on DRBD is provided.
If you want to use a different type of storage, refer to the section ["Configuring Storage"](../../documentation/admin/install/steps/storage.html).
{% endalert %}

At this step, the cluster is deployed in a minimal configuration. Configure the storage that will be used to create storage for metrics of the cluster components and virtual machine disks.

Enable `sds-replicated-volume` module — a module for the software-defined storage. Run the following commands on the **master node**:

```shell
kubectl create -f - <<EOF
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: snapshot-controller
spec:
  enabled: true
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-node-configurator
spec:
  version: 1
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-replicated-volume
spec:
  version: 1
  enabled: true
EOF
```

Wait for the module to start; you can use the following command for this:

```shell
kubectl wait module sds-replicated-volume --for='jsonpath={.status.phase}=Ready' --timeout=1200s
```

Combine the available block devices on the nodes into LVM volume groups. To obtain the available block devices, run the command:

```shell
sudo -i d8 k get blockdevices.storage.deckhouse.io
```

To combine block devices on one node, it is necessary to create an LVM volume group using the [LVMVolumeGroup](/modules/sds-node-configurator/stable/cr.html#lvmvolumegroup) resource.
To create the LVMVolumeGroup resource on the node, run the following command, replacing the names of the node and block devices with your own:

```shell
sudo -i d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: LVMVolumeGroup
metadata:
  name: "vg-on-dvp-worker"
spec:
  type: Local
  local:
    # Replace with the name of your node for which you are creating the volume group.
    nodeName: "dvp-worker"
  blockDeviceSelector:
    matchExpressions:
      - key: kubernetes.io/metadata.name
        operator: In
        values:
          # Replace with the names of your block devices of the node for which you are creating the volume group.
          - dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa
  # The name of the volume group in LVM that will be created from the specified block devices on the chosen node.
  actualVGNameOnTheNode: "vg"
  # Comment if it is important to have the ability to create Thin pools; details will be revealed later.
  # thinPools:
  #   - name: thin-pool-0
  #     size: 70%
EOF
```

Wait for the created LVMVolumeGroup resource to enter the `Operational` state:

```shell
sudo -i d8 k get lvg vg-on-dvp-worker -w
```

Example of the output:

```console
NAME               THINPOOLS   CONFIGURATION APPLIED   PHASE   NODE       SIZE       ALLOCATED SIZE   VG   AGE
vg-on-dvp-worker   1/1         True                    Ready   worker-0   360484Mi   30064Mi          vg   1h
```

Create an LVM volume pool:

```bash
sudo -i d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStoragePool
metadata:
  name: sds-pool
spec:
  type: LVM
  lvmVolumeGroups:
    - name: vg-on-dvp-worker
EOF
```

Wait for the created resource ReplicatedStoragePool to enter the `Completed` state:

```shell
sudo -i d8 k get rsp data -w
```

Example of the output:

```console
NAME         PHASE       TYPE   AGE
sds-pool     Completed   LVM    87d
```

Create a StorageClass:

```bash
sudo -i d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: ReplicatedStorageClass
metadata:
  name: sds-r1
spec:
  replication: None
  storagePool: sds-pool
  reclaimPolicy: Delete
  topology: Ignored
EOF
```

Check that the StorageClasses have been created:

```bash
sudo -i d8 k get storageclass
```

Set the StorageClass as the default StorageClass (specify the name of the StorageClass):

```shell
DEFAULT_STORAGE_CLASS=replicated-storage-class
sudo -i d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
```
