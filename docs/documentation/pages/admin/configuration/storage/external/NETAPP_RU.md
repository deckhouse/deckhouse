---
title: "Хранилище данных NetApp"
permalink: ru/admin/configuration/storage/external/netapp.html
lang: ru
---

В Deckhouse Kubernetes Platform (DKP) реализована поддержка систем хранения данных (СХД) NetApp для управления томами в Kubernetes с использованием CSI-драйвера. Такая интеграция обеспечивает надежное, масштабируемое и высокопроизводительное хранилище, подходящее для критически важных рабочих нагрузок. Для работы с системами хранения NetApp применяется [модуль `csi-netapp`](/modules/csi-netapp/), с помощью которого можно создавать StorageClass в Kubernetes через создание ресурса [NetappStorageClass](/modules/csi-netapp/cr.html#netappstorageclass).

{% alert level="warning" %}
Создание StorageClass для CSI-драйвера `csi.netapp.com` пользователем запрещено.
На данный момент модулем поддерживаются СХД, совместимые с [Trident CSI от NetApp](https://github.com/NetApp/trident). Для поддержки других СХД NetApp свяжитесь [с технической поддержкой Deckhouse](/tech-support/).
{% endalert %}

На этой странице представлены инструкции по подключению NetApp в DKP, настройке соединения, созданию StorageClass, а также проверке работоспособности хранилища.

## Системные требования

Перед настройкой модуля `csi-netapp` убедитесь, что выполнены следующие требования:

- Наличие развернутой и настроенной СХД NetApp.
- Уникальные IQN в `/etc/iscsi/initiatorname.iscsi` на каждом узле Kubernetes.

## Настройка

Для начала работы с СХД NetApp включите [модуль `csi-netapp`](/modules/csi-netapp/) и выполните настройку подключения к системе хранения. Выполняйте все команды на машине, имеющей доступ к API Kubernetes с правами администратора.

{% alert level="info" %}
Для работы со снимками требуется подключенный модуль [snapshot-controller](../../snapshot-controller/).
{% endalert %}

### Создание StorageClass

Создайте StorageClass для работы с томами NetApp. Используйте ресурсы [NetappStorageClass](/modules/csi-netapp/cr.html#netappstorageclass) и [NetappStorageConnection](/modules/csi-netapp/cr.html#netappstorageconnection) для создания StorageClass. Пример команд для создания таких ресурсов:

1. Создайте ресурс NetappStorageConnection:

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: NetappStorageConnection
   metadata:
     name: netapp
   spec:
     controlPlane:
       backendAddress: "172.17.1.55" # Адрес СХД (изменяемый параметр).
       username: "admin" # Имя пользователя для доступа к API (изменяемый параметр).
       password: "password" # Пароль для доступа к API (изменяемый параметр).
       serviceName: "trident-csp-svc"
       servicePort: "8080"
   EOF
   ```

1. Проверьте создание объекта следующей командой (`Phase` должен быть `Created`):

   ```shell
   d8 k get netappstorageconnections.storage.deckhouse.io <имя netappstorageconnection>
   ```

1. Создайте ресурс NetappStorageClass:

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: NetappStorageClass
   metadata:
     name: netapp
   spec:
     pool: "test-pool"
     accessProtocol: "fc" # fc или iscsi (по умолчанию iscsi), неизменяемый параметр.
     fsType: "xfs" # xfs, ext3, ext4 (по умолчанию ext4), изменяемый параметр.
     storageConnectionName: "netapp" # Неизменяемый параметр.
     reclaimPolicy: Delete # Delete или Retain.
     cpg: "test-pool"
   EOF
   ```

1. Проверьте создание объекта следующей командой (`Phase` должен быть `Created`):

   ```shell
   d8 k get netappstorageclasses.storage.deckhouse.io <имя netappstorageclass>
   ```

### Проверка работоспособности модуля

Проверьте корректность работы модуля `csi-netapp`. Убедитесь, что все поды в пространстве имён `d8-csi-netapp` находятся в статусе `Running` или `Completed` и запущены на каждом узле кластера:

```shell
d8 k -n d8-csi-netapp get pod -owide -w
```
