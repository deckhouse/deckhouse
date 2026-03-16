---
title: Подсистема Cluster & Infrastructure
permalink: ru/architecture/cluster-and-infrastructure/
lang: ru
search: cluster & infrastructure, управление узлами
description: Архитектура подсистемы Cluster & Infrastructure в Deckhouse Kubernetes Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

Данный раздел посвящён архитектуре подсистемы Cluster & Infrastructure платформы Deckhouse Kubernetes Platform (DKP).

Подсистема Cluster & Infrastructure отвечает за инфраструктурную часть управления Kubernetes-кластером. Управление узлами кластера реализовано с помощью модуля [`node-manager`](/modules/node-manager/), а взаимодействие с IaaS-провайдерами — через соответствующие модули семейства `cloud-provider-`.

В разделе описаны:

* Механизмы управления всеми используемыми в DKP типами узлов, а также [гибридными группами узлов и кластерами](hybrid-nodegroups-and-clusters.html).
* Типовая архитектура [CSI-драйвера](csi-driver.html), используемая в DKP.
* Служба [Bashible](bashible.html), которая является ключевым компонентом подсистемы Cluster & Infrastructure. Bashible используется модулем [`node-manager`](/modules/node-manager/) для управления конфигурацией узлов.

В подсистему Cluster & Infrastructure также входят следующие модули:

* [`chrony`](/modules/chrony/) — обеспечивает синхронизацию времени на всех узлах кластера;
* [`registry-packages-proxy`](/modules/registry-packages-proxy/) — предоставляет внутренний прокси-сервер для пакетов хранилища образов контейнеров;
* [`terraform-manager`](/modules/terraform-manager/) — предоставляет инструменты для работы с состоянием Terraform в Kubernetes-кластере;
* модули поддерживаемых DKP облачных провайдеров:

  * [`cloud-provider-aws`](/modules/cloud-provider-aws/);
  * [`cloud-provider-azure`](/modules/cloud-provider-azure/);
  * [`cloud-provider-dvp`](/modules/cloud-provider-dvp/);
  * [`cloud-provider-dynamix`](/modules/cloud-provider-dynamix/);
  * [`cloud-provider-gcp`](/modules/cloud-provider-gcp/);
  * [`cloud-provider-huaweicloud`](/modules/cloud-provider-huaweicloud/);
  * [`cloud-provider-openstack`](/modules/cloud-provider-openstack/);
  * [`cloud-provider-vcd`](/modules/cloud-provider-vcd/);
  * [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/);
  * [`cloud-provider-yandex`](/modules/cloud-provider-yandex/);
  * [`cloud-provider-zvirt`](/modules/cloud-provider-zvirt/).

В подразделе на данный момент описан только модуль [`cloud-provider-dvp`](/modules/cloud-provider-dvp/).
Материалы по модулям остальных облачных провайдеров будут добавляться по мере готовности.
