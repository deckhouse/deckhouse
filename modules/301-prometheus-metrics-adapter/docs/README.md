---
title: Модуль prometheus-metrics-adapter
search: autoscaler, HorizontalPodAutoscaler 
---

Позволяет работать [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)- и [VPA](/modules/302-vertical-pod-autoscaler/)- автоскейлерам по «любым» метрикам.

Устанавливает в кластер [имплементацию](https://github.com/DirectXMan12/k8s-prometheus-adapter) Kubernetes [resource metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/resource-metrics-api.md), [custom metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/custom-metrics-api.md) и [external metrics API](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/instrumentation/external-metrics-api.md) для получения метрик из Prometheus.

Это позволяет:
- kubectl top брать метрики из Prometheus, через адаптер;
- использовать [autoscaling/v2beta2](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#metricspec-v2beta2-autoscaling) для скейлинга приложений (HPA);
- получать информацию из prometheus средствами API kubernetes для других модулей (Vertical Pod Autoscaler, ...).

Модуль позволяет производить скейлинг по следующим параметрам:
* cpu (pod'а),
* memory (pod'а),
* rps (ingress'а) - за 1,5,15 минут (`rps_Nm`),
* cpu (pod'а) - за 1,5,15 минут (`cpu_Nm`) - среднее потребления CPU за N минут,
* memory (pod'a) - за 1,5,15 минут (`memory_Nm`) - среднее потребление Memory за N минут,
* любые Prometheus-метрики и любые запросы на их основе.

## Как работает

Данный модуль регистрирует `k8s-prometheus-adapter` в качестве external API-сервиса, который расширяет возможности Kubernetes API. Когда какому-то из компонентов Kubernetes (VPA, HPA) требуется информация об используемых ресурсах, он делает запрос в Kubernetes API, а тот, в свою очередь, проксирует запрос в адаптер. Адаптер на основе своего [конфигурационного файла](https://github.com/deckhouse/deckhouse/blob/master/modules/301-prometheus-metrics-adapter/templates/config-map.yaml) выясняет, как посчитать метрику и отправляет запрос в Prometheus.

