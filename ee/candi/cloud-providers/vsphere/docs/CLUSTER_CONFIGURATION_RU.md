---
title: "Cloud provider — VMware vSphere: настройки провайдера"
---

> Если control plane кластера размещен на виртуальных машинах или серверах bare-metal, то cloud-провайдер использует настройки модуля `cloud-provider-vsphere` в конфигурации Deckhouse. Иначе, если control plane кластера размещен в облаке, то cloud-провайдер использует структуру [VsphereClusterConfiguration](#vsphereclusterconfiguration) для настройки.
>
> Дополнительная информация о [Vsphere Cloud Load Balancers](https://github.com/kubernetes/cloud-provider-vsphere/tree/master/pkg/cloudprovider/vsphere/loadbalancer).

<!-- SCHEMA -->

