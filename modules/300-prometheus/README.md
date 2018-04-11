Модуль prometheus
=======

Модуль устанавливает [prometheus](https://prometheus.io/) (используя модуль [prometheus-operator](../200-prometheus-operator/)) и полностью его настраивает!

Дополнительная информация
-------------------------

* [Интеграция с Madison](docs/MADISON.md)
* [Разработка правил и графиков](docs/DEVELOPMENT.md)

Конфигурация
------------

### Что нужно настраивать?

При установке **нужно настроить два параметра**:
```yaml
prometheus:
  retentionDays: 15
  estimatedNumberOfMetrics: 250000
```

### Параметры

* `retentionDays` — сколько дней хранить данные.
    * По-умолчанию `7`.
    * **Важно!!!** При изменении этого параметра перезаказывается диск (при этом удаляются все данные).
* `estimatedNumberOfMetrics` — примерное количество метрик, которые плинируется хранить в prometheus (на основании этого параметра рассчитывается размер диска и размер памяти для prometheus'а)
    * По-умолчанию `200000`.
    * Примерные значения (в зависимости от количества узлов и подов):
        * 1 узел, 37 подов (someproject) — 22 000
        * 6 узлов, 72 пода (someproject) — 49 000
        * 10 узлов, 310 подов (someproject) — 333 000
        * 22 узла, 570  подов (someproject.prod) — 400 000
    * Для рассчета правильного значения нужно открыть prometheus, выполнить запрос `count(max_over_time({__name__=~".+"}[1h]))`, после чего добавить запас "на глаз" (обычно 20-50%). Если prometheus открывать не удобно, то вот готовый скрипт: `curl -s 'http://'$(kubectl -n kube-prometheus get pod/prometheus-main-0 -o json | jq '.status.podIP' -r)':9090/api/v1/query?query=count(max_over_time(%7B__name__%3D~%22.%2B%22%7D%5B1h%5D))' | jq '.data.result[0].value[1]' -r`
    * Значения для метрик собираются каждые 30 секунд, каждая метрика занимает примерно два байта, соотвественно объем данных за сутки рассчитывается по следующей формуле: `estimatedNumberOfMetrics / 30 секунд * 3600 * 24 * 2 байта / 1024 / 1024`.
    * Диск заказывается с запасом 30% так, чтобы хватило на `retentionDays` (с округлением до целого количества гигабайт в большую сторону).
    * Требования по памяти для пода (resources request) рассчтиваются таким образом, чтобы в память помещался двухдневный объем.
    * Ограничение по используемой память у prometheus 2.0+ явным образом никак не настраивается (он использует самый минимум и активно использует дисковый кеш).
    * **Важно!!!** При изменении этого параметра перезаказывается диск (при этом удаляются все данные).
* `storageClassName` — имя storageClass'а, который использовать.
    * Если не указано — используется или `global.storageClassName` или `global.cluster.defaultStorageClassName`, а если и они не указаны — данные сохраняются в emptyDir.
    * Если указать `false` — будет форсироваться использование emptyDir'а.
* `userPassword` — пароль пользователя `user` (генерируется автоматически, но можно изменять).
* `adminPassword` — пароль пользователя `admin` (генерируется автоматически, но можно изменять).
* `madisonAuthKey` — ключ для отправки алертов в Madison ([подробнее об интеграции с Madison](docs/MADISON.md)).
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет использовано значение `[{"key":"node-role/system","operator":"Exists"}]` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.

### Пример конфига

```yaml
prometheus:
  userPassword: xxxxxx
  adminPassword: yyyyyy
  retentionDays: 7
  estimatedNumberOfMetrics: 200000
  storageClassName: rbd
  nodeSelector:
    node-role/monitoring: ""
  tolerations:
  - key: node-role/monitoring
    operator: Exists
```
