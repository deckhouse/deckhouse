---
title: "Хранилище данных HPE"
permalink: ru/storage/admin/external/hpe.html
lang: ru
---

В Deckhouse предусмотрена поддержка систем хранения данных HPE 3PAR, позволяющая управлять томами в Kubernetes с использованием CSI-драйвера. Это обеспечивает надежное, масштабируемое и высокопроизводительное хранилище, подходящее для критически важных рабочих нагрузок. Для поддержки систем хранения данных HPE 3PAR используется модуль `csi-hpe`, который позволяет создавать StorageClass в Kubernetes через создание [HPEStorageClass](../../../reference/cr/hpestorageclass/).

{% alert level="warning" %}
Создание StorageClass для CSI-драйвера `csi.hpe.com` пользователем запрещено.
Модулем поддерживаются только СХД HPE 3PAR. Для использования других СХД HPE, пожалуйста, свяжитесь с технической поддержкой.
{% endalert %}

На этой странице представлены инструкции по подключению HPE 3PAR в Deckhouse, настройке соединения, созданию StorageClass, а также проверке работоспособности хранилища.

## Системные требования

- Наличие развернутой и настроенной СХД HPE;
- Уникальные IQN в `/etc/iscsi/initiatorname.iscsi` на каждой из Kubernetes Nodes.

## Настройка и конфигурация

Все команды следует выполнять на машине, имеющей доступ к API Kubernetes с правами администратора.

### Включение модуля

Включите модуль `csi-hpe`. Это приведет к тому, что на всех узлах кластера будет:
- зарегистрирован CSI-драйвер;
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

Дождитесь, когда модуль перейдет в состояние `Ready`.

```shell
d8 k get module csi-hpe -w
```

### Создание StorageClass

Для создания StorageClass необходимо использовать ресурсы [HPEStorageClass](../../../reference/cr/hpestorageclass/) и [HPEStorageConnection](../../../reference/cr/hpestorageconnection/). Пример команд для создания таких ресурсов:

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

Проверить создание объекта можно командой (Phase должен быть `Created`):

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
