---
title: "The csi-netapp module: FAQ"
description: CSI Netapp module FAQ
---

## How to check module health?

To do this, you need to check the status of the pods in the `d8-csi-netapp` namespace. All pods should be in the `Running` or `Completed` state and should be running on all nodes.

```shell
kubectl -n d8-csi-netapp get pod -owide -w
```
