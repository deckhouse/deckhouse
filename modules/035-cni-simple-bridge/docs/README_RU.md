---
title: "Модуль cni-simple-bridge"
description: "Обеспечение работы сети в кластере Deckhouse Kubernetes Platform ограниченной функциональности."
---

Модуль не имеет настроек.

Включается автоматически для следующих облачных провайдеров:

- [AWS](/modules/cloud-provider-aws/).
- [Azure](/modules/cloud-provider-azure/).
- [GCP](/modules/cloud-provider-gcp/).
- [Yandex](/modules/cloud-provider-yandex/).

{% alert level="info" %}
Начиная с DKP 1.77, для новых кластеров в AWS, Azure и GCP, а также с версии 1.76 для Yandex Cloud по умолчанию используется CNI `cilium`. В существующих кластерах текущая конфигурация CNI сохраняется.
{% endalert %}
