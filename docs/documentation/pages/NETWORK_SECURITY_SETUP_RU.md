---
title: Настройка сетевых политик для работы Deckhouse
permalink: ru/network_security_setup.html
lang: ru
---

Если в инфраструктуре, где работает Deckhouse Kubernetes Platform, есть требования для ограничения сетевого взаимодействия, то необходимо соблюсти следующие условия:

* Включен режим туннелирования трафика между подами ([настройки](modules/021-cni-cilium/configuration.html#parameters-tunnelmode) для CNI Cilium, [настройки](modules/035-cni-flannel/configuration.html#parameters-podnetworkmode) для CNI Flannel).
* В случае необходимости интеграции с внешними системами (например, LDAP, SMTP или прочие внешние API), с ними разрешено сетевое взаимодействие.
* Локальное сетевое взаимодействие полностью разрешено в рамках каждого отдельно взятого узла кластера.
* Разрешено взаимодействие между узлами по портам, приведенным в таблицах на текущей странице.

{% include network_security_setup.liquid %}
