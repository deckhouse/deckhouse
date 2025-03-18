---
title: "Locality failover с Istio"
permalink: ru/user/locality_failower_istio.html
lang: ru
---

В Deckhouse Kubernetes Platform вы можете реализовать механизм Locality failover средствами [istio](#).
Перед настройкой механизма убедитесь, что модуль включен в кластере.

Механизм Locality failover позволяет позволяет управлять маршрутизацией трафика и направлять его на приоритетный фейловер в случае недоступности определённых экземпляров сервисов.

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#locality-failover -->

> При необходимости ознакомьтесь с [основной документацией](https://istio.io/latest/docs/tasks/traffic-management/locality-load-balancing/failover/).

Istio позволяет настроить приоритетный географический фейловер между эндпоинтами. Для определения зоны Istio использует лейблы узлов с соответствующей иерархией:

* `topology.istio.io/subzone`;
* `topology.kubernetes.io/zone`;
* `topology.kubernetes.io/region`.

Это полезно для межкластерного фейловера при использовании совместно с [мультикластером](#устройство-мультикластера-из-двух-кластеров-с-помощью-ресурса-istiomulticluster).

> **Важно!** Для включения Locality Failover используется ресурс DestinationRule, в котором также необходимо настроить `outlierDetection`.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: helloworld
spec:
  host: helloworld
  trafficPolicy:
    loadBalancer:
      localityLbSetting:
        enabled: true # Включили LF.
    outlierDetection: # outlierDetection включить обязательно.
      consecutive5xxErrors: 1
      interval: 1s
      baseEjectionTime: 1m
```
