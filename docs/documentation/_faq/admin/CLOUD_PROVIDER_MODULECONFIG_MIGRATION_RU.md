---
title: Как мигрировать облачный провайдер на конфигурацию через ModuleConfig?
subsystems:
  - cluster_infrastructure
lang: ru
---

Если облачный кластер был установлен со схемой `<PROVIDER>ClusterConfiguration` (например, DVPClusterConfiguration, AWSClusterConfiguration и т. д.), эту конфигурацию нужно перенести на новую модель на базе ModuleConfig.

Deckhouse Kubernetes Platform переходит от единого ресурса `<PROVIDER>ClusterConfiguration` к модели, в которой конфигурация облачного провайдера разделена между четырьмя отдельными ресурсами:

1. [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) модуля `cloud-provider-<PROVIDER>` — настройки провайдера и схемы размещения;
1. Секрет с учётными данными типа `cloud-provider.deckhouse.io/credentials` — доступ к API облака;
1. InstanceClass провайдера (например, [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass)) — параметры виртуальных машин;
1. [NodeGroup](/modules/node-manager/cr.html#nodegroup) — описание групп узлов.

{% alert level="warning" %}
Миграция обязательна. Это не опциональный шаг: поддержка `<PROVIDER>ClusterConfiguration` будет удалена. Пока миграция не выполнена, обновление DKP может быть заблокировано.
{% endalert %}

Миграция безопасна, применение подготовленных ресурсов **не приводит к пересозданию узлов**. При этом требуется осознанное действие администратора — ресурсы нужно просмотреть и применить вручную.

## Как выполнить миграцию

1. Дождитесь появления алерта о необходимости миграции (например, для DVP — `D8CloudProviderDVPMigrationPending`). Это означает, что модуль обнаружил устаревшую конфигурацию и подготовил манифесты для перехода.

1. Просмотрите подготовленные ресурсы в секрете `d8-migration-resources` в неймспейсе `d8-cloud-provider-<PROVIDER>`:

   ```shell
   d8 k -n d8-cloud-provider-<PROVIDER> get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d
   ```

   Пример для DVP:

   ```shell
   d8 k -n d8-cloud-provider-dvp get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d
   ```

1. Убедитесь, что манифесты соответствуют ожидаемой конфигурации кластера (ModuleConfig, секрет с учётными данными, InstanceClass и NodeGroup).

1. Примените ресурсы:

   ```shell
   d8 k -n d8-cloud-provider-<PROVIDER> get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d | d8 k apply -f -
   ```

   Пример для DVP:

   ```shell
   d8 k -n d8-cloud-provider-dvp get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d | d8 k apply -f -
   ```

1. Дождитесь исчезновения алерта. Модуль снимет его автоматически, когда обнаружит, что все необходимые ресурсы применены.
