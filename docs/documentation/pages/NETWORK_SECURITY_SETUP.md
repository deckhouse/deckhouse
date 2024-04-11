---
title: Настройка сетевых политик для работы Deckhouse
permalink: ru/network_security_setup.html
lang: ru
---

If the infrastructure where Deckhouse is running has requirements to limit network communication, the following conditions must be met:

* [Tunneling mode](modules/021-cni-cilium/configuration.html#parameters-tunnelmode) for traffic between pods is enabled.
* Inter-node communication is allowed on the following ports:

{% include network_security_setup.liquid %}
