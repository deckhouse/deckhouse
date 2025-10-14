---
title: "Модуль csi-yadro-tatlin-unified"
description: "Модуль csi-yadro-tatlin-unified: общие концепции и положения."
d8Edition: ee
---

Модуль предоставляет CSI для управления томами c использованием СХД TATLIN.UNIFIED. Модуль позволяет создавать `StorageClass` в `Kubernetes` через создание [пользовательских ресурсов Kubernetes](./cr.html#yadrotatlinunifiedstorageclass) `YadroTatlinUnifiedStorageClass`.

> **Внимание!** Создание `StorageClass` для CSI-драйвера `csi-tatlinunified.yadro.com` пользователем запрещено.

{% alert level="info" %}
Для работы с снапшотами требуется подключенный модуль [snapshot-controller](/modules/snapshot-controller/).
{% endalert %}

## Системные требования и рекомендации

### Требования

- Наличие развернутой и настроенной СХД TATLIN.
- Уникальные iqn в /etc/iscsi/initiatorname.iscsi на каждой из Kubernetes Nodes

## Быстрый старт

Все команды следует выполнять на машине, имеющей доступ к API Kubernetes с правами администратора.

### Включение модуля

- Включить модуль `csi-yadro-tatlin-unified`.  Это приведет к тому, что на всех узлах кластера будет:
    - зарегистрирован CSI драйвер;
    - запущены служебные поды компонентов `csi-yadro-tatlin-unified`.

```shell
kubectl apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-yadro-tatlin-unified
spec:
  enabled: true
  version: 1
EOF
```

- Дождаться, когда модуль перейдет в состояние `Ready`.

```shell
kubectl get module csi-yadro-tatlin-unified -w
```

### Создание StorageClass

Для создания StorageClass необходимо использовать ресурсы [YadroTatlinUnifiedStorageClass](./cr.html#yadrotatlinunifiedstorageclass) и [YadroTatlinUnifiedStorageConnection](./cr.html#yadrotatlinunifiedstorageconnection). Пример команд для создания таких ресурсов:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroTatlinUnifiedStorageConnection
metadata:
  name: yad1
spec:
  controlPlane:
    address: "172.19.28.184"
    username: "admin"
    password: "cGFzc3dvcmQ=" # ДОЛЖЕН БЫТЬ ЗАКОДИРОВАН В BASE64
    ca: "base64encoded"
    skipCertificateValidation: true
  dataPlane:
    protocol: "iscsi"
    iscsi:
      volumeExportPort: "p50,p51,p60,p61"
EOF
```

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroTatlinUnifiedStorageClass
metadata:
  name: yad1
spec:
  fsType: "xfs"
  pool: "pool-hdd"
  storageConnectionName: "yad1"
  reclaimPolicy: Delete
EOF
```

- Проверить создание объекта можно командой (Phase должен быть `Created`):

```shell
kubectl get yadrotatlinunifiedstorageconnections.storage.deckhouse.io <имя yadrotatlinunifiedstorageconnection>
```

```shell
kubectl get yadrotatlinunifiedstorageclasses.storage.deckhouse.io <имя yadrotatlinunifiedstorageclass>
```

### Проверка работоспособности модуля.

Проверить работоспособность модуля можно [так](./faq.html#как-проверить-работоспособность-модуля)
