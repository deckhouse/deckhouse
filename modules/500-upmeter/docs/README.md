---
title: "The upmeter module"
---

Модуль собирает статистику по типам доступности для компонентов кластера и Deckhouse. Позволяет оценивать степень выполнения SLA на эти компоненты, показывает данные о доступности в web-интерфейсе и предоставляет web-страницу статуса работы компонентов кластера.

С помощью Custom Resource [UpmeterRemoteWrite](cr.html#upmeterremotewrite) модуль можно настроить на передачу метрик по протоколу [Prometheus Remote Write](https://docs.sysdig.com/en/docs/installation/prometheus-remote-write/).

Состав модуля:
- **agent** — this program periodically performs probes and feeds their results to the aggregator. It runs on master nodes;
- **upmeter** — aggregates the results and implement the API server to retrieve them. Upmeter can link the history of probe results to the Downtime custom resource (where incidents are manually described);
- **front**
    - **status** — shows the current availability level over the previous 10 minutes (this one requires authorization by default, but you can disable it);
    - **web-ui** — displays the availability levels based on probes in time (requires authorization);
- **smoke-mini** — continuous *smoke testing* using a StatefulSet that looks like a real application.

Модуль отправляет примерно 100 sample'ов метрик каждые 5 минут, но это значение зависит от количества включенных модулей Deckhouse.

## Interface

Пример web-интерфейса:
![](../../images/500-upmeter/image1.png)

Пример графиков по метрикам из upmeter в Grafana:
![](../../images/500-upmeter/image2.png)
