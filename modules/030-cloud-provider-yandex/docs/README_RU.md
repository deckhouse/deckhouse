---
title: "Cloud provider — Yandex Cloud"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Yandex Cloud."
---

Взаимодействие с облачными ресурсами провайдера [Yandex Cloud](https://cloud.yandex.ru/) осуществляется с помощью модуля `cloud-provider-yandex`. Он предоставляет возможность модулю [управления узлами](../../modules/node-manager/) использовать ресурсы Yandex Cloud при заказе узлов для описанной [группы узлов](../../modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-yandex`:

- Управление ресурсами Yandex Cloud с помощью модуля `cloud-controller-manager`:
  - Создание сетевых маршрутов для сети `PodNetwork` на стороне Yandex Cloud.
  - Актуализация метаданных Yandex Cloud Instances и Kubernetes Nodes. Удаление из Kubernetes узлов, которых уже нет в Yandex Cloud.
- Заказ дисков в Yandex Cloud с помощью компонента `CSI storage`.
- Регистрация в модуле [node-manager](../../modules/node-manager/), чтобы [YandexInstanceClass'ы](cr.html#yandexinstanceclass) можно было использовать при описании [NodeGroup](../../modules/node-manager/cr.html#nodegroup).
- Включение необходимого CNI (использует [`cni-cilium`](../../modules/cni-cilium/)).

{% alert level="warning" %}
Начиная с версии DKP 1.76, в Yandex Cloud CNI `cilium` используется по умолчанию для новых кластеров. В существующих кластерах текущая конфигурация CNI сохраняется.

Для новых кластеров требуется ядро Linux версии `5.8` или новее на всех узлах. Также убедитесь, что файрволы и группы безопасности разрешают межузловой UDP-трафик для Cilium VXLAN. Подробнее см. [требования к установке](/products/kubernetes-platform/documentation/v1/installing/), [раздел «Сетевое взаимодействие компонентов платформы»](/products/kubernetes-platform/documentation/v1/reference/network_interaction.html) и [документацию модуля `cni-cilium`](/modules/cni-cilium/).
{% endalert %}
