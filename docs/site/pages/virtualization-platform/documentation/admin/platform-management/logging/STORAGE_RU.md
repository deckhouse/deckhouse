---
title: Кратковременное хранение логов
permalink: ru/virtualization-platform/documentation/admin/platform-management/logging/storage.html
lang: ru
---

Deckhouse предоставляет встроенное решение для кратковременного хранения логов на базе проекта [Grafana Loki](https://grafana.com/oss/loki/).

Хранилище разворачивается в кластере и интегрируется с системой сбора логов.
После настройки ресурсов [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig), [PodLoggingConfig](/modules/log-shipper/cr.html#podloggingconfig) и [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination)
логи автоматически поступают со всех системных компонентов.
Настроенное хранилище добавляется в Grafana в качестве источника данных для визуализации и анализа.

Сбор логов с пользовательских приложений настраивается отдельно.

Параметры кратковременного хранилища задаются в настройках модуля [`loki`](/modules/loki/configuration.html).
В том числе возможно настроить размер диска и срок хранения, задать используемый StorageClass и ресурсы.

{% alert level="warning" %}
Кратковременное хранилище на базе Grafana Loki не поддерживает работу в режиме высокой доступности.
Для долговременного хранения важных логов используйте внешнее хранилище.
{% endalert %}

## Интеграция с Grafana Cloud

Чтобы настроить работу Deckhouse с платформой Grafana Cloud, выполните следующие шаги:

1. Создайте [ключ доступа к API Grafana Cloud](https://grafana.com/docs/grafana-cloud/reference/create-api-key/).
1. Закодируйте токен доступа к Grafana Cloud в формате Base64:

   ![API-ключ Grafana Cloud](/images/log-shipper/grafana_cloud.png)

   ```bash
   echo -n "<YOUR-GRAFANACLOUD-TOKEN>" | base64 -w0
   ```

1. Создайте ресурс [ClusterLogDestination](/modules/log-shipper/cr.html#clusterlogdestination), следуя примеру:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ClusterLogDestination
   metadata:
     name: loki-storage
   spec:
     loki:
       auth:
         password: PFlPVVItR1JBRkFOQUNMT1VELVRPS0VOPg==
         strategy: Basic
         user: "<YOUR-GRAFANACLOUD-USERNAME>"
       endpoint: <YOUR-GRAFANACLOUD-URL> # Например https://logs-prod-us-central1.grafana.net или https://logs-prod-eu-west-0.grafana.net
     type: Loki
   ```

## Переход с Grafana Promtail

Для миграции с Promtail отредактируйте URL-адрес Loki, убрав из него путь `/loki/api/v1/push`.

Агент логирования Vector, который используется в Deckhouse, автоматически добавит этот путь при отправке данных в Loki.
