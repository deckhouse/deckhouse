---
title: "Модуль extended-monitoring"
---

Содержит следующие Prometheus exporter'ы:

- `extended-monitoring-exporter` — включает расширенный сбор метрик и отправку алертов по свободному месту и inode на узлах, плюс включает «расширенный мониторинг» объектов в namespace, у которых есть лейбл `extended-monitoring.deckhouse.io/enabled=""`;
- `image-availability-exporter` — добавляет метрики и включает отправку алертов, позволяющих узнать о проблемах с доступностью образа контейнера в registry, прописанному в поле `image` из spec пода в `Deployments`, `StatefulSets`, `DaemonSets`, `CronJobs`;
- `events-exporter` — собирает события в кластере Kubernetes и отдает их в виде метрик;
- `x509-certificate-exporter`— сканирует Secret'ы кластера Kubernetes и генерирует метрики об истечении срока действия сертификатов в них.
