---
title: "Cloud provider — VMware vSphere: настройки провайдера"
description: Настройки облачного провайдера Deckhouse для VMware vSphere.
---

> Если control plane кластера размещен на виртуальных машинах или bare-metal-серверах, cloud-провайдер использует настройки модуля `cloud-provider-vsphere` в конфигурации Deckhouse. Иначе, если control plane кластера размещен в облаке, cloud-провайдер использует структуру [VsphereClusterConfiguration](#vsphereclusterconfiguration) для настройки.
>
> Дополнительная информация о [Vsphere Cloud Load Balancers](https://github.com/kubernetes/cloud-provider-vsphere/tree/master/pkg/cloudprovider/vsphere/loadbalancer).

<!-- SCHEMA -->
