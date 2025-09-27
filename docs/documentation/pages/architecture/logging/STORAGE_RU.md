---
title: Кратковременное хранение
permalink: ru/architecture/logging/storage.html
lang: ru
---

Deckhouse Kubernetes Platform (DKP) предоставляет встроенное решение для кратковременного хранения логов на базе проекта [Grafana Loki](https://grafana.com/oss/loki/).

Хранилище разворачивается в кластере и интегрируется с системой сбора логов.
После настройки ресурсов [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig), [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) и [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination)
логи автоматически поступают со всех системных компонентов.
Настроенное хранилище добавляется в Grafana в качестве источника данных для визуализации и анализа.

{% alert level="warning" %}
Кратковременное хранилище на базе Grafana Loki не поддерживает работу в режиме высокой доступности.
Для долговременного хранения важных логов используйте внешнее хранилище.
{% endalert %}
