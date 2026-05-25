---
title: "Cloud provider — Yandex Cloud"
description: "Управление облачными ресурсами в Deckhouse Kubernetes Platform с помощью Yandex Cloud."
---

Взаимодействие с облачными ресурсами провайдера [Yandex Cloud](https://cloud.yandex.ru/) осуществляется с помощью модуля `cloud-provider-yandex`. Он предоставляет возможность модулю [управления узлами](/modules/node-manager/) использовать ресурсы Yandex Cloud при заказе узлов для описанной [группы узлов](/modules/node-manager/cr.html#nodegroup).

Функции модуля `cloud-provider-yandex`:

- Управление ресурсами Yandex Cloud с помощью модуля `cloud-controller-manager`:
  - Создание сетевых маршрутов для сети `PodNetwork` на стороне Yandex Cloud.
  - Актуализация метаданных Yandex Cloud Instances и Kubernetes Nodes. Удаление из Kubernetes узлов, которых уже нет в Yandex Cloud.
- Заказ дисков в Yandex Cloud с помощью компонента `CSI storage`.
- Регистрация в модуле [node-manager](/modules/node-manager/), чтобы [YandexInstanceClass'ы](cr.html#yandexinstanceclass) можно было использовать при описании [NodeGroup](/modules/node-manager/cr.html#nodegroup).
- Включение необходимого CNI (использует [`cni-cilium`](/modules/cni-cilium/)).

{% alert level="warning" %}
Начиная с DKP 1.77, для новых кластеров в Yandex Cloud по умолчанию используется CNI `cilium`. В существующих кластерах текущая конфигурация CNI сохраняется.

Для новых кластеров на всех узлах требуется ядро Linux версии 5.8 или новее. Также убедитесь, что правила межсетевого экрана разрешают межузловой UDP-трафик, необходимый для работы Cilium VXLAN.

Подробнее в разделах [«Требования к установке»](/products/kubernetes-platform/documentation/v1/installing/), [«Сетевое взаимодействие компонентов платформы»](/products/kubernetes-platform/documentation/v1/reference/network_interaction.html) и [документации модуля `cni-cilium`](/modules/cni-cilium/).
{% endalert %}
