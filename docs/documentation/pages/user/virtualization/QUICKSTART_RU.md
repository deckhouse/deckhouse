---
title: "Быстрый старт"
permalink: ru/user/virtualization/quickstart.html
description: "Быстрый старт: создание виртуальной машины из образа, подключение диска и вход в гостевую систему по SSH."
search: быстрый старт, создание ВМ, первая виртуальная машина
lang: ru
---

В разделе показан минимальный сценарий, в котором вы создаёте образ Ubuntu 24.04, диск из этого образа и виртуальную машину (ВМ), подключаетесь к ней по консоли, а затем удаляете созданные ресурсы.

{% tabs quickstart %}

{% tab "В командной строке" %}

1. Создайте образ [VirtualImage](/modules/virtualization/cr.html#virtualimage) из внешнего источника:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: ubuntu
   spec:
     storage: ContainerRegistry
     dataSource:
       type: HTTP
       http:
         url: https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
   EOF
   ```

1. Создайте диск [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) из этого образа. Убедитесь, что в кластере задан StorageClass по умолчанию, и примените манифест:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: linux-disk
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualImage
         name: ubuntu
   EOF
   ```

1. Создайте виртуальную машину [VirtualMachine](/modules/virtualization/cr.html#virtualmachine). В примере используется сценарий cloud-init, который создаёт пользователя `cloud`:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachine
   metadata:
     name: linux-vm
   spec:
     virtualMachineClassName: generic
     cpu:
       cores: 1
     memory:
       size: 1Gi
     provisioning:
       type: UserData
       userData: |
         #cloud-config
         ssh_pwauth: True
         users:
           - name: cloud
             passwd: <PASSWORD_HASH>
             shell: /bin/bash
             sudo: ALL=(ALL) NOPASSWD:ALL
             lock_passwd: False
     blockDeviceRefs:
       - kind: VirtualDisk
         name: linux-disk
   EOF
   ```

   Здесь `<PASSWORD_HASH>` — хеш пароля пользователя в кавычках. Получите его командой `mkpasswd --method=SHA-512 --rounds=4096`, она запросит пароль и выведет готовое значение. Формат сценария описан в [документации cloud-init](https://cloudinit.readthedocs.io/).

1. Проверьте, что образ и диск созданы, а ВМ запущена. Ресурсы переходят в готовое состояние не мгновенно, поэтому дождитесь нужных значений в колонке `PHASE`:

   ```bash
   d8 k get vi,vd,vm
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                                                 PHASE   CDROM   PROGRESS   AGE
   virtualimage.virtualization.deckhouse.io/ubuntu      Ready   false   100%       7h50m

   NAME                                                 PHASE   CAPACITY   VIRTUALMACHINE   AGE
   virtualdisk.virtualization.deckhouse.io/linux-disk   Ready   4Gi        linux-vm         7h40m

   NAME                                                 PHASE     UPTIME   NODE           IPADDRESS    AGE
   virtualmachine.virtualization.deckhouse.io/linux-vm  Running   7h30m    virtlab-pt-2   10.66.10.2   7h46m
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Подключитесь к ВМ по консоли:

   ```bash
   d8 v console linux-vm
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   Successfully connected to linux-vm console. The escape sequence is ^]

   linux-vm login: cloud
   Password:
   ...
   cloud@linux-vm:~$
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Чтобы выйти из консоли, нажмите `Ctrl+]`.

1. Удалите созданные ресурсы:

   ```bash
   d8 k delete vm linux-vm
   d8 k delete vd linux-disk
   d8 k delete vi ubuntu
   ```

{% endtab %}

{% tab "В веб-интерфейсе" %}

1. Создайте образ из внешнего источника:

   1. Перейдите на вкладку «Проекты» и выберите нужный проект.
   1. Перейдите в раздел «Виртуализация» → «Образы».
   1. Нажмите кнопку «Создать».
   1. В блоке «Источник» выберите «По ссылке».
   1. В открывшейся форме в поле «Имя образа» введите `ubuntu`.
   1. В блоке «Хранилище» в поле «Тип хранилища» выберите `ContainerRegistry`.
   1. В поле «URL» вставьте `https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img`.
   1. Нажмите кнопку «Создать».
   1. Проверьте статус образа на его странице.

1. Создайте диск из этого образа. Шаг можно пропустить и создать диск сразу при создании ВМ.

   1. Перейдите в раздел «Виртуализация» → «Диски».
   1. Нажмите кнопку «Создать».
   1. В открывшейся форме в поле «Имя диска» введите `linux-disk`.
   1. В поле «Источник» из выпадающего списка выберите образ `ubuntu`.
   1. При необходимости в поле «Размер» укажите больший размер, например `5Gi`.
   1. В поле «Класс хранения» выберите StorageClass или оставьте вариант по умолчанию.
   1. Нажмите кнопку «Создать».
   1. Проверьте статус диска на его странице.

   > Если у выбранного StorageClass задан режим `WaitForFirstConsumer`, диск ожидает создания ВМ, которая его использует.
   > До этого момента диск отображается со статусом «СОЗДАНИЕ 0%», но его уже можно выбрать при создании ВМ.

1. Создайте виртуальную машину:

   1. Перейдите в раздел «Виртуализация» → «Виртуальные машины».
   1. Нажмите кнопку «Создать».
   1. В открывшейся форме в поле «Имя» введите `linux-vm`.
   1. В разделах «Платформа» и «Ресурсы» оставьте настройки по умолчанию.
   1. В разделе «Диски» нажмите кнопку «Добавить».

      Если диск уже создан, в открывшемся окне «Диски / Образы» выберите «Существующий» и укажите в списке диск `linux-disk`.

      Если диск не создан, в том же окне выберите «Создать из» и задайте параметры:

      - в поле «Название» введите `linux-disk`;
      - в поле «Источник» из выпадающего списка выберите образ `ubuntu`, в списке указан тип ресурса;
      - при необходимости в поле «Размер» укажите больший размер, например `5Gi`;
      - в поле «Хранилище» выберите StorageClass или оставьте вариант по умолчанию.

      Нажмите кнопку «Добавить».

   1. Прокрутите страницу вниз до переключателя «Cloud-init» и включите его.
   1. В появившееся поле вставьте сценарий, подставив вместо `<PASSWORD_HASH>` хеш пароля в кавычках, полученный командой `mkpasswd --method=SHA-512 --rounds=4096`:

      ```yaml
      #cloud-config
      ssh_pwauth: True
      users:
        - name: cloud
          passwd: <PASSWORD_HASH>
          shell: /bin/bash
          sudo: ALL=(ALL) NOPASSWD:ALL
          lock_passwd: False
      ```

   1. Нажмите кнопку «Создать».
   1. Проверьте статус ВМ на её странице.

1. Подключитесь к ВМ по консоли:

   1. Перейдите в раздел «Виртуализация» → «Виртуальные машины».
   1. Из списка выберите нужную ВМ и нажмите на её имя.
   1. В открывшейся форме перейдите на вкладку «TTY» и войдите в систему в окне консоли.

1. Удалите созданные ресурсы:

   1. Перейдите в раздел «Виртуализация» и выберите нужный подраздел, например «Виртуальные машины», «Диски» или «Образы».
   1. В строке ресурса нажмите кнопку с многоточием и выберите «Удалить». В некоторых списках, например в списке снимков ВМ, удаление вынесено отдельной кнопкой.
   1. В окне подтверждения нажмите кнопку «Удалить» либо откажитесь от действия кнопкой «Не удалять».

   > **Важно:** Удаление ресурса необратимо. Диск, подключённый к запущенной виртуальной машине, удалить нельзя, для него пункт «Удалить» неактивен.

{% endtab %}

{% endtabs %}
