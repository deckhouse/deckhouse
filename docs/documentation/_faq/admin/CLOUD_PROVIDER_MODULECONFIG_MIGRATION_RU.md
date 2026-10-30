---
title: Как мигрировать облачный провайдер на конфигурацию через ModuleConfig?
subsystems:
  - cluster_infrastructure
lang: ru
---

Если облачный кластер был установлен со схемой `<PROVIDER>ClusterConfiguration` (например, DVPClusterConfiguration, AWSClusterConfiguration и т. д.), эту конфигурацию нужно перенести на новую модель на базе ModuleConfig.

Deckhouse Platform переходит от единого ресурса `<PROVIDER>ClusterConfiguration` к модели, в которой конфигурация облачного провайдера разделена между четырьмя отдельными ресурсами:

1. [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) модуля `cloud-provider-<PROVIDER>` — настройки провайдера и схемы размещения;
1. Секрет с учётными данными типа `cloud-provider.deckhouse.io/credentials` — доступ к API облака;
1. InstanceClass провайдера (например, [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass)) — параметры виртуальных машин;
1. [NodeGroup](/modules/node-manager/cr.html#nodegroup) — описание групп узлов.

{% alert level="warning" %}
Миграция обязательна. Это не опциональный шаг: поддержка `<PROVIDER>ClusterConfiguration` будет прекращена. Пока миграция не выполнена, обновление DP может быть заблокировано.
{% endalert %}

Миграция безопасна. Она подразумевает применение автоматически подготовленных DP ресурсов и **не приводит к пересозданию узлов**. При этом требуется осознанное действие администратора, который просматривает и применяет подготовленные ресурсы вручную.

## Как выполнить миграцию

1. Дождитесь появления алерта о необходимости миграции. У каждого провайдера он свой, например [`D8CloudProviderDVPMigrationPending`](reference/alerts.html#cloud-provider-dvp-d8cloudproviderdvpmigrationpending) для DVP и [`D8CloudProviderYandexCloudMigrationPending`](reference/alerts.html#cloud-provider-yandex-d8cloudprovideryandexcloudmigrationpending) для Yandex Cloud. Алерт означает, что модуль обнаружил устаревшую конфигурацию и подготовил манифесты для перехода.

1. Просмотрите подготовленные ресурсы в секрете `d8-migration-resources` в неймспейсе `d8-cloud-provider-<PROVIDER>`:

   ```shell
   d8 k -n d8-cloud-provider-<PROVIDER> get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d
   ```

   Пример для встроенной виртуализации:

   ```shell
   d8 k -n d8-cloud-provider-dvp get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d
   ```

1. Убедитесь, что манифесты соответствуют ожидаемой конфигурации кластера (ModuleConfig, секрет с учётными данными, InstanceClass и NodeGroup).

1. Примените ресурсы:

   ```shell
   d8 k -n d8-cloud-provider-<PROVIDER> get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d | d8 k apply -f -
   ```

   Пример для встроенной виртуализации:

   ```shell
   d8 k -n d8-cloud-provider-dvp get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d | d8 k apply -f -
   ```

1. Дождитесь исчезновения алерта. Модуль снимет его автоматически, когда обнаружит, что все необходимые ресурсы применены.

## Как удалить устаревший секрет

После миграции кластера, развёрнутого во встроенной виртуализации, в Yandex Cloud или в zVirt, настройки провайдера задаются через ModuleConfig, а устаревший секрет `d8-provider-cluster-configuration` в неймспейсе `kube-system` можно удалить вручную.

Чтобы удалить секрет, выполните следующие шаги:

1. Убедитесь, что миграция завершена и алерт о ней больше не срабатывает. Для встроенной виртуализации это алерт [`D8CloudProviderDVPMigrationPending`](reference/alerts.html#cloud-provider-dvp-d8cloudproviderdvpmigrationpending), для Yandex Cloud алерт [`D8CloudProviderYandexCloudMigrationPending`](reference/alerts.html#cloud-provider-yandex-d8cloudprovideryandexcloudmigrationpending), для zVirt алерт [`D8CloudProviderZvirtMigrationPending`](reference/alerts.html#cloud-provider-zvirt-d8cloudproviderzvirtmigrationpending). Если удалить секрет до завершения миграции, модуль облачного провайдера перестанет корректно работать.

1. Проверьте число узлов в группах узлов с типом `CloudPermanent`:

   ```shell
   d8 k get nodegroups \
     -o custom-columns='NAME:.metadata.name,TYPE:.spec.nodeType,MIN_PER_ZONE:.spec.cloudInstances.minPerZone'
   ```

   После удаления секрета `dhctl converge` берёт число узлов в этих группах из параметра [`spec.cloudInstances.minPerZone`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-minperzone). Если у группы узлов с типом `CloudPermanent` параметр не задан, `dhctl converge` завершится с ошибкой.

1. Проверьте, что данные об инфраструктуре облака, например идентификаторы сетей и зоны, хранятся в отдельном секрете. Команда должна вывести непустое значение:

   ```shell
   d8 k -n d8-cloud-provider-<PROVIDER> get secret d8-candi-cloud-provider-discovery-data \
     -o jsonpath='{.data.cloud-provider-discovery-data\.json}'
   ```

   Вместо `<PROVIDER>` укажите `dvp` для встроенной виртуализации, `yandex` для Yandex Cloud или `zvirt` для zVirt.

   Если команда ничего не вывела или секрет не найден, данные хранятся только в устаревшем секрете. Если удалить его без копии данных, модуль `terraform-manager` отключится. Модуль `cloud-provider-yandex` также потеряет идентификаторы сети, подсетей и таблицы маршрутизации, которые платформа создала в Yandex Cloud. Скопируйте данные в отдельный секрет и удалите временный файл:

   ```shell
   d8 k -n kube-system get secret d8-provider-cluster-configuration \
     -o jsonpath='{.data.cloud-provider-discovery-data\.json}' \
     | base64 -d > /tmp/discovery-data.json

   d8 k -n d8-cloud-provider-<PROVIDER> create secret generic d8-candi-cloud-provider-discovery-data \
     --from-file=cloud-provider-discovery-data.json=/tmp/discovery-data.json \
     --dry-run=client -o yaml \
     | d8 k apply -f -

   rm /tmp/discovery-data.json
   ```

   После этого повторите проверку и убедитесь, что команда выводит непустое значение.

1. Сохраните резервную копию секрета. Секрет содержит учётные данные облака, поэтому храните файл в защищённом месте:

   ```shell
   d8 k -n kube-system get secret d8-provider-cluster-configuration \
     -o yaml > d8-provider-cluster-configuration-backup.yaml
   ```

1. Разрешите удаление секрета. Секрет защищён вебхуком, который запрещает удаление без аннотации `deckhouse.io/allow-delete`:

   ```shell
   d8 k -n kube-system annotate secret d8-provider-cluster-configuration \
     deckhouse.io/allow-delete=true
   ```

1. Удалите секрет:

   ```shell
   d8 k -n kube-system delete secret d8-provider-cluster-configuration
   ```
