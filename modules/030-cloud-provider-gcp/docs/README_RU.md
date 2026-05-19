---
title: "Cloud provider — GCP"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Google Cloud Platform."
---

Взаимодействие с облачными ресурсами провайдера [Google](https://cloud.google.com/) осуществляется с помощью модуля `cloud-provider-gcp`. Он предоставляет возможность модулю [управления узлами](../../modules/node-manager/) использовать ресурсы GCP при заказе узлов для описанной [группы узлов](../../modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-gcp`:

- Управление ресурсами GCP с помощью модуля `cloud-controller-manager`:
  - Создание сетевых маршрутов для сети `PodNetwork` на стороне GCP.
  - Создание LoadBalancer'ов для Service-объектов Kubernetes с типом `LoadBalancer`.
  - Актуализация метаданных узлов кластера согласно описанным параметрам конфигурации и удаление из кластера узлов, которых уже нет в GCP.
- Заказ дисков в GCP с помощью компонента `CSI storage`.
- Включение необходимого CNI (использует [`cni-cilium`](../../modules/cni-cilium/)).
- Регистрация в модуле [node-manager](../../modules/node-manager/) для использования [GCPInstanceClass'ов](cr.html#gcpinstanceclass) при описании [NodeGroup](../../modules/node-manager/cr.html#nodegroup).

{% alert level="warning" %}
Начиная с версии DKP 1.77, в GCP CNI `cilium` используется по умолчанию для новых кластеров. В существующих кластерах текущая конфигурация CNI сохраняется.

Для новых кластеров требуется ядро Linux версии `5.8` или новее на всех узлах. Также убедитесь, что правила межсетевого экрана и группы безопасности разрешают межузловой UDP-трафик для Cilium VXLAN. Подробнее см. [требования к установке](/products/kubernetes-platform/documentation/v1/installing/), [раздел «Сетевое взаимодействие компонентов платформы»](/products/kubernetes-platform/documentation/v1/reference/network_interaction.html) и [документацию модуля `cni-cilium`](/modules/cni-cilium/).
{% endalert %}
