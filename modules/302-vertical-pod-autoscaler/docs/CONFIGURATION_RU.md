---
title: "Модуль vertical-pod-autoscaler: настройки"
search: autoscaler
---

По умолчанию — **включен** в кластерах начиная с версии 1.11. В общем случае конфигурации не требуется.

VPA работает не с контроллером пода, а с самим подом — измеряя и изменяя параметры его контейнеров. Вся настройка происходит с помощью custom resource [`VerticalPodAutoscaler`](cr.html#verticalpodautoscaler).

## Параметры

У модуля есть только настройки `nodeSelector/tolerations`.

<!-- SCHEMA -->
