---
title: "The csi-ceph module: FAQ"
---

## How to get a list of RBD volumes separated by nodes?

```shell
kubectl -n d8-csi-ceph get po -l app=csi-node-rbd -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName --no-headers \
  | awk '{print "echo "$2"; kubectl -n d8-csi-ceph exec  "$1" -c node -- rbd showmapped"}' | bash
```

## Which versions of Ceph clusters are supported

Officially, versions >= 16.2.0 are currently supported. Based on our experience, the current version can work with clusters of versions >= 14.2.0, but we recommend updating the Ceph version.

## Which volume access modes are supported

RBD supports only ReadWriteOnce (RWO, access to the volume within a single node). CephFS supports both ReadWriteOnce and ReadWriteMany (RWX, simultaneous access to the volume from multiple nodes).
