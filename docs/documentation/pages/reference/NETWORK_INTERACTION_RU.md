---
title: Сетевое взаимодействие компонентов платформы
permalink: ru/reference/network_interaction.html
description: |
  Подробная информация о настройке сетевых политик для Deckhouse Kubernetes Platform, в частно в средах с ограничениями на сетевое взаимодействие между узлами. Описывает необходимые условия для включения режимов туннелирования для трафика подов с использованием CNI Cilium и Flannel.
lang: ru
search: network interaction, network policies, CNI configuration, Cilium, Flannel, network tunneling, сетевое взаимодействие, сетевые политики, конфигурация CNI, туннелирование сети
---

Если на площадке, где работает Deckhouse Kubernetes Platform (DKP), есть требования для ограничения сетевого взаимодействия между серверами на уровне инфраструктуры, то необходимо соблюсти следующие условия:

* Включен режим туннелирования трафика между подами ([настройки](/modules/cni-cilium/configuration.html#parameters-tunnelmode) для CNI Cilium, [настройки](/modules/cni-flannel/configuration.html#parameters-podnetworkmode) для CNI Flannel).
* Разрешена передача трафика между [podSubnetCIDR](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration), инкапсулированного внутри VXLAN (если выполняется инспектирование и фильтрация трафика внутри VXLAN-туннеля).
* В случае необходимости интеграции с внешними системами (например, LDAP, SMTP или прочие внешние API), с ними разрешено сетевое взаимодействие.
* Локальное сетевое взаимодействие полностью разрешено в рамках каждого отдельно взятого узла кластера.
* Разрешено взаимодействие между узлами по портам, приведенным в таблицах на текущей странице. Обратите внимание, что большинство портов входит в диапазон 4200-4299. При добавлении новых компонентов платформы им будут назначаться порты из этого диапазона (при наличии возможности).

{% offtopic title="Как проверить текущий порт VXLAN..." %}

```bash
d8 k -n d8-cni-cilium get cm cilium-config -o yaml | grep tunnel
```

Пример вывода команды:

```console
routing-mode: tunnel
tunnel-port: "4298"
tunnel-protocol: vxlan
```

{%- endofftopic %}

{% alert level="info" %}
Изменения, связанные с добавлением, удалением или переопределением портов в таблицах,
перечислены в подразделе «Сеть» соответствующей версии DKP [на странице «История изменений»](../release-notes.html).
{% endalert %}

{% include network_security_setup.liquid %}
