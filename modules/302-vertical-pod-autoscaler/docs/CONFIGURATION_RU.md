---
title: "Модуль vertical-pod-autoscaler: настройки"
search: autoscaler
---

VPA работает не с контроллером пода, а с самим подом, измеряя и изменяя параметры его контейнеров. Вся настройка происходит с помощью custom resource'а [`VerticalPodAutoscaler`](cr.html#verticalpodautoscaler).

В общем случае конфигурация модуля не требуется. У модуля есть только настройки `nodeSelector/tolerations`.

<!-- SCHEMA -->
