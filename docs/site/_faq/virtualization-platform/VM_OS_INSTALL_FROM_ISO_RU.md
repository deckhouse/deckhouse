---
title: Как установить ОС в виртуальной машине из ISO-образа?
section: vm_operations
lang: ru
---

Ниже приведён типовой сценарий установки гостевой ОС Windows из ISO-образа. Перед началом разместите ISO-образ на HTTP-ресурсе, доступном из кластера.

1. Создайте пустой [VirtualDisk](/modules/virtualization/cr.html#virtualdisk) для установки ОС:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualDisk
   metadata:
     name: win-disk
     namespace: default
   spec:
     persistentVolumeClaim:
       size: 100Gi
       storageClassName: local-path
   ```

1. Создайте ресурсы [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) для ISO-образа ОС Windows и дистрибутива драйверов `VirtIO`:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: win-11-iso
   spec:
     dataSource:
       type: HTTP
       http:
         url: "http://example.com/win11.iso"
   ```

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: ClusterVirtualImage
   metadata:
     name: win-virtio-iso
   spec:
     dataSource:
       type: HTTP
       http:
         url: "https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso"
   ```

1. Создайте виртуальную машину:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachine
   metadata:
     name: win-vm
     namespace: default
     labels:
       vm: win
   spec:
     virtualMachineClassName: generic
     runPolicy: Manual
     osType: Windows
     bootloader: EFI
     cpu:
       cores: 6
       coreFraction: 50%
     memory:
       size: 8Gi
     enableParavirtualization: true
     blockDeviceRefs:
       - kind: VirtualDisk
         name: win-disk
       - kind: ClusterVirtualImage
         name: win-11-iso
       - kind: ClusterVirtualImage
         name: win-virtio-iso
   ```

1. Запустите виртуальную машину:

   ```bash
   d8 v start win-vm
   ```

1. Подключитесь к консоли ВМ и завершите установку ОС и драйверов `VirtIO` при помощи графического установщика.

   Подключение по VNC:

   ```bash
   d8 v vnc -n default win-vm
   ```

1. После завершения установки перезагрузите виртуальную машину.

1. Для дальнейшей работы снова подключитесь по VNC:

   ```bash
   d8 v vnc -n default win-vm
   ```
