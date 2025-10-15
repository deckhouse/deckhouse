---
title: "Locality failover с Istio"
permalink: ru/user/network/locality_failover_istio.html
lang: ru
---

В Deckhouse Kubernetes Platform можно реализовать механизм Locality failover средствами модуля [`istio`](/modules/istio/).
Перед настройкой механизма убедитесь, что модуль включен в кластере.

Механизм Locality failover управляет маршрутизацией трафика и направляет его на приоритетный фейловер в случае недоступности определённых экземпляров сервисов.

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/examples.html#locality-failover -->

{% alert level="info" %}
При необходимости ознакомьтесь с [документацией Locality failover](https://istio.io/latest/docs/tasks/traffic-management/locality-load-balancing/failover/).
{% endalert %}

С использованием Istio настраивается приоритетный географический фейловер между эндпоинтами. Для определения зоны применяются лейблы узлов с соответствующей иерархией:

* `topology.istio.io/subzone`;
* `topology.kubernetes.io/zone`;
* `topology.kubernetes.io/region`.

Это полезно для межкластерного фейловера при использовании совместно с мультикластером.

{% alert level="warning" %}
Для активации Locality Failover используется ресурс [DestinationRule](../network/managing_request_between_service_istio.html#ресурс-destinationrule), в котором также необходимо указать параметр `outlierDetection`.
{% endalert %}

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
        enabled: true # Включение LF.
    outlierDetection: # Обязательное включение outlierDetection.
      consecutive5xxErrors: 1
      interval: 1s
      baseEjectionTime: 1m
```
