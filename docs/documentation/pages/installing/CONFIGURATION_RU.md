---
title: "Установка: настройки"
permalink: ru/installing/configuration.html
lang: ru
---

Описание ресурсов, используемых при [установке Deckhouse](./).

{% alert level="danger" %}
Не изменяйте параметры `serviceSubnetCIDR`, `podSubnetNodeCIDRPrefix`, `podSubnetCIDR` в работающем кластере. Если изменение параметров необходимо — разверните новый кластер.
{% endalert %}

{{ site.data.schemas.global.cluster_configuration | format_cluster_configuration }}

{{ site.data.schemas.global.init_configuration | format_cluster_configuration }}

{{ site.data.schemas.global.static_cluster_configuration | format_cluster_configuration }}
