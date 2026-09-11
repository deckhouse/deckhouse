---
title: Как создать golden image для Linux?
subsystems:
- virtualization
lang: ru
---

Golden image — это предварительно настроенный образ виртуальной машины, который можно использовать для быстрого создания новых ВМ с уже установленным программным обеспечением и настройками.

1. Создайте виртуальную машину, установите на неё необходимое программное обеспечение и выполните все требуемые настройки.

1. Установите и настройте `qemu-guest-agent` (рекомендуется):

   - Для RHEL/CentOS:

     ```bash
     yum install -y qemu-guest-agent
     ```

   - Для Debian/Ubuntu:

     ```bash
     apt-get update
     apt-get install -y qemu-guest-agent
     ```

1. Включите и запустите сервис:

   ```bash
   systemctl enable qemu-guest-agent
   systemctl start qemu-guest-agent
   ```

1. Задайте машине политику запуска [`AlwaysOnUnlessStoppedManually`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-runpolicy), иначе выключить её не получится.

1. Подготовьте образ. Очистите неиспользуемые блоки файловой системы:

   ```bash
   fstrim -v /
   fstrim -v /boot
   ```

1. Очистите сетевые настройки:

   - Для RHEL:

     ```bash
     nmcli con delete $(nmcli -t -f NAME,DEVICE con show | grep -v ^lo: | cut -d: -f1)
     rm -f /etc/sysconfig/network-scripts/ifcfg-eth*
     ```

   - Для Debian/Ubuntu:

     ```bash
     rm -f /etc/network/interfaces.d/*
     ```

1. Очистите системные идентификаторы:

   ```bash
   echo -n > /etc/machine-id
   rm -f /var/lib/dbus/machine-id
   ln -s /etc/machine-id /var/lib/dbus/machine-id
   ```

1. Удалите ключи хоста SSH:

   ```bash
   rm -f /etc/ssh/ssh_host_*
   ```

1. Очистите журнал systemd:

   ```bash
   journalctl --vacuum-size=100M --vacuum-time=7d
   ```

1. Очистите кеш пакетных менеджеров:

   - Для RHEL:

     ```bash
     yum clean all
     ```

   - Для Debian/Ubuntu:

     ```bash
     apt-get clean
     ```

1. Очистите временные файлы:

   ```bash
   rm -rf /tmp/*
   rm -rf /var/tmp/*
   ```

1. Очистите логи:

   ```bash
   find /var/log -name "*.log" -type f -exec truncate -s 0 {} \;
   ```

1. Очистите историю команд:

   ```bash
   history -c
   ```

1. В RHEL сбросьте и восстановите контексты SELinux одним из двух способов.

   Восстановите контексты сразу:

   ```bash
   restorecon -R /
   ```

   Либо запланируйте пересчёт контекстов на следующую загрузку:

   ```bash
   touch /.autorelabel
   ```

1. Проверьте, что в `/etc/fstab` указаны UUID или `LABEL`, а не имена вида `/dev/sdX`:

   ```bash
   blkid
   cat /etc/fstab
   ```

1. Сбросьте состояние cloud-init (логи и seed):

   ```bash
   cloud-init clean --logs --seed
   ```

1. Выполните финальную синхронизацию и очистку буферов:

   ```bash
   sync
   echo 3 > /proc/sys/vm/drop_caches
   ```

1. Выключите виртуальную машину:

   ```bash
   poweroff
   ```

1. Создайте ресурс [VirtualImage](/modules/virtualization/cr.html#virtualimage), указав исходный ресурс [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) подготовленной ВМ:

   ```bash
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualImage
   metadata:
     name: <IMAGE_NAME>
     namespace: <NAMESPACE>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualDisk
         name: <SOURCE_DISK_NAME>
   EOF
   ```

   Либо создайте ресурс [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage), чтобы образ был доступен на уровне кластера для всех проектов:

   ```bash
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: <IMAGE_NAME>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualDisk
         name: <SOURCE_DISK_NAME>
         namespace: <NAMESPACE>
   EOF
   ```

   Здесь `<IMAGE_NAME>` — имя создаваемого образа, `<NAMESPACE>` — неймспейс подготовленной машины, а `<SOURCE_DISK_NAME>` — имя её диска.

1. Создайте новый ресурс [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) из полученного образа:

   ```bash
   d8 k apply -f -<<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: <VM_DISK_NAME>
     namespace: <NAMESPACE>
   spec:
     dataSource:
       type: ObjectRef
       objectRef:
         kind: VirtualImage
         name: <IMAGE_NAME>
   EOF
   ```

   Здесь `<VM_DISK_NAME>` — имя диска новой машины.

После этих шагов golden image готов, и из него быстро создаются новые машины с уже установленным программным обеспечением и настройками.
