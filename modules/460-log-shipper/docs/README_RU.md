---
title: "Модуль log-shipper"
---

Модуль для настройки log-pipeline на нодах с управлением через Custom Resources.

Log-pipeline позволяет доставлять логи из pod'ов в Loki/Elasticsearch/Logstash.

Модуль включен по умолчанию, но начинает чтение логов только если создан pipeline в виде связанных между собой [ClusterLoggingConfig](cr.html#clusterloggingconfig)/[PodLoggingConfig](cr.html#podloggingconfig) и [ClusterLogDestination](cr.html#clusterlogdestination).
