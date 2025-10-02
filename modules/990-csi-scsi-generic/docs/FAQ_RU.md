---
title: "Модуль csi-scsi-generic: FAQ"
description: FAQ по модулю CSI SCSI GENERIC
---

## Как проверить работоспособность модуля?

Для этого необходимо проверить состояние подов в namespace `d8-csi-scsi-generic`. Все поды должны быть в состоянии `Running` или `Completed` и запущены на всех узлах.

```shell
kubectl -n d8-csi-scsi-generic get pod -owide -w
```

