---
title: Настройка сетевых политик для работы Deckhouse
permalink: ru/network_security_setup.html
lang: ru
---

Если в инфраструктуре, где работает Deckhouse, есть требования для ограничения сетевого взаимодействия, то необходимо соблюсти следующие условия:

* Включен [режим туннелирования](modules/021-cni-cilium/configuration.html#parameters-tunnelmode) трафика между подами.
* Разрешено взаимодействие между узлами по следующим портам:

{% include network_security_setup.liquid %}
