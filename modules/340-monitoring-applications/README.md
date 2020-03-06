Модуль monitoring-applications
==============================

Модуль собирает из кластера информацию о работающих приложениях и настраивает для них:
* набор дашборд для Графаны (дашборды взяты из интернета, не выверены и не проверены),
* PrometheusRules для Прометея,
* ServiceMonitor для Прометея.

Активировать приложения можно явно с помощью [параметров](#параметры).

Параметры
---------
* `enabledApplications` — список приложений, который необходимо включить безотносительно результатов автоматического исследования.
  * Формат — список строк.
  * Поддерживаемые приложения:

| **Application** | **Grafana Dashboard** | **PrometheusRule** | **Sample Limit** |
| ------ |:------:|:------:|:------:|
| consul        |                    |                    | 500 |
| elasticsearch | :white_check_mark: |                    | 5000 |
| etcd3         | :white_check_mark: |                    | 1000 |
| fluentd       |                    |                    | 500 |
| memcached     | :white_check_mark: |                    | 2500 |
| minio         |                    |                    | 500 |
| mongodb       | :white_check_mark: |                    | 1000 |
| nats          | :white_check_mark: | :white_check_mark: | 500 |
| nginx         |                    |                    | 500 |
| php-fpm       | :white_check_mark: | :white_check_mark: | 1000 |
| prometheus    | :white_check_mark: |                    | 5000 |
| rabbitmq      | :white_check_mark: | :white_check_mark: | 2500 |
| redis         | :white_check_mark: | :white_check_mark: | 1000 |
| sidekiq       | :white_check_mark: |                    | 1000 |
| trickster     |                    |                    | 1000 |
| uwsgi         | :white_check_mark: |                    | 1000 |


### Как собирать метрики с приложения?

1. Необходимо поставить лейбл `prometheus.deckhouse.io/target` на Service, который вы хотите мониторить. В значении указать имя application из списка выше, на который ведет этот Service.
2. Указать порту, с которого необходимо собирать метрики, имя `http-metrics` и `https-metrics` для подключения по HTTP или HTTPS соответственно.
Если это не возможно, предлагается воспользоваться двумя аннотациями: `prometheus.deckhouse.io/port: номер_порта` для указания порта и `prometheus.deckhouse.io/tls: "true"`, если сбор метрик будет проходить по HTTPS.
3. Указать дополнительные аннотации для более тонкой настройки:
    * `prometheus.deckhouse.io/path` — путь для сбора метрик (по умолчанию: `/metrics`)
    * `prometheus.deckhouse.io/allow-unready-pod` — разрешает сбор метрик с подов в любом состоянии (по умолчанию метрики собираются только с подов в состоянии Ready).
    * `prometheus.deckhouse.io/sample-limit` — сколько семплов разрешено собирать с пода (значение лимита по умолчанию можно посмотреть в таблице выше).

Подробнее о том, как мониторить приложения, можно ознакомиться [здесь](../../docs/guides/MONITORING.md).
