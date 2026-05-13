---
title: Модуль prometheus
permalink: ru/architecture/storage/prometheus.html
lang: ru
search: prometheus module, monitoring architecture, monitoring components, monitoring, metrics, архитектура мониторинга, компоненты мониторинга, мониторинг, метрики
description: Архитектура модуля prometheus в Deckhouse Kubernetes Platform.
---

Модуль `prometheus` разворачивает стек мониторинга с предустановленными параметрами для Deckhouse Kubernetes Platform (DKP) и приложений, что упрощает начальную настройку.

Подробнее с описанием модуля можно ознакомиться в [соответствующем разделе документации](/modules/prometheus/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`prometheus`](/modules/prometheus/) на уровне 2 модели C4 и его взаимодействия с другими компонентами DKP изображены на следующей диаграмме:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля prometheus](../../../images/architecture/observability/c4-l2-prometheus.ru.png)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Prometheus-main** (StatefulSet) — основной Prometheus. [Prometheus](https://github.com/prometheus/prometheus) — система мониторинга и оповещения, использующая базу данных временных рядов (TSDB или time series database). Она в реальном времени собирает и анализирует метрики работы приложений и серверов. Prometheus-main собирает метрики с настроенных объектов мониторинга каждые 30 секунд. С помощью параметра [scrapeInterval](/modules/prometheus/configuration.html#parameters-scrapeinterval) можно изменить это значение. 

Prometheus-main обрабатывает настроенные правила, отправляет алерты и является основным источником данных. Prometheus-main также использует настроенный с помощью файла конфигурации `prometheus.yaml` свой внутренний механизм Service Discovery. Посредством Service Discovery prometheus-main взаимодействует с kube-apiserver (в основном получает endpoint`ы) и обновляет список target'ов (целей для мониторинга). Подробнее с описанием работы компонента prometheus-main можно ознакомиться в разделе [Архитектура мониторинга](monitoring.html#prometheus). 

В модуле по умолчанию используется [Deckhouse Prom++](https://github.com/deckhouse/prompp) — высокопроизводительный форк [Prometheus](https://github.com/prometheus/prometheus) с открытым исходным кодом, разработанный для значительного сокращения потребления памяти при сохранении полной совместимости с оригинальным проектом. Deckhouse Prom++ является разработкой компании «Флант». 

Есть возможность переключиться с Deckhouse Prom++ на оригинальный Prometheus. В этом случае потребуется миграция данных журнала упреждающей записи (WAL или write-ahead log), поскольку в Deckhouse Prom++ используется свой формат журнала WAL. Миграция осуществляется автоматически при помощи init-контейнера prompptool.

   Состоит из следующих контейнеров:

   * **init-config-reloader** — init-контейнер, выполняющий однократный запуск config-reloader для загрузки конфигурации Prometheus;
   * **prompptool** — init-контейнер, выполняющий автоматическую миграцию данных журнала WAL в случае переключения с Deckhouse Prom++ на оригинальный Prometheus и наоборот;
   * **config-reloader** — сайдкар-контейнер, который выполняет следующие операции:
     * следит за изменениями в файле конфигурации `prometheus.yaml` и, при необходимости, вызывает перезагрузку конфигурации Prometheus (HTTP-запросом на специальный эндпойнт `/-/reload`);
     * следит за PrometheusRule'ами и по необходимости скачивает их и перезапускает Prometheus.

     Config-reloader является [утилитой](https://github.com/coreos/prometheus-operator/tree/master/cmd/prometheus-config-reloader) из Open Source-проекта [Prometheus Operator](https://github.com/coreos/prometheus-operator/).

   * **prometheus** — 
   
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к серверу Prometheus. Является [Open Source-проектом](https://github.com/brancz/kube-rbac-proxy).

2. **Prometheus-longterm** (StatefulSet) — дополнительный Prometheus, хранящий выборку разреженных метрик из основного prometheus-main. Это позволяет пользователям просматривать и анализировать исторические тренды за длительный период времени. Prometheus-longterm получает данные благодаря настроенной федерации с основным Prometheus. Состав контейнеров у prometheus-longterm такой же, как и у prometheus-main. 

{% alert level="info" %}
Для отображения дашбордов мониторинга в веб-интерфейсе DKP используется Grafana, входящая в модуль [observability](/modules/observability/).
{% endalert %}

3. **Grafana-v10** — необязательный компонент Grafana, предоставляющий веб-интерфейс для визуализации данных мониторинга. Grafana включает подготовленные дашборды для всех модулей DKP и некоторых популярных приложений. Grafana умеет работать в режиме высокой доступности, не хранит состояние и настраивается с помощью [кастомных ресурсов](/modules/prometheus/cr.html#grafanaadditionaldatasource). Grafana по умолчанию включена, но ее можно удалить из модуля при помощи [следующего параметра модуля](/modules/prometheus/configuration.html#parameters-grafana-enabled).

   Состоит из следующих контейнеров:

   * **dashboard-provisioner** — сайдкар-контейнер, который следит за кастомными ресурсами [GrafanaDashboardDefinition](/modules/prometheus/cr.html#grafanadashboarddefinition) и при появлении новых GrafanaDashboardDefinition добавляет описанные в них дашборды в фолдер Grafana;
   * **grafana** — основной контейнер. Является [Open Source-проектом](https://github.com/grafana/grafana);
   * **kube-rbac-proxy** — сайдкар-контейнер, обеспечивающий авторизованный доступа к серверу Grafana и его метрикам. Подробно описан выше.

4. **Aggregating-proxy** — агрегирующий и кеширующий прокси, объединяющий main-prometheus и longterm-prometheus в один источник. Помогает избежать провалов в данных при недоступности одного из Prometheus.

   Состоит из следующих контейнеров:

   * **wait-memcached** — init-контейнер, ожидающий доступности компонента memcached по сети. Aggregating-proxy использует memcached для кеширования метрик в оперативной памяти;
   * **mimir** — сайдкар-контейнер, работающий с компонентом memcached для оптимизации запросов и кэширования данных. При отсутствии данных в кеше, mimir пересылает запрос на компонент prometheus-main через еще один сайдкар-контейнер promxy. Является [Open Source-проектом](https://github.com/grafana/mimir);
   * **promxy** — сайдкар-контейнер, проксирующий запросы на компонент prometheus-main. Promxy - это прокси-сервер для Prometheus, который позволяет нескольким узлам Prometheus выглядеть как одна конечная точка API для пользователя. Является [Open Source-проектом](https://github.com/jacksontj/promxy);
   * **kube-rbac-proxy** — сайдкар-контейнер, обеспечивающий авторизованный доступа к контейнерам mimir (запросы на сервер Prometheus и запросы на метрики контейнера) и promxy (запросы на метрики контейнера). Подробно описан выше.

5. **Memcached** (StatefulSet) — компонент, используемый aggregating-proxy для кэширования метрик Prometheus. Memcached - программное обеспечение, реализующее сервис кэширования данных в оперативной памяти. Цель — ускорить выполнение запросов к метрикам Prometheus. 

   Состоит из следующих контейнеров:

   * **memcached** — основной контейнер. Является [Open Source-проектом](https://github.com/memcached/memcached);
   * **exporter** — сайдкар-контейнер, экспортирующий метрики контейнера memcached. Memcached собирает метрики контейнера memcached через сетевое подключение, а также из PID-файла процесса memcached. Является [Open Source-проектом](https://github.com/prometheus/memcached_exporter).

6. **Trickster** — кеширующий прокси-сервер, снижающий нагрузку на Prometheus. Используется для кеширования и проксирования запросов на prometheus-longterm. В ближайшее время будет deprecated.

   Состоит из следующих контейнеров:

   * **trickster** — основной контейнер. Является [Open Source-проектом](https://github.com/trickstercache/trickster);
   * **kube-rbac-proxy** — сайдкар-контейнер, обеспечивающий авторизованный доступа к прокси-серверу и его метрикам. Подробно описан выше.

7. **Alerts-receiver** — сервер, совместимый с API [Alertmanager](https://github.com/prometheus/alertmanager). Alerts-receiver принимает базовые алерты от prometheus-main, создает на их основе кастомные ресурсы [ClusterAlerts](https://deckhouse.ru/modules/prometheus/cr.html#clusteralert), обновляет их статусы и удаляет, если алерт больше не активен. Кастомные ресурсы ClusterAlerts используется для информирования пользователей DKP об активных алертах и отображаются в веб-интерфейсе платформы. Является разработкой компании «Флант». Состоит из одного контейнера.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   * мониторинг кастомных ресурсов PrometheusRule и GrafanaDashboardDefinition;
   * управление кастомными ресурсами ClusterAlert;
   * авторизация запросов на получение метрик компонентов модуля.

2. **Alertmanager** — отправка кастомных алертов.

Prometheus, входящий в состав модуля, собирает метрики со всех компонентов DKP:
   
   * компоненты модулей;
   * компоненты control plane кластера;
   * экспортеры, собирающие метрики загрузки аппаратных ресурсов кластеры;
   * экспортеры, собирающие метрики ресурсов Kubernetes;
   * пользовательские приложения (требуется дополнительная настройка).

   Взаимодействия Prometheus с компонентами DKP, связанные со сбором метрик, не показаны на схеме, чтобы не усложнять её большим количеством связей.

С модулем взаимодействуют следующие внешние компоненты:

1. **Ingress-controller** (controller nginx на схеме) — пересылает запросы пользователей к Grafana.
