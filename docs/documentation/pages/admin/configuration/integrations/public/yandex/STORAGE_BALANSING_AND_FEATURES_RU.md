---
title: Хранилище и балансировка
permalink: ru/admin/integrations/public/yandex/yandex-storage.html
lang: ru
---

Этот раздел охватывает дополнительные аспекты интеграции Deckhouse с Yandex Cloud:

- Подключение облачных дисков через CSI;
- Автоматическое создание StorageClass;
- Использование балансировщиков нагрузки;
- Особенности применения изменений;
- Работу с CloudStatic-узлами и bastion-хостами.

## Хранилище (CSI и StorageClass)

DKP обеспечивает интеграцию с блочным хранилищем Yandex Cloud через компонент CSI (Container Storage Interface). Это позволяет кластерам Deckhouse автоматически заказывать и подключать диски, а также использовать стандартные Kubernetes-ресурсы PersistentVolumeClaim для работы с хранилищем.

DKP автоматически создает ресурсы StorageClass для всех поддерживаемых типов дисков Yandex Cloud. Это позволяет пользователям сразу использовать хранилище, не создавая вручную описания классов.

Поддерживаются следующие типы дисков:

| Тип диска                 | Имя StorageClass          | Комментарии              |
|--------------------------|---------------------------|--------------------------|
| `network-hdd`            | `network-hdd`             | —                        |
| `network-ssd`            | `network-ssd`             | —                        |
| `network-ssd-nonreplicated` | `network-ssd-nonreplicated` | Размер кратен 93 ГБ      |
| `network-ssd-io-m3`      | `network-ssd-io-m3`       | Размер кратен 93 ГБ      |

> Размеры дисков `network-ssd-nonreplicated` и `network-ssd-io-m3` должны быть кратны 93 ГБ, иначе произойдёт ошибка при заказе тома.

### Исключение ненужных StorageClass

Если в кластере не планируется использовать определённые типы дисков, можно отключить автоматическое создание соответствующих StorageClass. Это делается с помощью параметра `settings.storageClass.exclude` в ресурсе ModuleConfig:

```yaml
settings:
  storageClass:
    exclude:
    - network-ssd-.*
    - network-hdd
```

В приведённом примере DKP не создаст StorageClass для всех `network-ssd` дисков и для `network-hdd`.

### Назначение StorageClass по умолчанию

По умолчанию Deckhouse выбирает StorageClass на основе аннотации `storageclass.kubernetes.io/is-default-class=true`.

Чтобы задать другой StorageClass по умолчанию, необходимо использовать глобальный параметр Deckhouse `global.defaultClusterStorageClass`. Изменить его можно следующей командой:

```console
kubectl edit mc global
```

Если параметр `defaultClusterStorageClass` не указан, платформа будет определять StorageClass, используемый по умолчанию, в следующем порядке:

- StorageClass с аннотацией `storageclass.kubernetes.io/is-default-class='true'` (если такой имеется в кластере).
- Первый StorageClass по алфавиту среди тех, что автоматически создаются облачным провайдером.
- По умолчанию значение параметра `defaultClusterStorageClass` — пустая строка ("").

## Балансировка нагрузки

### Внешний LoadBalancer

Deckhouse автоматически подписывается на Kubernetes-объекты `Service` с типом `LoadBalancer`. При их создании в кластере, он создаёт соответствующие ресурсы:

- `NetworkLoadBalancer` — сетевой балансировщик нагрузки в Yandex Cloud;
- `TargetGroup` — группа конечных точек для балансировки трафика.

Эти ресурсы позволяют Kubernetes-сервисам с типом `LoadBalancer` принимать входящий трафик из интернета или внутренних сетей в зависимости от настроек.

Подробнее об архитектуре см. в [в документации Kubernetes Cloud Controller Manager for Yandex Cloud](https://github.com/flant/yandex-cloud-controller-manager).

### Внутренний LoadBalancer

Для создания внутреннего балансировщика нагрузки (INTERNAL LoadBalancer), необходимо явно указать подсеть, в которой должен быть создан слушатель (listener) балансировщика.

Для этого добавьте следующую аннотацию в объект `Service`:

```yaml
metadata:
  annotations:
    yandex.cpi.flant.com/listener-subnet-id: <SubnetID>
```

Значение SubnetID — это ID подсети, в которой будет создан внутренний слушатель Yandex LoadBalancer. Использование этой аннотации позволяет контролировать сетевую доступность балансировщика, ограничивая его только внутренними адресами.

## Особенности применения изменений

Deckhouse не пересоздаёт уже существующие объекты Machine при изменении параметров.
Пересоздание узлов происходит только при изменении:

- параметров в секции NodeGroup;
- параметров YandexInstanceClass.

Это поведение позволяет избежать лишних операций и простоя существующих узлов, однако требует ручного вмешательства при необходимости пересоздать машины.

Если вы изменили объект YandexClusterConfiguration (например, изменили параметры провайдера, схемы размещения, подсетей и т.д.), чтобы изменения вступили в силу, выполните команду:

```console
dhctl converge
```

Команда инициирует пересчёт конфигурации и приводит текущее состояние кластера в соответствие с описанным в ресурсах.

## Интеграция вручную созданных ВМ

Платформа Deckhouse позволяет подключать существующие виртуальные машины в Yandex Cloud к Kubernetes-кластеру в качестве узлов. Такие узлы называются CloudStatic, поскольку они не управляются напрямую модулем `node-manager`, но могут использоваться в составе кластера.

Чтобы вручную подключить виртуальную машину в качестве CloudStatic-узла, необходимо:

1. Узнать актуальное значение `nodeNetworkCIDR` из кластера:

   ```console
   kubectl -n kube-system get secret d8-provider-cluster-configuration -o json | \
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

   Параметр `node-network-cidr` должен совпадать с тем значением, которое указано в объекте YandexClusterConfiguration, поле `nodeNetworkCIDR`.
