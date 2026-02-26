---
title: Обзор
permalink: ru/architecture/observability/
lang: ru
search: observability, наблюдаемость, подсистема наблюдаемости
---

В данном подразделе описывается архитектура подсистемы **Observability** (подсистемы наблюдаемости) DKP.

В подсистему **Observability** входят следующие модули:

* [prometheus](/modules/prometheus/) - разворачивает стек мониторинга с предустановленными параметрами для DKP и приложений, что упрощает начальную настройку,
* [operator-prometheus](/modules/operator-prometheus/) - устанавливает [Prometheus Operator](https://github.com/coreos/prometheus-operator), который автоматизирует развёртывание и управление инсталляциями [Prometheus](https://prometheus.io/).
* [prometheus-metrics-adapter](/modules/prometheus-metrics-adapter/) - позволяет HPA и VPA автоскейлерам использовать метрики мониторинга для принятия решений о масштабировании.
* [log-shipper](/modules/log-shipper/) - упрощает настройку сбора логов в Kubernetes.
* [loki](/modules/loki/) - разворачивает в кластере хранилище оперативных логов на базе [Grafana Loki](https://grafana.com/oss/loki/).
* [observability](/modules/observability/stable/) - расширяет функциональность модулей [prometheus](/modules/prometheus/) и [console](/modules/console/stable/), предоставляя дополнительные возможности для гибкого управления визуализацией метрик и разграничения доступа к ним.
* [extended-monitoring](/modules/extended-monitoring/) - расширяет возможности мониторинга кластера за счёт дополнительных Prometheus exporter’ов, которые позволяют выявлять потенциальные проблемы до того, как они скажутся на работе сервисов.
* [monitoring-custom](/modules/monitoring-custom/) - упрощает настройку мониторинга пользовательских приложений, требуя только указания определенного лейбла для нужного приложения.
* [monitoring-deckhouse](/modules/monitoring-deckhouse/) - мониторинг компонентов и сервисов DKP.
* [monitoring-kubernetes](/modules/monitoring-kubernetes/) - обеспечивает прозрачный и своевременный контроль состояния всех узлов кластера и ключевых инфраструктурных компонентов.
* [monitoring-kubernetes-control-plane](/modules/monitoring-kubernetes-control-plane/) - организует безопасный сбор метрик и предоставляет базовый набор правил мониторинга компонентов control plane кластера.
* [upmeter](/modules/upmeter/) - тестирует доступность платформы и состояние компонентов кластера в реальном времени, выводит информацию в виде соответствующих дашбордов.

В подразделе на данный момент описаны:

* [архитектура мониторинга в DKP](../monitoring/),
* [работа компонентов системы логирования](../logging/delivery.html)
* [модули логирования](logging/).
