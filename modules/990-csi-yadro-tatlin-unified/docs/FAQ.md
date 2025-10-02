---
title: "The csi-yadro-tatlin-unified module: FAQ"
description: CSI YADRO TU module FAQ
---

## How to check module health?

To do this, you need to check the status of the pods in the `d8-csi-yadro-tatlin-unified` namespace. All pods should be in the `Running` or `Completed` state and should be running on all nodes.

```shell
kubectl -n d8-csi-yadro-tatlin-unified get pod -owide -w
```
