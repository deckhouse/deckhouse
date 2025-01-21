---
title: "Модуль csi-nfs"
description: "Модуль csi-nfs: общие концепции и положения."
moduleStatus: experimental
---

Модуль предоставляет CSI для управления томами на основе `NFS`. Модуль позволяет создавать `StorageClass` в `Kubernetes` через создание [пользовательских ресурсов Kubernetes](./cr.html#nfsstorageclass) `NFSStorageClass`.

> **Внимание!** Создание `StorageClass` для CSI-драйвера `nfs.csi.k8s.io` пользователем запрещено.

## Системные требования и рекомендации

### Требования

- Использование стоковых ядер, поставляемых вместе с [поддерживаемыми дистрибутивами](https://deckhouse.ru/documentation/v1/supported_versions.html#linux);
- Наличие развернутого и настроенного `NFS` сервера.

## Быстрый старт

Все команды следует выполнять на машине, имеющей доступ к API Kubernetes с правами администратора.

### Включение модуля

- Включить модуль `csi-nfs`.  Это приведет к тому, что на всех узлах кластера будет:
    - зарегистрирован CSI драйвер;
    - запущены служебные поды компонентов `csi-nfs`.

```shell
kubectl apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-nfs
spec:
  enabled: true
  version: 1
EOF
```

- Дождаться, когда модуль перейдет в состояние `Ready`.

```shell
kubectl get module csi-nfs -w
```

### Создание StorageClass

Для создания StorageClass необходимо использовать ресурс [NFSStorageClass](./cr.html#nfsstorageclass). Пример команды для создания такого ресурса:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: NFSStorageClass
metadata:
  name: nfs-storage-class
spec:
  connection:
    host: 10.223.187.3
    share: /
    nfsVersion: "4.1"
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```
 
Для каждой `PV` будет создаваться каталог `<директория из share>/<имя PV>`.

### Проверка работоспособности модуля.

Проверить работоспособность модуля можно [так](./faq.html#как-проверить-работоспособность-модуля)
