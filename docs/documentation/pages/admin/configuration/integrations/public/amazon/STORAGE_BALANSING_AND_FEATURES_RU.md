---
title: Хранилище и балансировка
permalink: ru/admin/integrations/public/amazon/storage.html
lang: ru
---

Этот раздел охватывает дополнительные аспекты интеграции Deckhouse Kubernetes Platform (DKP) с AWS:

- Подключение облачных дисков через CSI;
- Автоматическое создание StorageClass;
- Использование LoadBalancer;
- Доступ через bastion-хост;
- Подключение вручную созданных CloudStatic-узлов.

## Хранилище (CSI и StorageClass)

DKP обеспечивает интеграцию с хранилищем AWS через CSI. Это позволяет кластеру автоматически заказывать и подключать диски к своим узлам.

StorageClass создаётся автоматически для следующих типов дисков:

- `gp3`;
- `gp2`;
- `sc1`;
- `st1`.

Также поддерживаются диски `io1` и `io2` с настройками IOPS и throughput в ModuleConfig.

Для исключения из кластера ненужных StorageClass укажите фильтры в [`settings.storageClass.exclude`](/modules/cloud-provider-aws/configuration.html#parameters-storageclass-exclude):

```yaml
settings:
  storageClass:
    exclude:
    - sc.*
    - st1
```

Можно явно указать класс и настроить для StorageClass параметры, включая `iops`, `throughput`, `type`:

```yaml
settings:
  storageClass:
    provision:
    - name: fast-io
      type: io2
      iopsPerGB: "50"
```

### Увеличение объема тома (volume)

Чтобы изменить размер тома (например, при нехватке дискового пространства), выполните следующие шаги:

1. Измените параметр `spec.resources.requests.storage` в соответствующем объекте PersistentVolumeClaim. Операция выполняется автоматически и обычно занимает не более одной минуты. За ходом процесса можно наблюдать с помощью команды:

   ```shell
   d8 k describe pvc <имя-claim>
   ```

1. После изменения объема подождите не менее 6 часов и убедитесь, что статус volume — `in-use` или `available`. Только после этого возможны повторные изменения. Подробнее [в официальной документации AWS](https://docs.aws.amazon.com/ebs/latest/userguide/modify-volume-requirements.html).

## Балансировка нагрузки

DKP поддерживает работу c LoadBalancer Service через AWS Load Balancer Controller.

Для контроля создания ресурсов в AWS используются аннотации объекта Service:

```yaml
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
    service.beta.kubernetes.io/aws-load-balancer-subnets: "subnet-12345,subnet-67890"
```

- `service.beta.kubernetes.io/aws-load-balancer-type` — при значении none создаётся только Target Group, без самого LoadBalancer.
- `service.beta.kubernetes.io/aws-load-balancer-backend-protocol` — применяется только в связке с `aws-load-balancer-type`: `none` и определяет протокол взаимодействия с Target Group. Поддерживаются следующие значения:
  - `tcp` (значение по умолчанию);
  - `tls`;
  - `http`;
  - `https`.

{% alert level="info" %}
При изменении значения этой аннотации `cloud-controller-manager` попытается пересоздать Target Group. Если она уже используется в связке с NLB или ALB, удалить её не удастся, и контроллер будет бесконечно повторять попытку. Чтобы избежать этого, необходимо вручную отсоединить балансировщик от группы.

Если Ingress-узлы есть не во всех зонах, укажите явно подсети в `aws-load-balancer-subnets`.
{% endalert %}

### Настройка балансировщика при отсутствии Ingress-узлов в некоторых зонах

Если Ingress-узлы присутствуют не во всех зонах AWS, необходимо явно указать, какие подсети использовать для балансировщика. Это делается с помощью аннотации `service.beta.kubernetes.io/aws-load-balancer-subnets`:

```yaml
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-subnets: "subnet-foo,subnet-bar"
```

Это особенно важно при ручной настройке Ingress-контроллеров или нестандартной схеме размещения узлов.

Чтобы получить список текущих подсетей, используемых в установке DKP, выполните команду:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse \
  -- deckhouse-controller module values cloud-provider-aws -o json | \
  jq -r '.cloudProviderAws.internal.zoneToSubnetIdMap'
```

## Подключение CloudStatic-узлов

Для подключения созданных вручную EC2-инстансов к кластеру DKP:

1. Добавьте IAM-роль `<prefix>-node`.
1. Добавьте группу безопасности `<prefix>-node`.
1. Укажите теги:

   ```yaml
   "kubernetes.io/cluster/<cluster_uuid>" = "shared"
   "kubernetes.io/cluster/<prefix>" = "shared"
   ```

Узнать `cluster_uuid` можно с помощью команды:

```shell
d8 k -n kube-system get cm d8-cluster-uuid -o json | jq -r '.data."cluster-uuid"'
```

Узнать `prefix` можно с помощью команды:

```shell
d8 k -n kube-system get secret d8-cluster-configuration -o json | \
  jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix
```
