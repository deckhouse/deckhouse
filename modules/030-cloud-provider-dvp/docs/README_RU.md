---
title: "Cloud provider — DVP"
description: "Интеграция Deckhouse Kubernetes Platform с платформой виртуализации Deckhouse Virtualization Platform."
---

Модуль `cloud-provider-dvp` обеспечивает интеграцию Deckhouse Kubernetes Platform с [Deckhouse Virtualization Platform](https://deckhouse.ru/products/virtualization-platform/). Он позволяет модулю [node-manager](/modules/node-manager/) использовать ресурсы DVP при заказе узлов для [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-dvp`:

- Управление ресурсами DVP через `cloud-controller-manager`: актуализирует метаданные виртуальных машин и узлов Kubernetes и удаляет из Kubernetes узлы, которых больше нет в DVP.
- Заказ дисков через CSI-драйвер DVP (`csi.dvp.deckhouse.io`), чтобы из кластера можно было заказывать PersistentVolume.
- Заказ CloudEphemeral-узлов через Cluster API (CAPI). Параметры виртуальных машин задаются в ресурсе [DVPInstanceClass](cr.html#dvpinstanceclass).
- Регистрация в модуле [node-manager](/modules/node-manager/), чтобы `DVPInstanceClass` можно было указывать при описании `NodeGroup`.
- Автоматическое включение CNI для новых кластеров. По умолчанию используется [`cni-cilium`](/modules/cni-cilium/) в режиме `VXLAN`.

{% alert level="warning" %}
Если кластер был установлен со схемой `DVPClusterConfiguration`, необходима миграция на конфигурацию через ModuleConfig.
Пока миграция не выполнена, может срабатывать алерт `D8CloudProviderDVPMigrationPending`, а обновление Deckhouse — блокироваться.

Инструкция: [Как мигрировать облачный провайдер на конфигурацию через ModuleConfig](/products/kubernetes-platform/documentation/v1/faq.html#как-мигрировать-облачный-провайдер-на-конфигурацию-через-modulec).
{% endalert %}
