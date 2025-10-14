---
title: Хранилище и балансировка
permalink: ru/admin/integrations/public/yandex/storage.html
lang: ru
---

Этот раздел охватывает дополнительные аспекты интеграции Deckhouse Kubernetes Platform (DKP) с Yandex Cloud:

- подключение облачных дисков через CSI;
- автоматическое создание StorageClass;
- использование балансировщиков нагрузки;
- особенности применения изменений;
- работу с CloudStatic-узлами и bastion-хостами.

## Хранилище (CSI и StorageClass)

DKP обеспечивает интеграцию с блочным хранилищем Yandex Cloud через компонент Container Storage Interface (CSI). Это даёт возможность кластерам DKP автоматически заказывать и подключать диски, а также использовать стандартные Kubernetes-ресурсы PersistentVolumeClaim для работы с хранилищем.

DKP автоматически создает ресурсы StorageClass для всех поддерживаемых типов дисков Yandex Cloud. Это делает возможным для всех пользователей сразу использовать хранилище, не создавая вручную описания классов.

Поддерживаются следующие типы дисков:

| Тип диска                 | Имя StorageClass          | Комментарии              |
|--------------------------|---------------------------|--------------------------|
| `network-hdd`            | `network-hdd`             | —                        |
| `network-ssd`            | `network-ssd`             | —                        |
| `network-ssd-nonreplicated` | `network-ssd-nonreplicated` | Размер кратен 93 ГБ      |
| `network-ssd-io-m3`      | `network-ssd-io-m3`       | Размер кратен 93 ГБ      |

{% alert level="info" %}
Размеры дисков `network-ssd-nonreplicated` и `network-ssd-io-m3` должны быть кратны 93 ГБ, иначе произойдёт ошибка при заказе тома.
{% endalert %}

### Исключение ненужных StorageClass

Если в кластере не планируется использовать определённые типы дисков, можно отключить автоматическое создание соответствующих StorageClass. Это делается с помощью [параметра `settings.storageClass.exclude`](/modules/cloud-provider-yandex/configuration.html#parameters-storageclass-exclude) в ресурсе ModuleConfig:

```yaml
settings:
  storageClass:
    exclude:
    - network-ssd-.*
    - network-hdd
```

В приведённом примере DKP не создаст StorageClass для всех `network-ssd` дисков и для `network-hdd`.

### Назначение StorageClass по умолчанию

По умолчанию DKP выбирает StorageClass на основе аннотации `storageclass.kubernetes.io/is-default-class=true`.

Чтобы задать другой StorageClass по умолчанию, необходимо использовать [глобальный параметр DKP `global.defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass). Изменить его можно следующей командой:

```shell
d8 k edit mc global
```

Если параметр `defaultClusterStorageClass` не указан, платформа будет определять StorageClass, используемый по умолчанию, в следующем порядке:

- StorageClass с аннотацией `storageclass.kubernetes.io/is-default-class='true'` (если такой имеется в кластере).
- Первый StorageClass по алфавиту среди тех, что автоматически создаются облачным провайдером.
- По умолчанию значение параметра `defaultClusterStorageClass` — пустая строка (`""`).

## Балансировка нагрузки

### Внешний LoadBalancer

DKP автоматически подписывается на Kubernetes-объекты Service с типом LoadBalancer. При их создании в кластере, он создаёт соответствующие ресурсы:

- **NetworkLoadBalancer** — сетевой балансировщик нагрузки в Yandex Cloud;
- **TargetGroup** — группа конечных точек для балансировки трафика.

Эти ресурсы предоставляют Kubernetes-сервисам с типом LoadBalancer возможность принимать входящий трафик из интернета или внутренних сетей в зависимости от настроек.

Подробнее об архитектуре см. в [в документации Kubernetes Cloud Controller Manager for Yandex Cloud](https://github.com/flant/yandex-cloud-controller-manager).

### Внутренний LoadBalancer

Чтобы создать внутренний балансировщик (INTERNAL LoadBalancer), укажите подсеть, в которой должен быть создан listener:

Для этого добавьте следующую аннотацию в объект Service:

```yaml
metadata:
  annotations:
    yandex.cpi.flant.com/listener-subnet-id: <SubnetID>
```

Значение `SubnetID` — это ID подсети, в которой будет создан внутренний слушатель Yandex LoadBalancer. Использование этой аннотации даёт возможность контролировать сетевую доступность балансировщика, ограничивая его только внутренними адресами.

> Поведение по умолчанию (внешний или внутренний LB) зависит от конфигурации кластера. Для явного выбора типа используйте аннотацию `yandex.cpi.flant.com/loadbalancer-external`.

#### Аннотации объекта Service

В кластере заданы значения по умолчанию для размещения ресурсов балансировщиков (сеть для Target Group и подсеть для Listener). Эти значения выставляются автоматически во время развёртывания кластера и могут быть переопределены аннотациями на уровне конкретного Service.

В Yandex Cloud Controller Manager поддерживаются следующие аннотации:

1. `yandex.cpi.flant.com/target-group-network-id` — указывает NetworkID, в котором будет создана Target Group для данного Service. Переопределяет соответствующее значение по умолчанию.
1. `yandex.cpi.flant.com/listener-subnet-id` — задаёт SubnetID для Listener’ов создаваемого LB для данного Service. Переопределяет соответствующее значение по умолчанию.
1. `yandex.cpi.flant.com/listener-address-ipv4` — задаёт предопределённый IPv4-адрес для Listener’ов (поддерживаются и внутренние, и внешние LB).
1. `yandex.cpi.flant.com/loadbalancer-external` — включает создание внешнего (external) LB для данного Service (используйте, если нужно явно создать внешний балансировщик). Переопределяет поведение по умолчанию.
1. `yandex.cpi.flant.com/target-group-name-prefix` — задаёт префикс имени Target Group в формате `<значение аннотации><Yandex cluster name><NetworkID>` (для Service). Аналогичную аннотацию можно выставить на узле, чтобы включать узел в нестандартную Target Group (будут созданы TG с именами `<значение аннотации><Yandex cluster name><network id интерфейсов инстанса>`).

Если для управляющего слоя (control plane) или master-узлов создаются отдельные Target Group, добавьте на master-узлы метку `node.kubernetes.io/exclude-from-external-load-balancers: ""`. Это предотвратит попытки контроллера автоматически добавлять master-узлы в новые Target Group для балансировщиков.
Если вы создаёте собственный балансировщик для master-узлов и хотите, чтобы YCC также мог размещать свои балансировщики на master-узлах, заранее создайте Target Group с именем по маске `${CLUSTER-NAME}${VPC.ID}`.

#### Проверки состояния Target Group

Параметры healthcheck’ов (для создаваемых LB Target Group):

1. `yandex.cpi.flant.com/healthcheck-interval-seconds` — как часто запускать проверку, в секундах (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-timeout-seconds` — сколько ждать ответа от эндпоинта, в секундах. Если за это время ответ не получен, проверка считается неуспешной (по умолчанию 1).
1. `yandex.cpi.flant.com/healthcheck-unhealthy-threshold` — сколько подряд неуспешных проверок нужно, чтобы пометить эндпоинт как неработоспособный (unhealthy) и исключить его из балансировки (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-healthy-threshold` — сколько подряд успешных проверок нужно, чтобы вернуть эндпоинт в статус работоспособный (healthy) и снова включить его в балансировку (по умолчанию 2).

## Особенности применения изменений

DKP не пересоздаёт уже существующие объекты Machine при изменении параметров.
Пересоздание узлов происходит только при изменении:

- параметров в [секции NodeGroup](/modules/node-manager/cr.html#nodegroup);
- [параметров YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass).

Это поведение помогает избежать лишних операций и простоя существующих узлов, однако требует ручного вмешательства при необходимости пересоздать машины.

Если вы изменили [объект YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) (например, изменили параметры провайдера, схемы размещения, подсетей и т.д.), чтобы изменения вступили в силу, выполните команду:

```shell
dhctl converge
```

Команда инициирует пересчёт конфигурации и приводит текущее состояние кластера в соответствие с описанным в ресурсах.

## Интеграция вручную созданных ВМ

DKP позволяет подключать существующие виртуальные машины в Yandex Cloud к Kubernetes-кластеру в качестве узлов. Такие узлы называются CloudStatic, поскольку они не управляются напрямую [модулем `node-manager`](/modules/node-manager/), но могут использоваться в составе кластера.

Чтобы вручную подключить виртуальную машину в качестве CloudStatic-узла, необходимо:

1. Узнать актуальное значение `nodeNetworkCIDR` из кластера:

   ```shell
   d8 k -n kube-system get secret d8-provider-cluster-configuration -o json | \
     jq --raw-output '.data."cloud-provider-cluster-configuration.yaml"' | base64 -d | grep '^nodeNetworkCIDR'
   ```

   Результатом будет строка вида:

   ```console
   nodeNetworkCIDR: 192.168.12.13/24
   ```

   Это значение необходимо скопировать и указать как `value` в метаданных виртуальной машины.

1. Задать параметр `node-network-cidr` в метаданных ВМ:

   ```yaml
   key: node-network-cidr
   value: <nodeNetworkCIDR из кластера>
   ```

   Параметр `node-network-cidr` должен совпадать с тем значением, которое указано в объекте YandexClusterConfiguration, [поле `nodeNetworkCIDR`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-nodenetworkcidr).
