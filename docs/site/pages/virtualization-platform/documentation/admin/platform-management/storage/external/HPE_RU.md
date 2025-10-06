---
title: "Хранилище данных HPE"
permalink: ru/virtualization-platform/documentation/admin/platform-management/storage/external/hpe.html
lang: ru
---

В Deckhouse Virtualization Platform (DVP) реализована поддержка систем хранения данных (СХД) HPE 3PAR для управления томами в Kubernetes с использованием CSI-драйвера. Такая интеграция обеспечивает надежное, масштабируемое и высокопроизводительное хранилище, подходящее для критически важных рабочих нагрузок. Для работы с системами хранения HPE 3PAR применяется модуль `csi-hpe`, с помощью которого можно создавать StorageClass в Kubernetes через создание ресурса [HPEStorageClass](/modules/csi-hpe/stable/cr.html#hpestorageclass).

{% alert level="warning" %}
Создание StorageClass для CSI-драйвера `csi.hpe.com` пользователем запрещено.
Модулем поддерживаются только СХД HPE 3PAR. Для использования других СХД HPE свяжитесь [с технической поддержкой](https://deckhouse.ru/tech-support/).
{% endalert %}

На этой странице представлены инструкции по подключению HPE 3PAR в DVP, настройке соединения, созданию StorageClass, а также проверке работоспособности хранилища.

## Системные требования

- Наличие развернутой и настроенной СХД HPE.
- Уникальные IQN в `/etc/iscsi/initiatorname.iscsi` на каждом узле Kubernetes.

## Настройка

Все команды следует выполнять на машине, имеющей доступ к API Kubernetes с правами администратора.

### Включение модуля

Включите модуль `csi-hpe`. Это приведет к тому, что на всех узлах кластера будет:

- зарегистрирован CSI-драйвер;
- запущены служебные поды компонентов `csi-hpe`.

```shell
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

Дождитесь, когда модуль перейдет в состояние `Ready`. Проверьте состояние модуля следующей командой:

```shell
d8 k get module csi-hpe -w
```

### Создание StorageClass

Для создания StorageClass необходимо использовать ресурсы [HPEStorageClass](/modules/csi-hpe/stable/cr.html#hpestorageclass) и [HPEStorageConnection](/modules/csi-hpe/stable/cr.html#hpestorageconnection). Пример команд для создания таких ресурсов:

- Создание ресурса HPEStorageConnection:

  ```shell
  d8 k apply -f -<<EOF
  apiVersion: storage.deckhouse.io/v1alpha1
  kind: HPEStorageConnection
  metadata:
    name: hpe
  spec:
    controlPlane:
      backendAddress: "172.17.1.55" # Адрес СХД (изменяемый параметр).
      username: "3paradm" # Имя пользователя для доступа к API (изменяемый параметр).
      password: "3pardata" # Пароль для доступа к API (изменяемый параметр).
      serviceName: "primera3par-csp-svc"
      servicePort: "8080"
  EOF
  ```

  Проверьте создание объекта следующей командой (`Phase` должен быть `Created`):

  ```shell
  d8 k get hpestorageconnections.storage.deckhouse.io <имя hpestorageconnection>
  ```

- Создание ресурса HPEStorageClass:

  ```shell
  d8 k apply -f -<<EOF
  apiVersion: storage.deckhouse.io/v1alpha1
  kind: HPEStorageClass
  metadata:
    name: hpe
  spec:
    pool: "test-cpg"
    accessProtocol: "fc" # fc или iscsi (по умолчанию iscsi), неизменяемый параметр.
    fsType: "xfs" # xfs, ext3, ext4, btrfs (по умолчанию ext4), изменяемый параметр.
    storageConnectionName: "3par" # Неизменяемый параметр.
    reclaimPolicy: Delete # Delete или Retain.
    cpg: "test-cpg"
    EOF
  ```

  Проверьте создание объекта следующей командой (`Phase` должен быть `Created`):

  ```shell
  d8 k get hpestorageclasses.storage.deckhouse.io <имя hpestorageclass>
  ```

### Проверка работоспособности модуля

Для проверки работоспособности модуля убедитесь, что все поды в пространстве имён `d8-csi-hpe`находятся в статусе `Running` или `Completed` и запущены на каждом узле кластера:

```shell
d8 k -n d8-csi-hpe get pod -owide -w
```
