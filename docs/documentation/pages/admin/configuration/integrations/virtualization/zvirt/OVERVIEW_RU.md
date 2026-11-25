---
title: Интеграция с zVirt
permalink: ru/admin/integrations/virtualization/zvirt/overview.html
lang: ru
---

Deckhouse Platform интегрируется с [zVirt](https://www.zvirt.ru/) и заказывает в нём узлы кластера. Параметры виртуальных машин задаются в ресурсе [ZvirtInstanceClass](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass), который указывается в группе узлов [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Возможности интеграции:

- Управление ресурсами zVirt, в том числе актуализация метаданных виртуальных машин и узлов кластера и удаление из кластера узлов, которых больше нет в zVirt.
- Заказ дисков через CSI-драйвер zVirt (`csi.ovirt.org`), чтобы из кластера можно было заказывать PersistentVolume через [StorageClass для доменов хранения zVirt](load-balancing-and-storage.html#storageclass-для-доменов-хранения).
- Заказ CloudPermanent-узлов с помощью [Terraform/OpenTofu-провайдера](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-zvirt.html#взаимодействия-модуля) `terraform-provider-ovirt`.
- Заказ CloudEphemeral-узлов через Cluster API (CAPI).
- Автоматическое включение CNI в новых кластерах. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/).
- клонирование виртуальных машин из заранее подготовленного шаблона на основе cloud-образа с `cloud-init`;
- автоматическое создание сетевого интерфейса виртуальной машины на основе профиля vNIC;
- получение сетевых настроек узлов по DHCP или [статическая настройка сети](layout.html#статическая-настройка-сети-узлов) для узлов CloudPermanent-групп;
