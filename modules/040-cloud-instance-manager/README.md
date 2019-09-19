# Модуль cloud-instance-manager

## Содержимое модуля

1. machine-controller-manager — контроллер для управления ресурсами public cloud из Kubernetes. Манипулирует custom-объектами:

    * MachineDeployments
    * MachineSets
    * Machines

2. cluster-autoscaler ([форк](https://github.com/gardener/autoscaler)) — управляет количеством реплик в MachineDeployment.
3. Набор хуков, создающих MachineDeployment и MachineClass объекты.

## Принцип работы

Хуки в cloud-instance-manager модуле создают MachineClass'ы, Secret'ы и MachineDeployment'ы, согласно настройкам `cloud-provider-` модуля, а также `CloudInstanceGroup` объекта.

machine-controller-manager, увидев новый MachineDeployment, создаёт MachineSet, а после создаёт Machines. Принцип работы практически идентичен Kubernetes Deployments, но с виртуалками вместов Pod'ов.

cluster-autoscaler манипулирует полем replicas в MachineDeployment.

## Конфигурация

### Включение модуля

Модуль включается автоматически при активации одного из `cloud-provider-` модулей.

### Параметры

* `instancePrefix` — префикс, который следует использовать при создании instances в cloud provider.
  * Опциональный параметр.

#### Пример конфигурации

```yaml
cloudInstanceManager:
  instancePrefix: kube
```

### CloudInstanceGroup custom resource

Ресурс описывает runtime параметры группы instances, которые будет использовать machine-controller-manager из этого модуля.

Все опции идут в `.spec`.

* `instanceClassReference` — ссылка на объект InstanceClass. Уникален для каждого `cloud-provider-` модуля.
    * `kind` — тип объекта (например, `OpenStackInstanceClass`). Тип объекта указан в документации соответствующего `cloud-provider-` модуля.
    * `name` — имя нужного InstanceClass объекта (например, `finland-medium`).
* `maxInstancesPerZone` — максимальное количество инстансов в зоне. Проставляется как верхняя граница в cluster-autoscaler.
* `minInstancesPerZone` — минимальное количество инстансов в зоне. Проставляется в объект MachineDeployment и в качестве нижней границы в cluster-autoscaler.
* `maxInstancesUnavailablePerZone` — сколько instances может быть недоступно при RollingUpdate'е MachineDeployment'а.
    * По-умолчанию `0`.
* `maxInstancesSurgePerZone` — сколько instances создавать одновременно при scale-up MachineDeployment'а.
    * По-умолчанию `1`.
* `zones` — уточнение списка зон. Обязательно должно быть подмножеством `zones` из конфигурации `cloud-provider-` модуля.
    * Формат — массив строк.
    * Опциональный параметр.
* `nodeTemplate` — настройки Node объектов в Kubernetes, который machine-controller-manager добавит после регистрации ноды.
    * `labels` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#objectmeta-v1-meta) `metadata.labels`

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

#### Пример CloudInstanceGroup

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: test
spec:
  minInstancesPerZone: 1
  maxInstancesPerZone: 3
  instanceClassReference:
    kind: GCPInstanceClass
    name: test
  nodeTemplate:
    labels:
      dedicated: web
```

## Как мне начать заказывать машины

1. Настройте один из поддерживаемых `cloud-provider-` модулей:

    * [GCP](modules/030-cloud-provider-gcp/README.md)
    * [OpenStack](modules/030-cloud-provider-gcp/README.md)

2. [Настройте](#параметры) модуль.
3. Создайте `CloudInstanceGroup` с желаемыми [параметрами](#CloudInstanceGroup-custom-resource) InstanceGroup.
