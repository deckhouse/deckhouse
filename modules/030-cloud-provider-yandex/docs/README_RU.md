---
title: "Cloud provider — Yandex Cloud"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Yandex Cloud."
---

Модуль `cloud-provider-yandex` обеспечивает интеграцию Deckhouse Kubernetes Platform с [Yandex Cloud](https://cloud.yandex.ru/). Он позволяет модулю [`node-manager`](/modules/node-manager/) использовать ресурсы Yandex Cloud при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-yandex`:

- Управление ресурсами Yandex Cloud через `cloud-controller-manager`:
  - создаёт сетевые маршруты для сети `PodNetwork` на стороне Yandex Cloud;
  - создаёт Network Load Balancer и целевые группы для Service с типом LoadBalancer;
  - актуализирует метаданные инстансов и узлов Kubernetes и удаляет из Kubernetes узлы, которых больше нет в Yandex Cloud.
- Заказ дисков через CSI-драйвер Yandex (`yandex.csi.flant.com`) и создание StorageClass для типов дисков Yandex Cloud, чтобы из кластера можно было заказывать PersistentVolume.
- Заказ CloudEphemeral-узлов через Machine Controller Manager (MCM) или Cluster API (CAPI). Параметры виртуальных машин задаются в ресурсе [YandexInstanceClass](cr.html#yandexinstanceclass).
- Регистрация в модуле [`node-manager`](/modules/node-manager/), чтобы YandexInstanceClass можно было указывать при описании NodeGroup.
- Автоматическое включение CNI для новых кластеров. Начиная с DKP 1.76 по умолчанию используется [`cni-cilium`](/modules/cni-cilium/) в режиме `VXLAN` с трансляцией исходных IP-адресов средствами [BPF](/products/kubernetes-platform/documentation/v1/admin/configuration/network/other/bpflb.html).

{% alert level="warning" %}
Модуль находится в процессе миграции управления CloudEphemeral-узлами с Machine Controller Manager (MCM) на Cluster API (CAPI). Существующие NodeGroup продолжают использовать MCM, а новые по умолчанию создаются с использованием CAPI. Порядок миграции существующих групп — в разделе [«Как мигрировать группы узлов на Cluster API (CAPI)»](/products/kubernetes-platform/documentation/v1/faq.html#как-мигрировать-группы-узлов-на-cluster-api-capi).
{% endalert %}
