---
title: "Модуль cni-flannel: настройки"
---

Модуль включается **автоматически** для следующих cloud provider'ов:
- [OpenStack](../../modules/030-cloud-provider-openstack/);
- [VMware vSphere](../../modules/030-cloud-provider-vsphere/).

{% include module-enable.liquid moduleName="cni-flannel" %}

## Параметры

<!-- SCHEMA -->

## Пример конфигурации

```yaml
cniFlannel: |
  podNetworkMode: VXLAN
```
