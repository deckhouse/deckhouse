---
title: "Сетевая инфраструктура"
permalink: ru/admin/configuration/network/policy/
lang: ru
---

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/network_security_setup.html -->

Если на площадке, где работает Deckhouse Kubernetes Platform, есть требования для ограничения сетевого взаимодействия между серверами на уровне инфраструктуры, то необходимо соблюсти следующие условия:

* Включен режим туннелирования трафика между подами ([настройки](/modules/cni-cilium/configuration.html#parameters-tunnelmode) для CNI Cilium, [настройки](/modules/cni-flannel/configuration.html#parameters-podnetworkmode) для CNI Flannel).
* Разрешена передача трафика между [`podSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetcidr), инкапсулированного внутри VXLAN (если выполняется инспектирование и фильтрация трафика внутри VXLAN-туннеля).
* В случае необходимости интеграции с внешними системами (например, LDAP, SMTP или прочие внешние API), с ними разрешено сетевое взаимодействие.
* Локальное сетевое взаимодействие полностью разрешено в рамках каждого отдельно взятого узла кластера.
* Разрешено взаимодействие между узлами по портам, приведенным в таблицах на текущей странице. Обратите внимание, что большинство портов входит в диапазон 4200-4299. При добавлении новых компонентов платформы им будут назначаться порты из этого диапазона (при наличии возможности).

{% include network_security_setup.liquid %}
