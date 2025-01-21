---
title: "Модуль csi-nfs: FAQ"
description: FAQ по модулю CSI NFS
---

## Как проверить работоспособность модуля?

Для этого необходимо проверить состояние подов в namespace `d8-csi-nfs`. Все поды должны быть в состоянии `Running` или `Completed` и запущены на всех узлах.

```shell
kubectl -n d8-csi-nfs get pod -owide -w
```

## Возможно ли изменение параметров NFS-сервера уже созданных PV?

Нет, данные для подключения к NFS-серверу сохраняются непосредственно в манифесте PV, и не подлежат изменению. Изменение Storage Class также не повлечет изменений настроек подключения в уже существующих PV.

## Как делать снимки томов (snapshots)?

В `csi-nfs` снимки создаются путем архивирования папки тома. Архив сохраняется в корне папки NFS сервера, указанной в параметре `spec.connection.share`.

### Шаг 1: Включение snapshot-controller

Для начала необходимо включить snapshot-controller:

```shell
kubectl apply -f -<<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: snapshot-controller
spec:
  enabled: true
  version: 1
EOF

```

### Шаг 2: Создание снимка тома

Теперь вы можете создавать снимки томов. Для этого выполните следующую команду, указав нужные параметры:

```shell
kubectl apply -f -<<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: my-snapshot
  namespace: <имя namespace, в котором находится PVC>
spec:
  volumeSnapshotClassName: csi-nfs-snapshot-class
  source:
    persistentVolumeClaimName: <имя PVC, для которого необходимо создать снимок>
EOF

```

### Шаг 3: Проверка состояния снимка 

Чтобы проверить состояние созданного снимка, выполните команду:

```shell
kubectl get volumesnapshot

```

Эта команда покажет список всех снимков и их текущее состояние.
