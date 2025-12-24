---
title: Интеграция со службами VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/services.html
lang: ru
---

Deckhouse Kubernetes Platform интегрируется с инфраструктурой VMware vSphere и использует [ресурсы VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) для описания характеристик виртуальных машин, создаваемых в составе кластера Kubernetes.

Основные возможности:

- Заказ и удаление виртуальных машин через vCenter API;
- Размещение узлов кластера в разных кластерах ([`zones`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-zones)) и датацентрах ([`region`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-region));
- Использование шаблонов виртуальных машин с `cloud-init`;
- Поддержка сетей с DHCP, статической адресацией и дополнительными интерфейсами;
- Работа с хранилищем: заказ root-дисков и PVC на базе Datastore или CNS-дисков;
- Поддержка механизмов балансировки входящего трафика:
  - через внешние балансировщики;
  - через MetalLB (в режиме BGP).

{% alert level="info" %}
Возможно создание гибридного кластера с узлами на vSphere и bare-metal.
{% endalert %}
