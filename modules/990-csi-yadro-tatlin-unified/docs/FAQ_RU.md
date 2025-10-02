---
title: "Модуль csi-yadro-tatlin-unified: FAQ"
description: FAQ по модулю CSI YADRO TU
---

## Как проверить работоспособность модуля?

Для этого необходимо проверить состояние подов в namespace `d8-csi-yadro-tatlin-unified`. Все поды должны быть в состоянии `Running` или `Completed` и запущены на всех узлах.

```shell
kubectl -n d8-csi-yadro-tatlin-unified get pod -owide -w
```
