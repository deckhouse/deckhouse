---
title: Модуль prometheus-metrics-adapter
search: autoscaler, HorizontalPodAutoscaler 
description: "Обеспечение работы горизонтального и вертикального масштабирования по любым метрикам в кластере Deckhouse Platform Certified Security Edition."
---

Позволяет работать HPA- и [VPA](../vertical-pod-autoscaler/)-автоскейлерам по «любым» метрикам.

Устанавливает в кластер имплементацию Kubernetes resource metrics API, custom metrics API и external metrics API для получения метрик из Prometheus.

Это позволяет:
- `kubectl top` брать метрики из Prometheus, через адаптер;
- использовать custom resource версии autoscaling/v2 для масштабирования приложений (HPA);
- получать информацию из Prometheus средствами API Kubernetes для других модулей (Vertical Pod Autoscaler, ...).

Модуль позволяет производить масштабирование по следующим параметрам:
* CPU (пода);
* память (пода);
* rps (Ingress'а) — за 1, 5, 15 минут (`rps_Nm`);
* CPU (пода) — за 1, 5, 15 минут (`cpu_Nm`) — среднее потребление CPU за N минут;
* память (пода) — за 1, 5, 15 минут (`memory_Nm`) — среднее потребление памяти за N минут;
* любые Prometheus-метрики и любые запросы на их основе.

## Как работает

Данный модуль регистрирует `k8s-prometheus-adapter` в качестве external API-сервиса, который расширяет возможности Kubernetes API. Когда какому-то из компонентов Kubernetes (VPA, HPA) требуется информация об используемых ресурсах, он делает запрос в Kubernetes API, а тот, в свою очередь, проксирует запрос в адаптер. Адаптер на основе своего конфигурационного файла выясняет, как посчитать метрику, и отправляет запрос в Prometheus.
