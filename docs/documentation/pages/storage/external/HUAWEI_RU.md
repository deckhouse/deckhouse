---
title: "Huawei-хранилище"
permalink: ru/storage/admin/external/huawei.html
lang: ru
---

Данный модуль хранилища предоставляет CSI для управления томами c использованием СХД Huawei. Модуль позволяет создавать `StorageClass` в `Kubernetes` через создание [пользовательских ресурсов Kubernetes](./cr.html#huaweistorageclass) `HuaweiStorageClass`.

> **Внимание!** Создание `StorageClass` для CSI-драйвера `csi.huawei.com` пользователем запрещено.
> **Внимание!** На данный момент модулем поддерживаются СХД Huawei 3par. Для поддержки других СХД Huawei, пожалуйста, свяжитесь с техподдержкой.

## Системные требования и рекомендации

### Требования

- Наличие развернутой и настроенной СХД Huawei.
- Уникальные iqn в /etc/iscsi/initiatorname.iscsi на каждой из Kubernetes Nodes

## Быстрый старт

Все команды следует выполнять на машине, имеющей доступ к API Kubernetes с правами администратора.

### Включение модуля

- Включить модуль `csi-huawei`.  Это приведет к тому, что на всех узлах кластера будет:
  - зарегистрирован CSI драйвер;
  - запущены служебные поды компонентов `csi-huawei`.

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-huawei
spec:
  enabled: true
  version: 1
EOF
```

- Дождаться, когда модуль перейдет в состояние `Ready`.

```shell
kubectl get module csi-huawei -w
```

### Создание StorageClass

Для создания StorageClass необходимо использовать ресурсы `HuaweiStorageClass` и `HuaweiStorageConnection`. Пример команд для создания таких ресурсов:

```yaml
d8 k apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: HuaweiStorageConnection
metadata:
  name: huaweistorageconn
spec:
  storageType: OceanStorSAN
  pools:
    - test
  urls: 
    - https://192.168.128.101:8088 
  login: "admin"
  password: "ivkerg43grdsf_"
  protocol: ISCSI
  portals:
    - 10.240.0.101
    - 10.250.0.101 
  maxClientThreads: 30

EOF
```

```yaml
d8 k apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: HuaweiStorageClass
metadata:
  name: huaweisc
spec:
  fsType: ext4
  pool: test
  reclaimPolicy: Delete
  storageConnectionName: huaweistorageconn
  volumeBindingMode: WaitForFirstConsumer
EOF
```

- Проверить создание объекта можно командой (Phase должен быть `Created`):

```shell
d8 k get huaweistorageconnections.storage.deckhouse.io <имя huaweistorageconnection>
```

```shell
d8 k get huaweistorageclasses.storage.deckhouse.io <имя huaweistorageclass>
```

### Как проверить работоспособность модуля?

Для этого необходимо проверить состояние подов в namespace `d8-csi-huawei`. Все поды должны быть в состоянии `Running` или `Completed` и запущены на всех узлах.

```shell
d8 k -n d8-csi-huawei get pod -owide -w
```
