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

На этой странице представлены инструкции по подключению NetApp к DKP, настройке соединения и созданию StorageClass.

## Системные требования

Перед настройкой работы с СХД NetApp убедитесь, что выполнены следующие требования:

- Наличие развернутой и настроенной СХД NetApp.
- Уникальные IQN в `/etc/iscsi/initiatorname.iscsi` на каждом узле Kubernetes.

## Настройка интеграции кластера с системой хранения NetApp

Чтобы начать работу с СХД NetApp, следуйте пошаговым инструкциям ниже. Все команды выполняйте на машине с административным доступом к API Kubernetes.

{% alert level="info" %}
Для работы со снимками требуется подключенный модуль [snapshot-controller](/modules/snapshot-controller/).
{% endalert %}

1. Выполните команду для активации модуля `csi-netapp`:

   ```shell
   d8 s module enable csi-netapp
   ```

   После активации модуля на всех узлах кластера будут:

   - Зарегистрирован CSI-драйвер;
   - Развернуты служебные поды компонентов `csi-netapp`.

1. Дождитесь перехода модуля в состояние `Ready`:

   ```shell
   d8 k get module csi-netapp -w
   ```

1. Убедитесь, что все поды в пространстве имен `d8-csi-netapp` находятся в состоянии `Running` или `Completed` и развернуты на всех узлах кластера:

   ```shell
   d8 k -n d8-csi-netapp get pod -owide -w
   ```

1. Создайте ресурс [NetappStorageConnection](/modules/csi-netapp/cr.html#netappstorageconnection):

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: NetappStorageConnection
   metadata:
     name: netapp
   spec:
     controlPlane:
       address: "172.17.1.55"
       username: "admin"
       password: "password"
       svm: "svm1"
   EOF
   ```

1. Проверьте создание объекта следующей командой (`Phase` должен быть `Created`):

   ```shell
   d8 k get netappstorageconnections.storage.deckhouse.io <имя netappstorageconnection>
   ```

1. Создайте ресурс [NetappStorageClass](/modules/csi-netapp/cr.html#netappstorageclass):

   ```shell
   d8 k apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: NetappStorageClass
   metadata:
     name: netapp
   spec:
     pool: "test-cpg"
     accessProtocol: "iscsi" # fc или iscsi (по умолчанию iscsi), неизменяемый параметр.
     fsType: "xfs" # xfs, ext3, ext4 (по умолчанию ext4), изменяемый параметр.
     storageConnectionName: "netapp" # Неизменяемый параметр.
     reclaimPolicy: Delete # Delete или Retain.
     cpg: "test-cpg"
   EOF
   ```

1. Проверьте создание объекта следующей командой (`Phase` должен быть `Created`):

   ```shell
   d8 k get netappstorageclasses.storage.deckhouse.io <имя netappstorageclass>
   ```

Теперь система хранения NetApp готова к работе. Вы можете использовать созданный StorageClass для создания PersistentVolumeClaim в ваших приложениях.
