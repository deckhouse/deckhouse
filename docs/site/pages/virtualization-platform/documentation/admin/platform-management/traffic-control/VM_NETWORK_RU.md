---
title: "Сеть виртуальных машин"
permalink: ru/virtualization-platform/documentation/admin/platform-management/traffic-control/vm-network.html
lang: ru
---

Каждой виртуальной машине выделяется адрес из диапазонов заданных в настройках ModuleConfig [virtualization](../../../reference/configuration.module.html#virtualization) в блоке `.spec.settings.virtualMachineCIDRs`

Для просмотра текущей конфигурации - выполните команду:

```bash
d8 k get mc virtualization
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

Для редактирования списка подсетей используйте следующую команду:

```bash
d8 k edit mc virtualization
```

Адреса назначаются последовательно из каждого указанного диапазона, исключаются только первый (адрес сети) и последний (широковещательны адрес).

При выделении IP-адреса виртуальной машине, создаётся соответствующий кластерный ресурс [VirtualMachineIPAddressLease](../../../../reference/cr.html#virtualmachineipaddresslease), который связывается с проектным ресурсом [VirtualMachineIPAddress](../../../../reference/cr.html#virtualmachineipaddress), который в свою очередь связан с виртуальной машиной.

После удаления [VirtualMachineIPAddress](../../../../reference/cr.html#virtualmachineipaddress), адрес отвязывается и в течении 10 минут остаётся зарезервирован за проектом, в проектом в котором он использовался ранее.
