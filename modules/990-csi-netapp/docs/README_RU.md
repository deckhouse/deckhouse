---
title: "Модуль csi-netapp"
description: "Модуль csi-netapp: общие концепции и положения."
d8Edition: ee
---

Модуль предоставляет CSI для управления томами c использованием СХД Netapp. Модуль позволяет создавать `StorageClass` в `Kubernetes` через создание [пользовательских ресурсов Kubernetes](./cr.html#Netappstorageclass) `NetappStorageClass`.

> **Внимание!** Создание `StorageClass` для CSI-драйвера `csi.Netapp.com` пользователем запрещено.

> **Внимание!** На данный момент модулем поддерживаются СХД, совместимые с [Trident CSI от NetApp](https://github.com/NetApp/trident). Для поддержки других СХД Netapp, пожалуйста, свяжитесь с техподдержкой.

{% alert level="info" %}
Для работы с снапшотами требуется подключенный модуль [snapshot-controller](../../snapshot-controller/).
{% endalert %}

## Системные требования и рекомендации

### Требования

- Наличие развернутой и настроенной СХД Netapp.
- Уникальные iqn в /etc/iscsi/initiatorname.iscsi на каждой из Kubernetes Nodes

## Быстрый старт

Все команды следует выполнять на машине, имеющей доступ к API Kubernetes с правами администратора.

### Включение модуля

- Включить модуль `csi-netapp`.  Это приведет к тому, что на всех узлах кластера будет:
    - зарегистрирован CSI драйвер;
    - запущены служебные поды компонентов `csi-netapp`.

```shell
kubectl apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-netapp
spec:
  enabled: true
  version: 1
EOF
```

- Дождаться, когда модуль перейдет в состояние `Ready`.

```shell
kubectl get module csi-netapp -w
```

### Создание StorageClass

Для создания StorageClass необходимо использовать ресурсы [NetappStorageClass](./cr.html#Netappstorageclass) и [NetappStorageConnection](./cr.html#Netappstorageconnection). Пример команд для создания таких ресурсов:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: NetappStorageConnection
metadata:
  name: Netapp
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
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: NetappStorageClass
metadata:
  name: Netapp
spec:
  pool: "test-cpg"
  accessProtocol: "fc" # fc или iscsi (default iscsi), immutable
  fsType: "xfs" # xfs, ext3, ext4 (default ext4), mutable
  storageConnectionName: "3par" # immutable
  reclaimPolicy: Delete # Delete of Retain
  cpg: "test-cpg"
  EOF
```

- Проверить создание объекта можно командой (Phase должен быть `Created`):

```shell
kubectl get Netappstorageconnections.storage.deckhouse.io <имя Netappstorageconnection>
```

```shell
kubectl get Netappstorageclasses.storage.deckhouse.io <имя Netappstorageclass>
```

### Проверка работоспособности модуля.

Проверить работоспособность модуля можно [так](./faq.html#как-проверить-работоспособность-модуля)
