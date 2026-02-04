---
title: Обзор
permalink: ru/architecture/cluster-and-infrastructure/
lang: ru
search: cluster & infrastructure, узел, нода, управление узлами, управление нодами
---

Данный раздел посвящён архитектуре подсистемы **Cluster & Infrastructure** (подсистема кластер и инфраструктура) DKP.

Подсистема **Cluster & Infrastructure** отвечает за инфраструктурную часть управления кластером Kubernetes. В частности, механика управления узлами кластера реализована при помощи модуля [node-manager](/modules/node-manager/), взаимодействие с IaaS облачных провайдеров - соответствующими модулями **cloud-provider-**. В разделе описаны варианты управления всеми используемыми в DKP типами узлов, [гибридными группами узлов и кластерами](hybrid-nodegroups-and-clusters/). В подсистему **Cluster & Infrastructure** входят также следующие модули:
<!--- TODO: дописать про интеграции со всеми поддерживаемыми облачными провайдерами, как будет готово. --->

* [chrony](/modules/chrony/) - обеспечивает синхронизацию времени на всех узлах кластера,
* [registry-packages-proxy](/modules/registry-packages-proxy/) - внутренний прокси-сервер пакетов registry,
* [terraform-manager](/modules/terraform-manager/) - предоставляет инструменты для работы с состоянием Terraform’а в кластере Kubernetes.

В подразделе также описана служба [Bashible](bashible/). **Bashible** - ключевой компонент подсистемы **Cluster & Infrastructure**, на котором завязана работа модуля [node-manager](/modules/node-manager/).
