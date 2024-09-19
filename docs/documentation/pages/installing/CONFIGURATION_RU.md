---
title: "Установка: настройки"
permalink: ru/installing/configuration.html
lang: ru
---

Описание ресурсов, используемых при [установке Deckhouse](./).

{% alert level="warning" %}Внимание!{% endalert %}

Любые измения параметров `internalNetworkCIDRs`, `serviceSubnetCIDR`, `podSubnetNodeCIDRPrefix`, `podSubnetCIDR` в работающем кластере не рекомендуются, так как приводят к декструктивным изменениям кластера. Для их изменения, рекомендуется создание нового кластера с нуля.

Для восстановления работоспособности кластера после изменения, требуюется ручное восстновление и изменение настроек etcd, перевыпуск всех сертификатов и перезапуск всех подов, что приводит к длительному простою, и может привести к потерям данных.

{{ site.data.schemas.global.cluster_configuration | format_cluster_configuration }}

{{ site.data.schemas.global.init_configuration | format_cluster_configuration }}

{{ site.data.schemas.global.static_cluster_configuration | format_cluster_configuration }}
