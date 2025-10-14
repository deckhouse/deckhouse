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

В кластере заданы значения по умолчанию для размещения ресурсов балансировщиков (сеть для Target Group и подсеть для Listener). Эти значения выставляются автоматически во время развёртывания кластера и могут быть переопределены аннотациями на уровне конкретного Service.

> Поведение по умолчанию (внешний или внутренний LB) зависит от конфигурации кластера. Для явного выбора типа используйте аннотацию `yandex.cpi.flant.com/loadbalancer-external`.

В Yandex Cloud Controller Manager поддерживаются следующие аннотации:

1. `yandex.cpi.flant.com/target-group-network-id` — указывает NetworkID, в котором будет создана Target Group для данного Service. Переопределяет соответствующее значение по умолчанию.
1. `yandex.cpi.flant.com/listener-subnet-id` — задаёт SubnetID для Listener’ов создаваемого LB для данного Service. Переопределяет соответствующее значение по умолчанию.
1. `yandex.cpi.flant.com/listener-address-ipv4` — задаёт предопределённый IPv4-адрес для Listener’ов (поддерживаются и внутренние, и внешние LB).
1. `yandex.cpi.flant.com/loadbalancer-external` — включает создание внешнего (external) LB для данного Service (используйте, если нужно явно создать внешний балансировщик). Переопределяет поведение по умолчанию.
1. `yandex.cpi.flant.com/target-group-name-prefix` — задаёт префикс имени Target Group в формате `<значение аннотации><Yandex cluster name><NetworkID>` (для Service). Аналогичную аннотацию можно выставить на узле, чтобы включать узел в нестандартную Target Group (будут созданы TG с именами `<значение аннотации><Yandex cluster name><network id интерфейсов инстанса>`).

Если для управляющего слоя (control plane) или master-узлов создаются отдельные Target Group, добавьте на master-узлы метку `node.kubernetes.io/exclude-from-external-load-balancers: ""`. Это предотвратит попытки контроллера автоматически добавлять master-узлы в новые Target Group для балансировщиков.

Если вы создаёте собственный балансировщик для master-узлов и хотите, чтобы YCC также мог размещать свои балансировщики на master-узлах, заранее создайте Target Group с именем по маске `${CLUSTER-NAME}${VPC.ID}`.

### Проверки состояния Target Group

Параметры healthcheck’ов (для создаваемых LB Target Group):

1. `yandex.cpi.flant.com/healthcheck-interval-seconds` — как часто запускать проверку, в секундах (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-timeout-seconds` — сколько ждать ответа от эндпоинта, в секундах. Если за это время ответ не получен, проверка считается неуспешной (по умолчанию 1).
1. `yandex.cpi.flant.com/healthcheck-unhealthy-threshold` — сколько подряд неуспешных проверок нужно, чтобы пометить эндпоинт как неработоспособный (unhealthy) и исключить его из балансировки (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-healthy-threshold` — сколько подряд успешных проверок нужно, чтобы вернуть эндпоинт в статус работоспособный (healthy) и снова включить его в балансировку (по умолчанию 2).
