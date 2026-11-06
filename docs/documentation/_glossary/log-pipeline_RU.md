---
title: Pipeline логов
description: Цепочка сбора, фильтрации и доставки логов, которую модуль [`log-shipper`](/modules/log-shipper/) формирует из [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig), [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) и [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination).
lang: ru
---

ClusterLoggingConfig описывает источники логов на уровне кластера, PodLoggingConfig — источники в пределах одного неймспейса, а ClusterLogDestination задаёт параметры хранилища логов. По пути pipeline логи можно фильтровать по лейблам и содержимому сообщений, дополнять метаданными Kubernetes, [буферизировать](glossary.html#log-buffering) и отправлять сразу в несколько хранилищ, например в [кратковременное хранилище логов](glossary.html#short-term-log-storage) и в Elasticsearch.

Подробнее в разделе [«Сбор и доставка логов»](admin/configuration/logging/delivery.html).
