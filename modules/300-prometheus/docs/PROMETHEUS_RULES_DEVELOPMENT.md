Разработка правил Prometheus
============================

Общая информация
----------------

* Правила в Prometheus делятся на два типа:
    * recording rules ([официальная документация](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)) — позволяют предрасчитать PromQL выражение и сохранить результат в новую метрику (обычно это необходимо для ускорения работы Grafana или других правил).
    * alerting rules ([официальная документация](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/))— позволяют отправлять уведомления на основании результата выполнения PromQL выражения.
* Все правила у нас лежат в трех директориях в [prometheus-rules](../prometheus-rules/):
    * в [coreos](../prometheus-rules/coreos/) лежат правила, происходящие из репозитория prometheus-operator (местами сильно нами поправленные),
    * в [kubernetes](../prometheus-rules/kubernetes/) лежат наши правила, касаемые мониторинга самого kubernetes (самой платформы — control plane, nginx ingress, prometheus, etc) и мониторинг "объектов" в kubernetes (pod'ы, cronjob'ы, место на диске и пр.).
    * в [applications](../prometheus-rules/kubernetes/) лежат правила для мониторинга приложений (таких как redis, mongo и пр.)
* Изменения этих файлов (в том числе и создание новых) должно автоматически показываться на странице `/prometheus/rules` (требуется подождать около минуты после деплоя antiopa, пока отработает Prometheus Operator и компания).
* Если вы вносите изменение, а оно не показывается, путь диагностики следующий (подробнее см. [в нашей документации по устройству Prometheus Operator](../../200-prometheus-operator/docs/INTERNALS.md)):
    * Проверить, что ваши изменения попали в ConfigMap в Kubernetes:
        * `kubectl -n kube-prometheus get prometheusrule/prometheus-rules-<ИМЯ ДИРЕКТОРИИ> -o yaml`
        * Если изменений нет, то надо проверить, что antiopa сдеплоилась успешно:
            * `helm --tiller-namespace=antiopa list` — prometheus должен быть в статусе DEPLOYED
            * `kubectl -n antiopa logs deploy/antiopa -f` — в логе не должно быть ошибок
    * Проверить, что prometheus-config-reloader увидел ваши изменения:
          * В `kubectl -n kube-prometheus logs prometheus-main-0 prometheus-config-reloader -f` должна быть запись об этом:

                ts=2018-04-12T12:10:24Z caller=main.go:244 component=volume-watcher msg="ConfigMap modified."
                ts=2018-04-12T12:10:24Z caller=main.go:204 component=volume-watcher msg="Updating rule files..."
                ts=2018-04-12T12:10:24Z caller=main.go:209 component=volume-watcher msg="Rule files updated."

          * Если `prometheus-config-reloader` не видит изменений, то надо проверить prometheus-operator:
              * `kubectl -n kube-prometheus-operator get pod` — посмотреть, что pod запущен
              * `kubectl -n kube-prometheus-operator logs -f deploy/prometheus-operator --tail=50` — посмотреть, что в логе нет ошибок
          * Если `prometheus-config-reloader` не может релоаднуть prometheus, значит в правилах ошибка и надо смотреть лог Prometheus:
              * `kubectl -n kube-prometheus logs prometheus-main-0 prometheus -f`
          * **Важно!** Бывает так, что `prometheus-config-reloader` "зависает" на какой-то ошибке и перестает видеть новые изменения, а продолжает пытаться релоаднуть Prometheus со старой ошибочной конфигурацией. В этом случае единственное, что можно сделать — зайти в pod и прибить процесс `prometheus-config-reloader` (kubernetes перезапустит контейнер).

Лучшие практики
---------------


### Называть группу правил согласно нашему стандарту

Правила в Prometheus разделяются на группы (см. любой файл с правилами для примера). Группу нужно обязательно называть согласно следующему формату: `<имя директории>.<имя файла без расширения>.<имя группы>`. При этом имя группы можно опустить. Например:
* в [kubernetes/nginx-ingress.yaml](../prometheus-rules/kubernetes/nginx-ingress.yaml) есть три группы: `kubernetes.nginx-ingress.overview`, `kubernetes.nginx-ingress.details` и `kubernetes.nginx-ingress.controller`
* в [applications/redis.yaml](../prometheus-rules/applications/redis.yaml) есть только одна группа: `applications.redis`.


### Всегда явно указывать job

Имя метрики, пусть даже оно вам кажется уникальным, может перестать быть таковым в любой момент — в одном из кластеров кто-то может добавить custom приложение, которое возвращает метрики с таким-же названием, и все — ваши правила поломаются. Однако, мы очень четко контролируем лейбл job (благодаря servicemonitor'ам) и связка `название метрики` + `job` является гарантированно уникальной. Поэтому, **обязательно добавляйте имя job'а** во все запросы всех правил!

Например, метрика `nginx_filterzone_responses_total` является стандартной (ее экспортирует [nginx-vts-exporter](https://github.com/hnlq715/nginx-vts-exporter)), поэтому если не указать явно название job'а, то любое custom приложение экспортирующее эти метрики сломает все графики и все алерты ingress-nginx.
```
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
* Мы можем быть уверены, что мы не будем использовать `scrape_interval` больше 30m (это слишком редко и не имеет никакого практическогос смысла).

Таким образом получается, что `irate(foo[1h])` в правилах Prometheus эквивалентно `rate[$__interval_rv]` в Grafana (см. подробнее "Объяснение деталей и причин" в разделе ["Точность данных и детализация"](GRAFANA_DASHBOARD_DEVELOPMENT.md#Точность-данных-и-детализация) документации по разработке графиков Grafana).

### Не пытаться генерировать рулы Helm'ом

Prometheus rules — программируемая логика. Helm Chart template — программируемая логика. Использование одной программируемой логики для определения другой программируемой логики называется метапрограммированием и не стоит этим злоупотреблять не имея на то очень веских причин.

Например, если вы хотите в каком-то алерте сделать возможность переопределения порогового значения, то:
* Нужно сделать вот такой query: `foo_value > scalar(foo_threshold or vector(5))`.
    * если метрика `foo_threshold` определена, то `foo_value` будет сравниваться с ее значением,
    * если такой метрики нет — `foo_value` будет сравниваться с 5
* Тогда в том кластере, где это необходимо, можно сделать custom'ный рул, который будет возвращать метрику `foo_threshold` с необходимым пороговым значением.
