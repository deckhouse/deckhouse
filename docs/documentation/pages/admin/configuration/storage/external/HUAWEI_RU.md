---
title: "Хранилище данных Huawei"
permalink: ru/admin/configuration/storage/external/huawei.html
lang: ru
---

В Deckhouse предусмотрена поддержка систем хранения данных (СХД) Huawei Dorado, позволяющая управлять томами в Kubernetes с использованием CSI-драйвера через создание пользовательских ресурсов [HuaweiStorageClass](/modules/csi-huawei/cr.html#huaweistorageclass). Это решение обеспечивает высокую производительность и отказоустойчивость хранения данных, что делает его оптимальным выбором для критически важных рабочих нагрузок.

{% alert level="warning" %}
Создание StorageClass для CSI-драйвера `csi.huawei.com` пользователем запрещено.
Модулем поддерживаются только СХД Huawei Dorado. Для использования других СХД Huawei свяжитесь [с технической поддержкой Deckhouse](/tech-support/).
{% endalert %}

На этой странице представлены инструкции по подключению Huawei Dorado к Deckhouse, настройке соединения, созданию StorageClass, а также проверке работоспособности хранилища.

## Системные требования

- Наличие развернутой и настроенной СХД Huawei.
- Уникальные IQN в `/etc/iscsi/initiatorname.iscsi` на каждом узле Kubernetes.

## Настройка

Все команды следует выполнять на машине, имеющей доступ к API Kubernetes с правами администратора.

### Включение модуля

Для поддержки систем хранения данных Huawei Dorado включите [модуль `csi-huawei`](/modules/csi-huawei/). Это приведет к тому, что на всех узлах кластера будет:

- зарегистрирован CSI-драйвер;
- запущены служебные поды компонентов `csi-huawei`.

```shell
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

Дождитесь, когда модуль перейдет в состояние `Ready`. Проверьте состояние модуля следующей командой:

```shell
d8 k get module csi-huawei -w
```

### Создание StorageClass

Для создания StorageClass необходимо использовать ресурсы [HuaweiStorageClass](/modules/csi-huawei/cr.html#huaweistorageclass) и [HuaweiStorageConnection](/modules/csi-huawei/cr.html#huaweistorageconnection). Пример команд для создания таких ресурсов:

- Создание ресурса HuaweiStorageConnection:

  ```shell
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

  Проверьте создание объекта следующей командой (`Phase` должен быть `Created`):

  ```shell
  d8 k get huaweistorageconnections.storage.deckhouse.io <имя huaweistorageconnection>
  ```

- Создание ресурса HuaweiStorageClass:

  ```shell
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

  Проверьте создание объекта следующей командой (`Phase` должен быть `Created`):

  ```shell
  d8 k get huaweistorageclasses.storage.deckhouse.io <имя huaweistorageclass>
  ```

### Проверка работоспособности модуля

Для проверки работоспособности модуля убедитесь, что все поды в пространстве имён `d8-csi-huawei`находятся в статусе `Running` или `Completed` и запущены на каждом узле кластера:

```shell
d8 k -n d8-csi-huawei get pod -owide -w
```
