---
title: "The ceph-csi module: FAQ"
---

## How to get a list of RBD volumes separated by nodes?

```shell
kubectl -n d8-ceph-csi get po -l app=csi-node-rbd -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName --no-headers \
  | awk '{print "echo "$2"; kubectl -n d8-ceph-csi exec  "$1" -c node -- rbd showmapped"}' | bash
```
