---
title: "Модуль cloud-provider-openstack"
---

## Содержимое модуля

1. `cloud-controller-manager` — контроллер для управления ресурсами OpenStack из Kubernetes.
    1. Создаёт LoadBalancer'ы для Service-объектов Kubernetes с типом `LoadBalancer`.
    2. Синхронизирует метаданные OpenStack Servers и Kubernetes Nodes. Удаляет из Kubernetes ноды, которых более нет в OpenStack.
2. CSI storage — для заказа дисков в Cinder (block). Manilla (filesystem) пока не поддерживается.
3. Регистрация в модуле [node-manager]({{ site.baseurl }}/modules/040-node-manager/), чтобы [OpenStackInstanceClass'ы](#openstackinstanceclass-custom-resource) можно было использовать в [CloudInstanceClass'ах]({{ site.baseurl }}/modules/040-node-manager/#nodegroup-custom-resource).


## Конфигурация

### Включение модуля

Модуль автоматически включается для всех облачных кластеров развёрнутых в OpenStack.

### Параметры
Настройки модуля устанавливаются автоматически на основании [выбранной схемы размещения]({{ site.baseurl }}/candi/). В
большинстве случаев нет необходимости в ручной конфигурации модуля.

Если вам необходимо настроить модуль, потому что, например, у вас bare metal кластер, для которого нужно включить
возможность добавлять дополнительные инстансы из OpenStack, то смотрите раздел как [настроить Hybrid кластер в OpenStack](hybrid_cluster.html).

Если у вас в кластере есть инстансы, для которых будут использоваться External Networks, кроме указанных в схеме размещения,
то их следует передавать в параметре

* `additionalExternalNetworkNames` — имена дополнительных сетей, которые могут быть подключены к виртуальной машине, и используемые `cloud-controller-manager` для проставления `ExternalIP` в `.status.addresses` в Node API объект.
    * Формат — массив строк.

#### Пример конфигурации

```yaml
cloudProviderOpenstack: |
  additionalExternalNetworkNames:
  - some-bgp-network
```

### Заказ нод в кластере

Управляйте количеством и процессом заказа машин в облаке с помощью модуля [node-manager]({{ site.baseurl }}/modules/040-node-manager/).

#### OpenStackInstanceClass custom resource

Ресурс описывает параметры группы OpenStack servers, которые будет использовать `machine-controller-manager` из модуля [node-manager]({{ site.baseurl }}/modules/040-node-manager/). На этот ресурс ссылается ресурс `CloudInstanceClass` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `flavorName` — тип заказываемых server'ов
* `imageName` — имя образа.
    * **Внимание!** Сейчас поддерживается и тестируется только `Ubuntu 18.04`.
    * Увидеть список всех доступных образов можно найти командой: `openstack image list`
* `rootDiskSize` — если параметр присутствует, OpenStack server будет создан на Cinder volume с указанным размером и стандартным для кластера типом.
    * Опциональный параметр.
    * Формат — integer. В гигабайтах.
    > Если в *cloud provider* существует несколько типов дисков, то для выбора конкретного типа диска виртуальной машины у используемого образа можно установить тип диска по-умолчанию, для этого необходимо в метаданных образа указать имя определённого типа диска
    > Для этого также может понадобиться создать свой собственный image в OpenStack, как это сделать описано в разделе ["Загрузка image в OpenStack"](upload_image.html)
      > ```bash
        openstack volume type list
        openstack image set ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=VOLUME_NAME
        ```

* `mainNetwork` — путь до network, которая будет подключена к виртуальной машине, как основная сеть (шлюз по-умолчанию).
* `additionalNetworks` - список сетей, которые будут подключены к инстансу.
    * Опциональный параметр.
    * Формат — массив строк.
    * Пример:

      ```yaml
      - enp6t4snovl2ko4p15em
      - enp34dkcinm1nr5999lu
      ```
* `additionalSecurityGroups` — Список `securityGroups`, которые необходимо прикрепить к instances `OpenStackInstanceClass` в дополнение к указанным в конфигурации cloud провайдера. Используется для задания firewall правил по отношению к заказываемым instances.
    * Опциональный параметр.
    * Формат — массив строк.
    * Пример:

      ```yaml
      - sec_group_1
      - sec_group_2
      ```

##### Пример OpenStackInstanceClass

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: OpenStackInstanceClass
metadata:
  name: test
spec:
  flavorName: m1.large
  imageName: ubuntu-18-04-cloud-amd64
  mainNetwork: kube
```

#### LoadBalancer
**Внимание!!! На данный момент в OpenStack при заказе loadbalancer не определяется правильный клиентский IP.**

##### Пример IngressNginxController

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: main
spec:
  controllerVersion: "0.26"
  ingressClass: nginx
  inlet: LoadBalancer
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
```
