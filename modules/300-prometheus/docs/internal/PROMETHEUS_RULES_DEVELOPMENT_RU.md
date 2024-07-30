---
title: "Разработка правил Prometheus"
type:
- instruction
search: Разработка правил Prometheus, prometheus alerting rules
---

## Общая информация

* Правила в Prometheus делятся на два типа:
  * recording rules ([официальная документация](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)) — позволяют предрассчитать PromQL-выражение и сохранить результат в новую метрику (обычно это необходимо для ускорения работы Grafana или других правил).
  * alerting rules ([официальная документация](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/))— позволяют отправлять уведомления на основании результата выполнения PromQL выражения.
* Все правила распределены по модулям и лежат в каталоге [monitoring/prometheus-rules](https://github.com/deckhouse/deckhouse/tree/main/modules/300-prometheus/monitoring/prometheus-rules/)`. Правила делятся на три категории:
  * в `coreos` лежат правила, происходящие из репозитория prometheus-operator (местами сильно нами поправленные),
  * в `kubernetes` лежат наши правила, касаемые мониторинга самого kubernetes (самой платформы — control plane, NGINX Ingress, Prometheus, etc) и мониторинг "объектов" в kubernetes (Pod'ы, CronJob'ы, место на диске и пр.).
  * в `applications` лежат правила для мониторинга приложений (таких, как redis, mongo и пр.)
* Изменения этих файлов (в том числе и создание новых) должно автоматически показываться на странице `/prometheus/rules` (требуется подождать около минуты после деплоя deckhouse, пока отработает Prometheus Operator и компания).
* Если вы вносите изменение, а оно не показывается, путь диагностики следующий (подробнее см. в документации модуля [Prometheus Operator](../../modules/200-operator-prometheus/)):
  * Проверить, что ваши изменения попали в ConfigMap в Kubernetes:
    * `kubectl -n d8-monitoring get prometheusrule/prometheus-rules-<ИМЯ ДИРЕКТОРИИ> -o yaml`
    * Если изменений нет, то надо проверить, что deckhouse сдеплоилась успешно:
      * `helm -n d8-system ls` — prometheus должен быть в статусе DEPLOYED
      * `kubectl -n d8s-system logs deploy/deckhouse -f` — в логе не должно быть ошибок
  * Проверить, что prometheus-config-reloader увидел ваши изменения:
    * В `kubectl -n d8-monitoring logs prometheus-main-0 prometheus-config-reloader -f` должна быть запись об этом:

      ```text
      ts=2018-04-12T12:10:24Z caller=main.go:244 component=volume-watcher msg="ConfigMap modified."
      ts=2018-04-12T12:10:24Z caller=main.go:204 component=volume-watcher msg="Updating rule files..."
      ts=2018-04-12T12:10:24Z caller=main.go:209 component=volume-watcher msg="Rule files updated."
      ```

    * Если `prometheus-config-reloader` не видит изменений, то надо проверить prometheus-operator:
      * `kubectl -n d8-operator-prometheus get pod` — посмотреть, что Pod запущен
      * `kubectl -n d8-operator-prometheus logs -f deploy/prometheus-operator --tail=50` — посмотреть, что в логе нет ошибок
    * Если `prometheus-config-reloader` не может релоаднуть prometheus, значит в правилах ошибка и надо смотреть лог Prometheus:
      * `kubectl -n d8-monitoring logs prometheus-main-0 prometheus -f`
    * **Важно!** Бывает так, что `prometheus-config-reloader` "зависает" на какой-то ошибке и перестает видеть новые изменения, а продолжает пытаться релоаднуть Prometheus со старой ошибочной конфигурацией. В этом случае единственное, что можно сделать, — зайти в Pod и прибить процесс `prometheus-config-reloader` (Kubernetes перезапустит контейнер).

## Лучшие практики

### Называть группу правил согласно нашему стандарту

Правила в Prometheus разделяются на группы (см. любой файл с правилами для примера). Группу нужно обязательно называть согласно следующему формату: `<имя директории>.<имя файла без расширения>.<имя группы>`. При этом имя группы можно опустить. Например:
* в `kubernetes/nginx-ingress.yaml` есть три группы: `kubernetes.nginx-ingress.overview`, `kubernetes.nginx-ingress.details` и `kubernetes.nginx-ingress.controller`
* в `applications/redis.yaml` есть только одна группа: `applications.redis`.

### Всегда явно указывать job

Имя метрики, пусть даже оно вам кажется уникальным, может перестать быть таковым в любой момент — в одном из кластеров кто-то может добавить custom приложение, которое возвращает метрики с таким-же названием, и все — ваши правила поломаются. Однако мы очень четко контролируем лейбл job (благодаря servicemonitor'ам) и связка `название метрики` + `job` является гарантированно уникальной. Поэтому, **обязательно добавляйте имя job'а** во все запросы всех правил!

Например, метрика `nginx_filterzone_responses_total` является стандартной (ее экспортирует [nginx-vts-exporter](https://github.com/hnlq715/nginx-vts-exporter)), поэтому если не указать явно название job'а, то любое custom приложение экспортирующее эти метрики сломает все графики и все алерты ingress-nginx.

```text
sum(nginx_filterzone_responses_total{job="nginx-ingress-controller", server_zone="request_time_hist"}) by (job, namespace, scheme, server_name)
                                     ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
                                     гарантирует защиту от конфликта
```

### Использовать `irate(foo[1h])` при переводе counter в gauge

В некоторых ситуациях мы не можем использовать counter и нам необходима готовая метрика с gauge. Например, если мы хотим получить 90-ый перцентиль максимального трафика за последние три часа, а трафик у нас лежит в conter'е. В таком случае нам нужно сделать предрасчитанную метрику, в которой будет храниться gauge.

В Grafana, когда мы строим графики от counter'а, мы используем `rate[<scrape_interval> * 2]`, чтобы иметь данные с полной детализацией. И так как `scrape_interval` может быть разным в разных Prometheus'ах, в Grafana есть специальная переменная `$__interval_rv` (мы ее сами добавили в Grafana, но не суть), которая как раз и содержит двойной `scrape_interval`. Но как быть в Prometheus? Мы же не можем явно указывать 60s во всех правилах, так как они не будут работать в Prometheus'ах со `scrape_interval` большим чем 30s (ведь в range vector будет попадать меньше двух точек).

А очень просто! Нужно просто всегда использовать `irate(foo[1h])`. Дело в том, что:
* Правила выполняются каждый `evaluation_interval`,  но мы можем быть уверены, что он всегда равен `scrape_interval` в наших инсталляциях.
* `irate(foo[1h])` возвращает rate для последних двух точек (то есть не rate от первой до последней точки, а rate на основании двух последних точек).
* Мы можем быть уверены, что мы не будем использовать `scrape_interval` больше 30m (это слишком редко и не имеет никакого практического смысла).

Таким образом получается, что `irate(foo[1h])` в правилах Prometheus эквивалентно `rate[$__interval_rv]` в Grafana (см. подробнее "Объяснение деталей и причин" в разделе ["Точность данных и детализация"](grafana_dashboard_development.html#точность-данных-и-детализация) документации по разработке графиков Grafana).

### Не пытаться генерировать рулы Helm'ом

Prometheus rules — программируемая логика. Helm Chart template — программируемая логика. Использование одной программируемой логики для определения другой программируемой логики называется метапрограммированием и не стоит этим злоупотреблять не имея на то очень веских причин.

Например, если вы хотите в каком-то алерте сделать возможность переопределения порогового значения, то:
* Нужно сделать вот такой query: `foo_value > scalar(foo_threshold or vector(5))`.
  * если метрика `foo_threshold` определена, то `foo_value` будет сравниваться с ее значением,
  * если такой метрики нет — `foo_value` будет сравниваться с 5
* Тогда в том кластере, где это необходимо, можно сделать custom'ный рул, который будет возвращать метрику `foo_threshold` с необходимым пороговым значением.

## Alerting rules severity

Для указания критичности у alerting-правила используйте метку `severity_level` либо пару меток `impact` и `likehood`. Например:

```yaml
spec:
  groups:
  - name: custom.sentry.exporter-is-down
    rules:
    - alert: SentryExporterDown
      annotations:
        description: |-
          Prometheus не может достучаться до exporter-а метрик на протяжении 2 минут.
          Оповестите клиента команду и клиента.
        plk_markup_format: markdown
        plk_protocol_version: "1"
        summary: Не поступают метрики от Sentry
      expr: absent(up{job="custom-sentry"} == 1)
      for: 2m
      labels:
        severity_level: "1"
```

В таком случае в polk-e отобразится алерт с уровнем критичности S1.
Для достижения аналогичного разультата можно использовать `impact` и `likehood`:

```yaml
      labels:
        impact: deadly
        likehood: certain
```

Если инцидент можно описать негативными последствиями и вероятностью их наступления, то лучше применять второй способ — с метками `impact` и `likehood`. Например:
* Отказ одного диска массива RAID5 из четырех дисков, на котором лежит БД.
  * Выход из строя еще одного диска приведет к краху массива и потере данных, поэтому `impact: deadly`.
  * Скорее всего, второй диск не сломается сразу же, но такой исход возможен, поэтому `likehood: possible`.
  * Комбинация этих двух меток дает уровень S3.

Таблицу соответствия уровней и меток можно найти в Polk, нажав на значок критичности любого инцидента.
