---
title: Обзор
permalink: ru/admin/integrations/virtualization/vmware/overview.html
lang: ru
---

В этом разделе рассматривается настройка интеграции Deckhouse Kubernetes Platform (DKP) с системами виртуализации на базе [VMware vSphere](https://www.vmware.com/products/cloud-infrastructure/vsphere).

Интеграция даёт возможность использовать ресурсы vSphere при заказе узлов для заданной [группы узлов](../../../configuration/platform-scaling/node-management.html#конфигурация-группы-узлов).

Основные возможности:

- Управление ресурсами vSphere:
  - создание сетевых маршрутов для сети `PodNetwork` на стороне vSphere;
  - создание LoadBalancer'ов для Service-объектов Kubernetes с типом `LoadBalancer`;
  - актуализация метаданных узлов кластера согласно описанным параметрам конфигурации и удаление из кластера узлов, которых уже нет в GCP.
- Заказ дисков в vSphere на datastore через механизм First-Class Disk (с помощью компонента `CSI storage`).
