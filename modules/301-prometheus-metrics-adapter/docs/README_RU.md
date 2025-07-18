---
title: Модуль prometheus-metrics-adapter
search: autoscaler, HorizontalPodAutoscaler 
description: "Обеспечение работы горизонтального и вертикального масштабирования по любым метрикам в кластере Deckhouse Kubernetes Platform."
---

Позволяет работать [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)- и [VPA](../vertical-pod-autoscaler/)-автоскейлерам по «любым» метрикам.

Устанавливает в кластер [имплементацию](https://github.com/kubernetes-sigs/prometheus-adapter) Kubernetes [resource metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/resource-metrics-api.md), [custom metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/custom-metrics-api.md) и [external metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/external-metrics-api.md) для получения метрик из Prometheus.

Это позволяет:
- `kubectl top` брать метрики из Prometheus, через адаптер;
- использовать custom resource версии [autoscaling/v2](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.23/#objectmetricsource-v2-autoscaling) для масштабирования приложений (HPA);
- получать информацию из Prometheus средствами API Kubernetes для других модулей (Vertical Pod Autoscaler, ...).

Модуль позволяет производить масштабирование по следующим параметрам:
* CPU (пода);
* память (пода);
* rps (Ingress'а) — за 1, 5, 15 минут (`rps_Nm`);
* CPU (пода) — за 1, 5, 15 минут (`cpu_Nm`) — среднее потребление CPU за N минут;
* память (пода) — за 1, 5, 15 минут (`memory_Nm`) — среднее потребление памяти за N минут;
* любые Prometheus-метрики и любые запросы на их основе.

## Как работает

Данный модуль регистрирует `k8s-prometheus-adapter` в качестве external API-сервиса, который расширяет возможности Kubernetes API. Когда какому-то из компонентов Kubernetes (VPA, HPA) требуется информация об используемых ресурсах, он делает запрос в Kubernetes API, а тот, в свою очередь, проксирует запрос в адаптер. Адаптер на основе своего [конфигурационного файла](https://github.com/deckhouse/deckhouse/blob/main/modules/301-prometheus-metrics-adapter/templates/config-map.yaml) выясняет, как посчитать метрику, и отправляет запрос в Prometheus.
