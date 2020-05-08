# Модуль node-manager

## Содержимое модуля

1. machine-controller-manager — контроллер для управления ресурсами public cloud из Kubernetes. Манипулирует custom-объектами:

    * MachineDeployments
    * MachineSets
    * Machines

2. cluster-autoscaler ([форк](https://github.com/gardener/autoscaler)) — управляет количеством реплик в MachineDeployment.
3. Набор хуков, создающих MachineDeployment и MachineClass объекты.

## Принцип работы

Хуки в node-manager модуле создают MachineClass'ы, Secret'ы и MachineDeployment'ы, согласно настройкам `cloud-provider-` модуля, а также `NodeGroup` объекта.

machine-controller-manager, увидев новый MachineDeployment, создаёт MachineSet, а после создаёт Machines. Принцип работы практически идентичен Kubernetes Deployments, но с виртуалками вместо Pod'ов.

cluster-autoscaler манипулирует полем replicas в MachineDeployment.

## Конфигурация

### Включение модуля

Модуль включается автоматически при активации одного из `cloud-provider-` модулей.

### Параметры

* `instancePrefix` — префикс, который следует использовать при создании instances в cloud provider.
  * Опциональный параметр.

#### Пример конфигурации

```yaml
nodeManager: |
  instancePrefix: kube
```

### NodeGroup custom resource

Ресурс описывает runtime параметры группы нод, которые будет использовать machine-controller-manager из этого модуля.

Все опции идут в `.spec`.

* `nodeType` — Static, Cloud или Hybrid. TODO: подробнее?
* `allowDisruptions` — обновлять ли ноду автоматически при обновлении bashible.
  * Формат — boolean.
  * По-умолчанию, `true`.
* `kubernetesVersion` — желаемая minor версия Kubernetes.
  * Например, `1.16`.
* `static` — **Внимание!** использовать только с `nodeType: Static`
  * `internalNetworkCIDRs` — список подсетей, использующиеся для коммуникации внутри кластера.
    * Формат — массив строк. Subnet CIDR.
    * Пример:

      ```yaml
      internalNetworkCIDRs:
      - "10.2.2.3/24"
      - "10.1.1.1/24"
      ```

* `cloudInstances` — ссылка на объект InstanceClass. Уникален для каждого `cloud-provider-` модуля.
  * `classReference`
    * `kind` — тип объекта (например, `OpenStackInstanceClass`). Тип объекта указан в документации соответствующего `cloud-provider-` модуля.
    * `name` — имя нужного InstanceClass объекта (например, `finland-medium`).
  * `maxPerZone` — максимальное количество инстансов в зоне. Проставляется как верхняя граница в cluster-autoscaler.
  * `minPerZone` — минимальное количество инстансов в зоне. Проставляется в объект MachineDeployment и в качестве нижней границы в cluster-autoscaler.
  * `maxUnavailablePerZone` — сколько нод может быть недоступно при RollingUpdate'е MachineDeployment'а.
    * По-умолчанию `0`.
  * `maxSurgePerZone` — сколько нод создавать одновременно при scale-up MachineDeployment'а.
    * По-умолчанию `1`.
  * `zones` — уточнение списка зон. Обязательно должно быть подмножеством `zones` из конфигурации `cloud-provider-` модуля.
    * Формат — массив строк.
    * Опциональный параметр.
* `nodeTemplate` — настройки Node объектов в Kubernetes, который machine-controller-manager добавит после регистрации ноды.
  * `labels` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) `metadata.labels`
    * Пример:

      ```yaml
      labels:
        environment: production
        app: warp-drive-ai

  * `annotations` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) `metadata.annotations`
    * Пример:

      ```yaml
      annotations:
        ai.fleet.com/discombobulate: "true"
      ```

  * `taints` — аналогично полю `.spec.taints` из объекта [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#taint-v1-core). **Внимание!** Доступны только поля `effect`, `key`, `values`.
    * Пример:

      ```yaml
      taints:
      - effect: NoExecute
        key: ship-class
        value: frigate
      ```

* `chaos` — настройки chaos monkey для данной IG:
  * Опциональный параметр.
  * `mode` — режим работы chaos monkey, возможные значения: `DrainAndDelete` — при срабатывании drain'ит и удаляет ноду, `Disabled` — не трогает данную IG.
    * По-умолчанию `DrainAndDelete`.
  * `period` — в какой интервал времени сработает chaos monkey (указывать можно в [golang формате](https://golang.org/pkg/time/#ParseDuration));
    * По-умолчанию `6h`.

#### Пример NodeGroup

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: Cloud
  kubernetesVersion: "1.16"
  cloudInstances:
    zones:
    - eu-west-1a
    - eu-west-1b
    minPerZone: 1
    maxPerZone: 3
    maxUnavailablePerZone: 0
    maxSurgePerZone: 1
    classReference:
      kind: AWSInstanceClass
      name: test
  nodeTemplate:
    labels:
      environment: production
      app: warp-drive-ai
    annotations:
      ai.fleet.com/discombobulate: "true"
    taints:
    - effect: NoExecute
      key: ship-class
      value: frigate
  chaos:
    mode: DrainAndReboot
    period: 24h
  allowDisruptions: false
```

## Как мне начать заказывать машины?

1. Настройте один из поддерживаемых `cloud-provider-` модулей:

    * [AWS](modules/030-cloud-provider-aws/README.md)
    * [GCP](modules/030-cloud-provider-gcp/README.md)
    * [OpenStack](modules/030-cloud-provider-openstack/README.md)
    * [vSphere](modules/030-cloud-provider-vsphere/README.md)
    * [Yandex](modules/030-cloud-provider-yandex/README.md)

2. [Настройте](#параметры) модуль.
3. Создайте `NodeGroup` с желаемыми [параметрами](#NodeGroup-custom-resource) NodeGroup.

## Как мне перекатить машины с новой конфигурацией?

При изменении конфигурации Deckhouse машины не перекатятся. Перекат происходит только после изменения `InstanceClass` или `NodeGroup` объектов.

Для того, чтобы форсированно перекатить все Machines, следует добавить/изменить аннотацию `manual-rollout-id` в `NodeGroup`: `kubectl annotate NodeGroup имя_ng "manual-rollout-id=$(uuidgen)" --overwrite`.
