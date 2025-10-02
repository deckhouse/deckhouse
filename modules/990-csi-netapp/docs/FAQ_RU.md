---
title: "Модуль csi-netapp: FAQ"
description: FAQ по модулю CSI Netapp
---

## Как проверить работоспособность модуля?

Для этого необходимо проверить состояние подов в namespace `d8-csi-netapp`. Все поды должны быть в состоянии `Running` или `Completed` и запущены на всех узлах.

```shell
kubectl -n d8-csi-netapp get pod -owide -w
```

