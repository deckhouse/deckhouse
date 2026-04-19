---
title: "Сеть виртуальных машин"
permalink: ru/virtualization-platform/documentation/admin/platform-management/network/vm-network.html
lang: ru
---

Каждой виртуальной машине выделяется адрес из диапазонов, заданных в настройках `ModuleConfig` [virtualization](/modules/virtualization/configuration.html) в блоке `.spec.settings.virtualMachineCIDRs`.

Для просмотра текущей конфигурации выполните команду:

```bash
d8 k get mc virtualization -oyaml
```

Пример вывода:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  settings:
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 60G
          storageClassName: linstor-thin-r1
        type: PersistentVolumeClaim
    virtualMachineCIDRs:
      - 10.66.10.0/24
      - 10.66.20.0/24
      - 10.66.30.0/24
  version: 1
```

Для редактирования списка подсетей используйте команду:

```bash
d8 k edit mc virtualization
```

Адреса назначаются последовательно из каждого указанного диапазона, исключаются только первый (адрес сети) и последний (широковещательный адрес).

При назначении IP-адреса виртуальной машине создается соответствующий кластерный ресурс [VirtualMachineIPAddressLease](/modules/virtualization/cr.html#virtualmachineipaddresslease), который связывается с проектным ресурсом [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress), а тот, в свою очередь, — с виртуальной машиной.

После удаления ресурса VirtualMachineIPAddress, IP-адрес отвязывается, но остается зарезервированным за проектом в течение 10 минут после его удаления.
