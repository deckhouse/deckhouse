---
title: "Сеть виртуальных машин"
permalink: ru/admin/configuration/network/vm-network.html
description: "Подсети, из которых виртуальные машины получают IP-адреса, и правила их изменения в настройках модуля virtualization."
search: сеть виртуальных машин, virtualMachineCIDRs, подсети ВМ, IP-адреса
lang: ru
---

В блоке [`.spec.settings.virtualMachineCIDRs`](/modules/virtualization/configuration.html#parameters-virtualmachinecidrs) перечисляются подсети в формате CIDR, из которых DP выдаёт IP-адреса виртуальным машинам автоматически или по запросу.
Указывайте начальный адрес подсети, выровненный по маске, например `192.168.1.192/27`, а не произвольный адрес из диапазона.

Пример:

```yaml
spec:
  settings:
    virtualMachineCIDRs:
      - 10.66.10.0/24
      - 10.66.20.0/24
      - 10.77.20.0/16
```

Первый и последний адреса каждой подсети зарезервированы и виртуальным машинам не выдаются. Например, в подсети `10.66.10.0/24` недоступны адреса `10.66.10.0` и `10.66.10.255`.

Блок можно не задавать. Модуль в этом случае включится, но работать с адресами виртуальных машин уже нельзя, а именно:

- создать или использовать ресурс [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) нельзя;
- виртуальная машина не может запросить сеть `Main` в параметре [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks);
- параметр [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) виртуальной машины не может быть пустым.

{% alert level="warning" %}
Подсети блока [`.spec.settings.virtualMachineCIDRs`](/modules/virtualization/configuration.html#parameters-virtualmachinecidrs) не должны пересекаться с подсетями узлов кластера, подсетью сервисов или подсетью подов (`podCIDR`).

Удалить подсеть, из которой уже выданы адреса виртуальным машинам, нельзя. Заданный блок также нельзя очистить полностью.
{% endalert %}
