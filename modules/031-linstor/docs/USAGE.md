---
title: "The linstor module: configuration examples"
---

## Using the linstor scheduler

The linstor scheduler considers the placement of data in storage and tries to place Pods on nodes where data is available locally first.  

Specify the `schedulerName: linstor` parameter in the Pod description to use the `linstor` scheduler.

An example of such a Pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: busybox
  namespace: default
spec:
  schedulerName: linstor # Using the linstor scheduler
  containers:
  - name: busybox
    image: busybox
    command: ["tail", "-f", "/dev/null"]
    volumeMounts:
    - name: my-first-linstor-volume
      mountPath: /data
    ports:
    - containerPort: 80
  volumes:
  - name: my-first-linstor-volume
    persistentVolumeClaim:
      claimName: "test-volume"
```

## Application transfer to another node in case of storage problems (fencing)

If the label `linstor.csi.linbit.com/on-storage-lost: remove` is present on a Pod, the linstor module automatically removes the Pods from the node where the storage problem occurred. This leads to restarting them on another node.  


Example StatefulSet with the `linstor.csi.linbit.com/on-storage-lost: remove` label:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: my-stateful-app
spec:
  serviceName: my-stateful-app
  selector:
    matchLabels:
      app.kubernetes.io/name: my-stateful-app
  template:
    metadata:
      labels:
        app.kubernetes.io/name: my-stateful-app
        linstor.csi.linbit.com/on-storage-lost: remove # <--
    ...
```
