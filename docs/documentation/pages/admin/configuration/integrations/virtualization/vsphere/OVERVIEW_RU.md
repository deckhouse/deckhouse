---
title: Интеграция с VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/overview.html
lang: ru
---

Deckhouse Platform интегрируется с [VMware vSphere](https://www.vmware.com/products/vsphere.html) и заказывает в нём узлы кластера. Параметры виртуальных машин задаются в ресурсе [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass), который указывается в группе узлов [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Возможности интеграции:

- Управление ресурсами vSphere:
  - создание сетевых маршрутов для сети `PodNetwork` на стороне vSphere;
  - актуализация метаданных виртуальных машин и узлов кластера, удаление из кластера узлов, которых больше нет в vSphere.
- Заказ дисков через CSI на datastore. По умолчанию используются CNS-тома с изменением размера на лету. Режим First-Class Disk (FCD) доступен как legacy и настраивается параметром [`compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag).
- Заказ базовой инфраструктуры и CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-vsphere.html#взаимодействия-модуля) `terraform-provider-vsphere`.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM) или Cluster API (CAPI).
- Автоматическое включение CNI в новых кластерах. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).

{% alert level="info" %}
Deckhouse Platform поддерживает гибридную интеграцию с VMware vSphere. Подробнее о настройке можно узнать в разделе [«Гибридный кластер с vSphere»](../../hybrid/vsphere-hybrid.html).
{% endalert %}
