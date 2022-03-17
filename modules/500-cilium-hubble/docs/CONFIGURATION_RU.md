---
title: "Модуль cni-flannel: настройки"
---

Модуль включается **автоматически** для следующих cloud-provider'ов:
- [OpenStack](../../modules/030-cloud-provider-openstack/)
- [VMware vSphere](../../modules/030-cloud-provider-vsphere/)

Для включения в bare metal, необходимо в configMap Deckhouse добавить:
```
ciliumHubbleEnabled: "true"
```

## Параметры

<!-- SCHEMA -->

