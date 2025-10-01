---
title: "Cloud provider — Yandex Cloud: примеры"
---

Ниже представлен пример конфигурации cloud-провайдера Yandex Cloud.

## Пример конфигурации модуля

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 1
  enabled: true
  settings:
    additionalExternalNetworkIDs:
    - enp6t4snovl2ko4p15em
```

## Пример custom resource `YandexInstanceClass`

```yaml
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: test
spec:
  cores: 4
  memory: 8192
```

## LoadBalancer

### Аннотации объекта Service

Значения по умолчанию (эти параметры применяются ко всем Service, если для конкретного Service они не переопределены аннотациями):

- `YANDEX_CLOUD_DEFAULT_LB_TARGET_GROUP_NETWORK_ID` — обязательная переменная. Указывает NetworkID, в котором по умолчанию создаётся Target Group.
- `YANDEX_CLOUD_DEFAULT_LB_LISTENER_SUBNET_ID` — задаёт SubnetID для listener’ов создаваемых NLB по умолчанию.

> **Внимание.** Все новые NLB внутренние (internal) по умолчанию, поведение можно перекрыть аннотацией yandex.cpi.flant.com/loadbalancer-external.

В Yandex Cloud Controller Manager поддерживаются следующие аннотации:

1. `yandex.cpi.flant.com/target-group-network-id` — заменяет значение по умолчанию из `YANDEX_CLOUD_DEFAULT_LB_TARGET_GROUP_NETWORK_ID` для конкретного Service. Указывает NetworkID, в котором будет создан Target Group для NLB.
1. `yandex.cpi.flant.com/listener-subnet-id` — заменяет значение по умолчанию из `YANDEX_CLOUD_DEFAULT_LB_LISTENER_SUBNET_ID` для конкретного Service. Задаёт SubnetID для Listener’ов создаваемого NLB. При использовании этой аннотации создаваемые NLB будут внутренними (internal).
1. `yandex.cpi.flant.com/listener-address-ipv4` — позволяет задать предопределённый IPv4-адрес для listener’ов. Работает как для внутренних, так и для внешних NLB.
1. `yandex.cpi.flant.com/loadbalancer-external` — заменяет поведение по умолчанию (все новые NLB — внутренние). С помощью этой аннотации можно включить создание внешнего (external) NLB для конкретного Service.
1. `yandex.cpi.flant.com/target-group-name-prefix` — задаёт префикс имени Target Group в формате `<значение аннотации><Yandex cluster name><NetworkID>` (для Service). Аналогичная аннотация может быть выставлена на узле, чтобы включать узел в нестандартную Target Group (будут созданы TG с именами `<значение аннотации><Yandex cluster name><network id интерфейсов инстанса>`).

Если для управляющего слоя (control plane) или master-узлов создаются отдельные Target Group, добавьте на master-узлы метку `node.kubernetes.io/exclude-from-external-load-balancers: ""`. Это предотвратит попытки контроллера автоматически добавлять master-узлы в новые Target Group для балансировщиков.

Если вы создаёте собственный балансировщик для master-узлов и хотите, чтобы YCC также мог размещать свои балансировщики на master-узлах, заранее создайте Target Group с именем по маске `${CLUSTER-NAME}${VPC.ID}`.

### Проверки состояния Target Group

Параметры healthcheck’ов (для создаваемых NLB Target Group):

1. `yandex.cpi.flant.com/healthcheck-interval-seconds` — как часто запускать проверку, в секундах (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-timeout-seconds` — сколько ждать ответа от эндпоинта, в секундах. Если за это время ответ не получен, проверка считается неуспешной (по умолчанию 1).
1. `yandex.cpi.flant.com/healthcheck-unhealthy-threshold` — сколько подряд неуспешных проверок нужно, чтобы пометить эндпоинт как неработоспособный (unhealthy) и исключить его из балансировки (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-healthy-threshold` — сколько подряд успешных проверок нужно, чтобы вернуть эндпоинт в статус работоспособный (healthy) и снова включить его в балансировку (по умолчанию 2).
