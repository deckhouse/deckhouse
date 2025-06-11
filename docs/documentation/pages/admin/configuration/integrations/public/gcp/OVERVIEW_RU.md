---
title: Обзор
permalink: ru/admin/integrations/public/gcp/overview.html
lang: ru
---

В этом разделе рассматривается настройка интеграции Deckhouse Kubernetes Platform (DKP) с облачными ресурсами провайдера [Google Cloud Platform](https://cloud.google.com/) (GCP).

Интеграция даёт возможность использовать ресурсы GCP при заказе узлов для заданной [группы узлов](../../../configuration/platform-scaling/node-management.html#конфигурация-группы-узлов).

Основные возможности:

- Управление ресурсами GCP:
  - создание сетевых маршрутов для сети `PodNetwork` на стороне GCP;
  - создание LoadBalancer'ов для Service-объектов Kubernetes с типом `LoadBalancer`;
  - актуализация метаданных узлов кластера согласно описанным параметрам конфигурации и удаление из кластера узлов, которых уже нет в GCP.
- Заказ дисков в GCP (с помощью компонента `CSI storage`).
- Включение необходимого CNI (используется [simple bridge](../../../../reference/mc/cni-simple-bridge/)).
