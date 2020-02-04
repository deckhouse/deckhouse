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

| **Application** | **Grafana Dashboard** | **PrometheusRule** | **ServiceMonitor** |
| ------ |:------:|:------:|:------:|
| consul        |                    |                    | :white_check_mark: |
| elasticsearch | :white_check_mark: |                    | :white_check_mark: |
| etcd3         | :white_check_mark: |                    | :white_check_mark: |
| fluentd       |                    |                    | :white_check_mark: |
| jmx           | :white_check_mark: |                    |                    |
| memcached     | :white_check_mark: |                    | :white_check_mark: |
| minio         |                    |                    | :white_check_mark: |
| mongodb       | :white_check_mark: |                    | :white_check_mark: |
| nats          | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| nginx         |                    |                    | :white_check_mark: |
| php-fpm       | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| prometheus    | :white_check_mark: |                    | :white_check_mark: |
| rabbitmq      | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| redis         | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| sidekiq       | :white_check_mark: |                    | :white_check_mark: |
| trickster     |                    |                    | :white_check_mark: |
| uwsgi         | :white_check_mark: |                    | :white_check_mark: |
