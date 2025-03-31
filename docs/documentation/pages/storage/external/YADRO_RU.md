---
title: "Унифицированное хранилище TATLIN.UNIFIED (Yadro)"
permalink: ru/storage/admin/external/yadro.html
lang: ru
d8Revision: ee
---

{% alert level="info" %}
Доступно в некоторых коммерческих редакциях:  **EE**

Подробнее см. в разделе [Условия и цены](../../../../../pricing/).
{% endalert %}

Deckhouse поддерживает интеграцию с системой хранения данных [TATLIN.UNIFIED (Yadro)](https://yadro.com/ru/tatlin/unified), предоставляя возможность управления томами в Kubernetes. Это позволяет использовать централизованное хранилище для контейнеризированных рабочих нагрузок, обеспечивая высокую производительность и отказоустойчивость.

На этой странице представлены инструкции по подключению [TATLIN.UNIFIED (Yadro)](https://yadro.com/ru/tatlin/unified) к Deckhouse, настройке соединения, созданию StorageClass, а также проверке работоспособности системы.

## Включение модуля

Для управления томами на основе системы хранения данных [TATLIN.UNIFIED (Yadro)](https://yadro.com/ru/tatlin/unified) в Deckhouse используется модуль `csi-yadro-tatlin-unified`, позволяющий создавать ресурсы StorageClass через создание пользовательских ресурсов [YadroTatlinUnifiedStorageClass](../../../reference/cr/yadrotatlinunifiedstorageclass/). Чтобы включить модуль, выполните команду:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-yadro-tatlin-unified
spec:
  enabled: true
  version: 1
EOF
```

Дождитесь, когда модуль `csi-yadro-tatlin-unified` перейдет в состояние `Ready`. Проверить состояние модуля можно, выполнив следующую команду:

```shell
d8 k get module csi-yadro-tatlin-unified -w
```

В результате будет выведена информация о модуле:

```console
NAME                       WEIGHT   STATE     SOURCE     STAGE   STATUS
csi-yadro-tatlin-unified   910      Enabled   Embedded           Ready
```

## Подключение к системе хранения данных TATLIN.UNIFIED

Чтобы создать подключение к системе хранения данных `TATLIN.UNIFIED` и иметь возможность настраивать объекты StorageClass, примените следующий ресурс [YadroTatlinUnifiedStorageConnection](../../../reference/cr/yadrotatlinunifiedstorageconnection/):

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroTatlinUnifiedStorageConnection
metadata:
  name: yad1
spec:
  controlPlane:
    address: "172.19.28.184"
    username: "admin"
    password: "cGFzc3dvcmQ=" # Должен быть закодирован в Base64
    ca: "base64encoded"
    skipCertificateValidation: true
  dataPlane:
    protocol: "iscsi"
    iscsi:
      volumeExportPort: "p50,p51,p60,p61"
EOF
```

## Создание StorageClass

Для создания StorageClass необходимо использовать ресурс [YadroTatlinUnifiedStorageClass](../../../reference/cr/yadrotatlinunifiedstorageclass/). Ручное создание ресурса StorageClass без [YadroTatlinUnifiedStorageClass](../../../reference/cr/yadrotatlinunifiedstorageclass/) может привести к ошибкам.

Пример команды для создания класса хранения на основе системы хранения данных `TATLIN.UNIFIED`:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroTatlinUnifiedStorageClass
metadata:
  name: yad1
spec:
  fsType: "xfs"
  pool: "pool-hdd"
  storageConnectionName: "yad1"
  reclaimPolicy: Delete
EOF
```

## Проверка работоспособности модуля

Для того чтобы проверить работоспособность модуля `csi-yadro-tatlin-unified`, необходимо проверить состояние подов в пространстве имён `d8-csi-yadro-tatlin-unified`. Все поды должны быть в состоянии `Running` или `Completed`, поды `csi-yadro-tatlin-unified` должны быть запущены на всех узлах.

Проверить работоспособность модуля можно с помощью команды:

```shell
d8 k -n d8-csi-yadro-tatlin-unified get pod -owide -w
```
