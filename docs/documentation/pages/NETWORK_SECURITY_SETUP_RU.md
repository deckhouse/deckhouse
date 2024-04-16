---
title: Настройка сетевых политик для работы Deckhouse
permalink: ru/network_security_setup.html
lang: ru
---

Если в инфраструктуре, где работает Deckhouse Kubernetes Pllatform, есть требования для ограничения сетевого взаимодействия, то необходимо соблюсти следующие условия:

* Включить [режим туннелирования](modules/021-cni-cilium/configuration.html#parameters-tunnelmode) трафика между подами.
* Разрешить взаимодействие между узлами по портам, приведенным в таблицах на текущей странице.

{% include network_security_setup.liquid %}
