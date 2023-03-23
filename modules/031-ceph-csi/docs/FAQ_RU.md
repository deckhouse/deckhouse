---
title: "Модуль ceph-csi: FAQ"
---

## Как получить список томов RBD, разделённый по узлам?

```shell
kubectl -n d8-ceph-csi get po -l app=csi-node-rbd --no-headers -owide | awk '{print "echo "$7"; kubectl -n d8-ceph-csi exec  "$1" -c node -- rbd showmapped"}' | bash
```
