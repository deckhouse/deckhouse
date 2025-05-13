---
title: Хранилище и балансировка
permalink: ru/admin/integrations/public/amazon/amazon-storage.html
lang: ru
---

Этот раздел охватывает дополнительные аспекты интеграции Deckhouse с AWS:

- Подключение облачных дисков через CSI;
- Автоматическое создание StorageClass;
- Использование LoadBalancer;
- Доступ через bastion-хост;
- Подключение вручную созданных CloudStatic-узлов.

## Хранилище (CSI и StorageClass)

Deckhouse обеспечивает интеграцию с хранилищем AWS через CSI. Это позволяет кластерам автоматически заказывать и подключать диски к своим узлам.

DKP автоматически создает StorageClass для следующих типов дисков:

- `gp3`;
- `gp2`;
- `sc1`;
- `st1`.

Также поддерживаются диски `io1` и `io2` с настройками IOPS и throughput в ModuleConfig.

Для исключения из кластера ненужных StorageClass укажите фильтры в `settings.storageClass.exclude`:

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

## Балансировка нагрузки

Deckhouse поддерживает работу c LoadBalancer Service через AWS Load Balancer Controller.

Для контроля создания ресурсов в AWS используются аннотации объекта Service:

```yaml
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
    service.beta.kubernetes.io/aws-load-balancer-subnets: "subnet-12345,subnet-67890"
```

- `service.beta.kubernetes.io/aws-load-balancer-type` — при значении none создаётся только Target Group, без самого LoadBalancer.
- `service.beta.kubernetes.io/aws-load-balancer-backend-protocol` — применяется только в связке с `aws-load-balancer-type`: `none` и определяет протокол взаимодействия с Target Group. Поддерживаются следующие значения:
  - tcp (значение по умолчанию);
  - tls;
  - http;
  - https.

> При изменении значения этой аннотации `cloud-controller-manager` попытается пересоздать Target Group. Если она уже используется в связке с NLB или ALB, удалить её не удастся, и контроллер будет бесконечно повторять попытку. Чтобы избежать этого, необходимо вручную отсоединить балансировщик от группы.
>
> Если Ingress-узлы есть не во всех зонах, укажите явно подсети в `aws-load-balancer-subnets`.

## Подключение CloudStatic-узлов

Для подключения ручно созданных EC2-инстансов к кластеру Deckhouse:

1. Добавьте IAM-роль `<prefix>-node`.
1. Добавьте группу безопасности `<prefix>-node`.
1. Укажите теги:

   ```yaml
   "kubernetes.io/cluster/<cluster_uuid>" = "shared"
   "kubernetes.io/cluster/<prefix>" = "shared"
   ```

Узнать `cluster_uuid` можно с помощью команды:

```console
kubectl -n kube-system get cm d8-cluster-uuid -o json | jq -r '.data."cluster-uuid"'
```

Узнать `prefix` можно с помощью команды:

```console
kubectl -n kube-system get secret d8-cluster-configuration -o json | \
  jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix
```
