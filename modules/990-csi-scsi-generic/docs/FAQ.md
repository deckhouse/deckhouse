---
title: "The csi-scsi-generic module: FAQ"
description: CSI SCSI GENERIC module FAQ
---

## How to check module health?

To do this, you need to check the status of the pods in the `d8-csi-scsi-generic` namespace. All pods should be in the `Running` or `Completed` state and should be running on all nodes.

```shell
kubectl -n d8-csi-scsi-generic get pod -owide -w
```
