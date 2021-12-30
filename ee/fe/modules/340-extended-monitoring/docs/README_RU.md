---
title: "Модуль extended-monitoring"
---

Содержит следующие Prometheus exporter'ы:

- `extended-monitoring-exporter` — генерирует метрики и [алерты](configuration.html#non-namespaced-kubernetes-objects) по свободному месту и inode на нодах, плюс включает «расширенный мониторинг» объектов в указанных Namespace.
- `image-availability-exporter` — генерирует метрики о проблемах доступа к образу в container registry.
- `events-exporter` — собирает события в кластере Kubernetes и отдает их в виде метрик.
- `cert-exporter`— сканирует Secret'ы кластера Kubernetes и генерирует метрики об истечении срока действия сертификатов в них. 
