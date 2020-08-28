---
title: "Модуль extended-monitoring"
permalink: /modules/340-extended-monitoring/
---

Состоит из двух Prometheus exporter'ов:

- `extended-monitoring-exporter` — генерирует метрики и [алерты](configuration.html#non-namespaced-kubernetes-objects) по свободному месту и inode на нодах, плюс включает «расширенный мониторинг» объектов в указанных `namespace`.
- `image-availability-exporter` — генерирует метрики о проблемах доступа к Docker-образу в registry.

