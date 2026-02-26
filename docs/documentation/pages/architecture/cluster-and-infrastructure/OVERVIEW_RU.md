---
title: Подсистема Cluster & Infrastructure
permalink: ru/architecture/cluster-and-infrastructure/
lang: ru
search: cluster & infrastructure, управление узлами
description: Архитектура подсистемы Cluster & Infrastructure в Deckhouse Kubernetes Platform.
---

Данный раздел посвящён архитектуре подсистемы Cluster & Infrastructure платформы Deckhouse Kubernetes Platform (DKP).

Подсистема Cluster & Infrastructure отвечает за инфраструктурную часть управления Kubernetes-кластером. Управление узлами кластера реализовано с помощью модуля [`node-manager`](/modules/node-manager/), а взаимодействие с IaaS-провайдерами — через соответствующие модули семейства `cloud-provider-`.

В разделе описаны механизмы управления всеми используемыми в DKP типами узлов, а также [гибридными группами узлов и кластерами](hybrid-nodegroups-and-clusters/).

В подсистему Cluster & Infrastructure также входят следующие модули:
<!--- TODO: дописать про интеграции со всеми поддерживаемыми облачными провайдерами, как будет готово. --->

* [`chrony`](/modules/chrony/) — обеспечивает синхронизацию времени на всех узлах кластера;
* [`registry-packages-proxy`](/modules/registry-packages-proxy/) — предоставляет внутренний прокси-сервер для пакетов хранилища образов контейнеров;
* [`terraform-manager`](/modules/terraform-manager/) — предоставляет инструменты для работы с состоянием Terraform в Kubernetes-кластере.

В подразделе также описана служба [Bashible](bashible/), которая является ключевым компонентом подсистемы Cluster & Infrastructure. Bashible используется модулем [`node-manager`](/modules/node-manager/) для управления конфигурацией узлов.
