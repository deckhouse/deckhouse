---
title: "Хранилище YADRO"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/external/yadro.html
lang: ru
d8Revision: ee
---

Для управления томами на основе системы хранения данных [TATLIN.UNIFIED](https://yadro.com/ru/tatlin/unified) можно использовать модуль `csi-yadro`, позволяющий создавать ресурсы StorageClass через создание пользовательских ресурсов YadroStorageClass.

## Включение модуля

Чтобы включить модуль `csi-yadro`, выполните команду:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-yadro
spec:
  enabled: true
  version: 1
EOF
```

Дождитесь, когда модуль `csi-yadro` перейдет в состояние `Ready`.
Проверить состояние модуля можно, выполнив следующую команду:

```shell
d8 k get module csi-yadro -w
```

В результате будет выведена информация о модуле:

```console
NAME        STAGE   SOURCE   PHASE       ENABLED   READY
csi-yadro                    Available   True      True
```

## Подключение к системе хранения данных TATLIN.UNIFIED

Чтобы создать подключение к системе хранения данных TATLIN.UNIFIED и иметь возможность настраивать объекты StorageClass, примените следующий ресурс YadroStorageConnection:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroStorageConnection
metadata:
  name: yad1
spec:
  controlPlane:
    address: "172.19.28.184"
    username: "admin"
    password: "cGFzc3dvcmQ=" # Должен быть закодирован в Base64.
    ca: "base64encoded"
    skipCertificateValidation: true
  dataPlane:
    protocol: "iscsi"
    iscsi:
      volumeExportPort: "p50,p51,p60,p61"
EOF
```

## Создание StorageClass

Для создания StorageClass необходимо использовать ресурс YadroStorageClass.
Ручное создание ресурса StorageClass без YadroStorageClass может привести к ошибкам.

Пример команды для создания класса хранения на основе системы хранения данных TATLIN.UNIFIED:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroStorageClass
metadata:
  name: yad1
spec:
  fsType: "xfs"
  pool: "pool-hdd"
  storageConnectionName: "yad1"
  reclaimPolicy: Delete
EOF
```

## Проверка работоспособности модуля

Для того, чтобы проверить работоспособность модуля `csi-yadro`, необходимо проверить состояние подов в пространстве имен `d8-csi-yadro`.
Все поды должны быть в состоянии `Running` или `Completed`, поды `csi-yadro` должны быть запущены на всех узлах.
Проверить работоспособность модуля можно с помощью команды:

```shell
kubectl -n d8-csi-yadro get pod -owide -w
```
