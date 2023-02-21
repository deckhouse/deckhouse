---
title: "Модуль extended-monitoring"
---

Содержит следующие Prometheus exporter'ы:

- `extended-monitoring-exporter` — включает расширенный сбор метрик и отправку [алертов](configuration.html#non-namespaced-kubernetes-objects) по свободному месту и inode на узлах, плюс включает «расширенный мониторинг» объектов в Namespace, у которых есть лейбл `extended-monitoring.deckhouse.io/enabled=””`.
- `image-availability-exporter` — добавляет метрики и включает отправку алертов, позволяющих узнать о проблемах с доступностью образа контейнера в registry, прописанному в поле `image` из spec Pod’а в `Deployments`, `StatefulSets`, `DaemonSets`, `CronJobs`.
- `events-exporter` — собирает события в кластере Kubernetes и отдает их в виде метрик.
- `cert-exporter`— сканирует Secret'ы кластера Kubernetes и генерирует метрики об истечении срока действия сертификатов в них.
