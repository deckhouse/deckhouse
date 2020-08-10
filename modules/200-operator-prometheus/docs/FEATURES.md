---
title: "Управление оператором Prometheus"
---

Deckhouse может устанавливать, управлять ресурсами и процессом обновления [prometheus-operator](https://github.com/coreos/prometheus-operator) с помощью модуля [operator-prometheus]({{site.baseurl}}/modules/200-operator-prometheus/).

Данный оператор позволяет создавать и автоматизированно управлять инсталляциями [Prometheus](https://prometheus.io/), в том числе, с его помощью Deckhouse устанавливает Prometheus в кластер.

В общем случае модуль не требует настройки, он просто устанавливает *prometheus-operator*, который:
- с помощью механизма `CRD` (`Custom Resource Definitions`) позволяет определить следующие ресурсы:
  - `Prometheus` — определяет инсталляцию (кластер) *Prometheus*
  - `ServiceMonitor` — определяет, как собирать метрики с сервисов
  - `Alertmanager` — определяет кластер *Alertmanager*'ов
  - `PrometheusRule` — определяет список *Prometheus rules*
- cледит за этими ресурсами и:
  - генерирует `StatefulSet` с самим *Prometheus* и необходимые для его работы конфигурационные файлы, сохраняя их в `Secret`;
  - cледит за ресурсами `ServiceMonitor` и `PrometheusRule` и на их основании обновляет конфигурационные файлы *Prometheus* через внесение изменений в `Secret`.

Модуль имеет необязательные параметры — `nodeSelector` и `toleration`, которые предоставляют возможность запустить оператор на конкретных узлах;

Подробнее про внутреннее устройство можно узнать [в документации]({{site.baseurl}}/modules/200-operator-prometheus/internals.html).
