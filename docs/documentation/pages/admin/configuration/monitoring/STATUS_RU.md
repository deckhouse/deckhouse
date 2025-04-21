---
title: Доступность и SLA
permalink: ru/admin/configuration/monitoring/status.html
lang: ru
---

DKP может собирать статистику о доступности компонентов кластера и компонентам самого Deckhouse. Благодаря этим данным можно оценить степень выполнения SLA на компонентах, а также получить информацию о доступности в веб-интерфейсе.

Кроме того, с помощью кастомного ресурса [UpmeterRemoteWrite](/reference/cr/upmeterremotewrite/) можно экспортировать метрики доступности по протоколу Prometheus Remote Write.

Чтобы начать собирать метрики доступности и включить [интерфейс](#интерфейс), включите модуль `upmeter` в веб-интерфейсе (Deckhouse Console), или с помощью следующей команды:

```shell
d8 platform module enable upmeter
```

## Интерфейс

DKP предоставляет два веб-интерфейса для оценки доступности:
- Страница статуса.

  Получить адрес страницы можно в веб-интерфейсе на главной странице в разделе «Инструменты» (плитка «Статус-страница»), или выполнив команду:
  
  ```shell
  d8 k -n d8-upmeter get ing status -o jsonpath='{.spec.rules[*].host}'
  ``` 

  Пример веб-интерфейса страницы статуса:
  
  ![Пример веб-интерфейса страницы статуса](../../../images/upmeter/status.png)

- Страница доступности компонентов.

  Получить адрес страницы можно в веб-интерфейсе на главной странице в разделе «Инструменты» (плитка «Доступность компонентов»), или выполнив команду:
  
  ```shell
  d8 k -n d8-upmeter get ing upmeter -o jsonpath='{.spec.rules[*].host}'
  ``` 

  Пример страницы доступности компонентов:
  
  ![Пример графиков по метрикам из upmeter в Grafana](../../../images/upmeter/image1.png)

## Экспорт метрик статуса
 
Пример конфигурации UpmeterRemoteWrite для экспорта метрик статуса по протоколу [Prometheus Remote Write](https://docs.sysdig.com/en/docs/installation/prometheus-remote-write/):

```yaml
apiVersion: deckhouse.io/v1
kind: UpmeterRemoteWrite
metadata:
  labels:
    heritage: upmeter
    module: upmeter
  name: victoriametrics
spec:
  additionalLabels:
    cluster: cluster-name
    some: fun
  config:
    url: https://upmeter-victoriametrics.whatever/api/v1/write
    basicAuth:
      password: "Cdp#Cd.OxfZsx4*89SZ"
      username: upmeter
  intervalSeconds: 300
```
