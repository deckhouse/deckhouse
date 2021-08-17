---
title: "Модуль vertical-pod-autoscaler: настройки"
search: autoscaler
---

По умолчанию — **включен** в кластерах начиная с версии 1.11. В общем случае конфигурации не требуется.

VPA работает не с контроллером Pod'а, а с самим Pod'ом — измеряя и изменяя параметры его контейнеров. Вся настройка происходит с помощью Custom Resource'а [`VerticalPodAutoscaler`](cr.html#verticalpodautoscaler).

## Параметры

У модуля есть только настройки `nodeSelector/tolerations`.

<!-- SCHEMA -->
