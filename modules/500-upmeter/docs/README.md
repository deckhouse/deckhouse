---
title: "The upmeter module"
---

The module collects statistics by availability type for cluster components and Deckhouse. It enables evaluating the degree of SLA compliance for these components, presents availability data via a web interface, and provides a web page with the operating statuses of the cluster components.

You can export availability metrics over the [Prometheus Remote Write](https://docs.sysdig.com/en/docs/installation/prometheus-remote-write/) protocol using the [UpmeterRemoteWrite](cr.html#upmeterremotewrite) Custom Resource.

Module composition:
- **agent** — probes the availability of components and sends the results to the server; runs on the master nodes;
- **upmeter** — aggregates the results and implements the API server to retrieve them;
- **front**
  - **status** — показывает текущий уровень доступности за последние 10 минут (по умолчанию требует авторизации, но её можно отключить).
  - **webui** — дашборд со статистикой по пробам и группам доступности (требует авторизации).
- **smoke-mini** — постоянное *smoke-тестирование* с помощью StatefulSet, похожего на настоящее приложение.

Модуль отправляет около 100 показаний метрик каждые 5 минут. Это значение зависит от количества включенных модулей Deckhouse.

## Interface

Пример web-интерфейса:
![](../../images/500-upmeter/image1.png)

Пример графиков по метрикам из upmeter в Grafana:
![](../../images/500-upmeter/image2.png)
