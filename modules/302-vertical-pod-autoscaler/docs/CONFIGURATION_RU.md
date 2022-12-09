---
title: "Модуль vertical-pod-autoscaler: настройки"
search: autoscaler
---

VPA работает не с контроллером Pod'а, а с самим Pod'ом — измеряя и изменяя параметры его контейнеров. Вся настройка происходит с помощью Custom Resource'а [`VerticalPodAutoscaler`](cr.html#verticalpodautoscaler).

В общем случае конфигурация модуля не требуется. У модуля есть только настройки `nodeSelector/tolerations`.

<!-- SCHEMA -->
