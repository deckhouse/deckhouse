---
title: "HPE-хранилище"
permalink: ru/storage/admin/external/hpe.html
lang: ru
---

Данный модуль хранилища предоставляет CSI для управления томами c использованием СХД HPE. Модуль позволяет создавать `StorageClass` в `Kubernetes` через создание `HPEStorageClass`.

> **Внимание!** Создание `StorageClass` для CSI-драйвера `csi.hpe.com` пользователем запрещено.
> **Внимание!** На данный момент модулем поддерживаются СХД HPE 3par. Для поддержки других СХД HPE, пожалуйста, свяжитесь с техподдержкой.

## Системные требования и рекомендации

### Требования

- Наличие развернутой и настроенной СХД HPE.
- Уникальные iqn в /etc/iscsi/initiatorname.iscsi на каждой из Kubernetes Nodes

## Быстрый старт

Все команды следует выполнять на машине, имеющей доступ к API Kubernetes с правами администратора.

### Включение модуля

- Включить модуль `csi-hpe`.  Это приведет к тому, что на всех узлах кластера будет:
  - зарегистрирован CSI драйвер;
  - запущены служебные поды компонентов `csi-hpe`.

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-hpe
spec:
  enabled: true
  version: 1
EOF
```

- Дождаться, когда модуль перейдет в состояние `Ready`.

```shell
d8 k get module csi-hpe -w
```

### Создание StorageClass

Для создания StorageClass необходимо использовать ресурсы `HPEStorageClass` и `HPEStorageConnection`. Пример команд для создания таких ресурсов:

```yaml
d8 k apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: HPEStorageConnection
metadata:
  name: hpe
spec:
  controlPlane:
    backendAddress: "172.17.1.55" # mutable, адрес СХД
    username: "3paradm" # mutable, API username
    password: "3pardata" # mutable, API password
    serviceName: "primera3par-csp-svc"
    servicePort: "8080"
EOF
```

```yaml
d8 k apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: HPEStorageClass
metadata:
  name: hpe
spec:
  pool: "test-cpg"
  accessProtocol: "fc" # fc или iscsi (default iscsi), immutable
  fsType: "xfs" # xfs, ext3, ext4, btrfs (default ext4), mutable
  storageConnectionName: "3par" # immutable
  reclaimPolicy: Delete # Delete of Retain
  cpg: "test-cpg"
  EOF
```

- Проверить создание объекта можно командой (Phase должен быть `Created`):

```shell
d8 k get hpestorageconnections.storage.deckhouse.io <имя hpestorageconnection>
```

```shell
d8 k get hpestorageclasses.storage.deckhouse.io <имя hpestorageclass>
```

### Как проверить работоспособность модуля?

Для этого необходимо проверить состояние подов в namespace `d8-csi-hpe`. Все поды должны быть в состоянии `Running` или `Completed` и запущены на всех узлах.

```shell
d8 k -n d8-csi-hpe get pod -owide -w
```
