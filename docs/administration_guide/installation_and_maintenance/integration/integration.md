---
title: "Cloud provider — AWS: примеры"
---

## Пример custom resource `AWSInstanceClass`

Ниже представлен простой пример конфигурации custom resource `AWSInstanceClass`:

```yaml
apiVersion: deckhouse.io/v1
kind: AWSInstanceClass
metadata:
  name: worker
spec:
  instanceType: t3.large
  ami: ami-040a1551f9c9d11ad
  diskSizeGb: 15
  diskType:  gp2
```

## LoadBalancer

### Аннотации объекта Service

Поддерживаются следующие параметры в дополнение к существующим в upstream:

1. `service.beta.kubernetes.io/aws-load-balancer-type` — может иметь значение `none`, что приведет к созданию **только** Target Group, без какого-либо LoadBalanacer'а.
2. `service.beta.kubernetes.io/aws-load-balancer-backend-protocol` — используется в связке с `service.beta.kubernetes.io/aws-load-balancer-type: none`:
   * Возможные значения:
     * `tcp` (по умолчанию);
     * `tls`;
     * `http`;
     * `https`.
   * **Внимание!** При изменении этого параметра `cloud-controller-manager` попытается пересоздать Target Group. Если к ней уже привязаны NLB или ALB, удалить Target Group не получится и он будет бесконечно пытаться это сделать. В таком случае необходимо вручную отсоединить NLB или ALB от Target Group.

## Настройка политик безопасности на узлах

Вариантов, зачем может понадобиться ограничить или, наоборот, расширить входящий или исходящий трафик на виртуальных машинах кластера в AWS, может быть множество. Например:

* Разрешить подключение к узлам кластера с виртуальных машин из другой подсети.
* Разрешить подключение к портам статического узла для работы приложения.
* Ограничить доступ к внешним ресурсам или другим виртуальным машинам в облаке по требованию службы безопасности.

Для всех них следует применять дополнительные группы безопасности (security group). Можно использовать только предварительно созданные в облаке группы безопасности.

## Установка дополнительных security groups на статических и master-узлах

Данный параметр можно задать либо при создании кластера, либо в уже существующем кластере. В обоих случаях дополнительные группы безопасности (security group) указываются в `AWSClusterConfiguration`:
- для master-узлов — в секции `masterNodeGroup` в поле `additionalSecurityGroups`;
- для статических узлов — в секции `nodeGroups` в конфигурации, описывающей соответствующую nodeGroup, в поле `additionalSecurityGroups`.

Поле `additionalSecurityGroups` содержит массив строк с именами групп безопасности.

## Установка дополнительных security groups на эфемерных узлах

Необходимо указать параметр `additionalSecurityGroups` для всех [`AWSInstanceClass`](cr.html#awsinstanceclass) в кластере, которым нужны дополнительные группы безопасности (security group).

## Настройка балансировщика в случае наличия Ingress-узлов не во всех зонах

Необходимо указать аннотацию на объекте Service: `service.beta.kubernetes.io/aws-load-balancer-subnets: subnet-foo, subnet-bar`.

Чтобы получить список текущих подсетей, используемых для конкретной установки, выполните следующую команду:

```bash
kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller module values cloud-provider-aws -o json \
| jq -r '.cloudProviderAws.internal.zoneToSubnetIdMap'
```
---
title: "Cloud provider — AWS: FAQ"
---


## Как поднять пиринговое соединение  между VPC?

Для примера рассмотрим настройку пирингового соединения между двумя условными VPC — vpc-a и vpc-b.

> **Важно!** IPv4 CIDR у обоих VPC должен различаться.

Для настройки выполните следующие шаги:

1. Перейдите в регион, где работает vpc-a.
1. Нажмите `VPC` -> `VPC Peering Connections` -> `Create Peering Connection` и настройте пиринговое соединение:
   * Name: `vpc-a-vpc-b`.
   * Заполните `Local` и `Another VPC`.
1. Перейдите в регион, где работает vpc-b.
1. Нажмите `VPC` -> `VPC Peering Connections`.
1. Выделите созданное соединение и выберите `Action "Accept Request"`.
1. Для vpc-a добавьте во все таблицы маршрутизации маршруты до CIDR vpc-b через пиринговое соединение.
1. Для vpc-b добавьте во все таблицы маршрутизации маршруты до CIDR vpc-a через пиринговое соединение.

## Как создать кластер в новом VPC с доступом через имеющийся bastion-хост?

1. Выполнить bootstrap базовой инфраструктуры кластера:

   ```shell
   dhctl bootstrap-phase base-infra --config config
   ```

2. Поднять пиринговое соединение по инструкции [выше](#как-поднять-пиринговое-соединение--между-vpc).

3. Продолжить установку кластера. На вопрос про кэш Terraform ответить `y`:

   ```shell
   dhctl bootstrap --config config --ssh-...
   ```

## Как создать кластер в новом VPC и развернуть bastion-хост для доступа к узлам?

1. Выполнить bootstrap базовой инфраструктуры кластера:

   ```shell
   dhctl bootstrap-phase base-infra --config config
   ```

2. Запустить вручную bastion-хост в subnet <prefix>-public-0.

3. Продолжить установку кластера. На вопрос про кэш Terraform ответить `y`:

   ```shell
   dhctl bootstrap --config config --ssh-...
   ```

## Особенности настройки bastion

Поддерживаются сценарии:
* bastion-хост уже создан во внешней VPC:
  1. Создайте базовую инфраструктуру кластера — `dhctl bootstrap-phase base-infra`.
  1. Настройте пиринговое соединение между внешней и свежесозданной VPC.
  1. Продолжите установку с указанием bastion-хоста — `dhctl bootstrap --ssh-bastion...`.
* bastion-хост требуется поставить в свежесозданной VPC:
  1. Создайте базовую инфраструктуру кластера — `dhctl bootstrap-phase base-infra`.
  1. Запустите вручную bastion-хост в subnet <prefix>-public-0.
  1. Продолжите установку с указанием bastion-хоста — `dhctl bootstrap --ssh-bastion...`.

## Добавление CloudStatic-узлов в кластер

Для добавления виртуальной машины в качестве узла в кластер выполните следующие шаги:
1. Прикрепите группу безопасности `<prefix>-node`.
1. Укажите следующие теги у виртуальной машины (чтобы `cloud-controller-manager` смог найти виртуальные машины в облаке):

   ```text
   "kubernetes.io/cluster/<cluster_uuid>" = "shared"
   "kubernetes.io/cluster/<prefix>" = "shared"
   ```

   * Узнать `cluster_uuid` можно с помощью команды:

     ```shell
     kubectl -n kube-system get cm d8-cluster-uuid -o json | jq -r '.data."cluster-uuid"'
     ```

   * Узнать `prefix` можно с помощью команды:

     ```shell
     kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' \
       | base64 -d | grep prefix
     ```

## Как увеличить размер volume в кластере?

Задайте новый размер в соответствующем ресурсе PersistentVolumeClaim в параметре `spec.resources.requests.storage`.

Операция проходит полностью автоматически и занимает до одной минуты. Никаких дополнительных действий не требуется.

За ходом процесса можно наблюдать в events через команду `kubectl describe pvc`.

> После изменения volume нужно подождать не менее шести часов и убедиться, что volume находится в состоянии `in-use` или `available`, прежде чем станет возможно изменить его еще раз. Подробности можно найти [в официальной документации](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/modify-volume-requirements.html).

---
title: "Cloud provider — Azure: примеры"
---

## Пример custom resource `AzureInstanceClass`

Ниже представлен простой пример custom resource `AzureInstanceClass`:

```yaml
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: example
spec:
  machineSize: Standard_F4
```

---
title: "Cloud provider — GCP: примеры"
---

## Пример custom resource `GCPInstanceClass`

Ниже представлен простой пример конфигурации custom resource `GCPInstanceClass` :

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: test
spec:
  machineType: n1-standard-1
```

## Настройка политик безопасности на узлах

Вариантов, зачем может понадобиться ограничить или, наоборот, расширить входящий или исходящий трафик на виртуальных машинах кластера в GCP, может быть множество. Например:

* Разрешить подключение к узлам кластера с виртуальных машин из другой подсети.
* Разрешить подключение к портам статического узла для работы приложения.
* Ограничить доступ к внешним ресурсам или другим виртуальным машинам в облаке по требованию службы безопасности.

Для всего этого следует применять дополнительные network tags.

## Установка дополнительных network tags на статических и master-узлах

Данный параметр можно задать либо при создании кластера, либо в уже существующем кластере. В обоих случаях дополнительные network tags указываются в `GCPClusterConfiguration`:
- для master-узлов — в секции `masterNodeGroup` в поле `additionalNetworkTags`;
- для статических узлов — в секции `nodeGroups` в конфигурации, описывающей соответствующую nodeGroup, в поле `additionalNetworkTags`.

Поле `additionalNetworkTags` содержит массив строк с именами network tags.

## Установка дополнительных network tags на эфемерных узлах

Необходимо указать параметр `additionalNetworkTags` для всех [`GCPInstanceClass`](cr.html#gcpinstanceclass) в кластере, которым нужны дополнительные network tags.

---
title: "Cloud provider — GCP: FAQ"
---

## Как поднять кластер

1. Настройте облачное окружение.
2. Включите модуль или передайте флаг `--extra-config-map-data base64_encoding_of_custom_config` [с параметрами модуля](configuration.html) в скрипт установки `install.sh`.
3. Создайте один или несколько custom resource [GCPInstanceClass](cr.html#gcpinstanceclass).
4. Создайте один или несколько custom resource [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup) для управления количеством и процессом заказа машин в облаке.

## Добавление CloudStatic узлов в кластер

К виртуальным машинам, которые вы хотите добавить к кластеру в качестве узлов, добавьте `Network Tag`, аналогичный префиксу кластера.

Префикс кластера можно узнать, воспользовавшись следующей командой:

```shell
kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' \
  | base64 -d | grep prefix
```

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

---
title: "Cloud provider — Yandex Cloud: FAQ"
---

## Как настроить INTERNAL LoadBalancer?

Для настройки INTERNAL LoadBalancer'а установите аннотацию для сервиса:

```yaml
yandex.cpi.flant.com/listener-subnet-id: SubnetID
```

Аннотация указывает, какой subnet будет слушать LoadBalancer.

## Как зарезервировать публичный IP-адрес?

Для использования в `externalIPAddresses` и `natInstanceExternalAddress` выполните следующую команду:

```shell
$ yc vpc address create --external-ipv4 zone=ru-central1-a
id: e9b4cfmmnc1mhgij75n7
folder_id: b1gog0h9k05lhqe5d88l
created_at: "2020-09-01T09:29:33Z"
external_ipv4_address:
  address: 178.154.226.159
  zone_id: ru-central1-a
  requirements: {}
reserved: true
```

## Проблемы `dhcpOptions` и пути их решения

Использование в настройках DHCP-серверов адресов DNS, отличающихся от предоставляемых Yandex Cloud, является временным решением. От него можно будет отказаться, когда Yandex Cloud введет услугу Managed DNS. Чтобы обойти ограничения, описанные ниже, рекомендуется использовать `stubZones` из модуля [`kube-dns`](../042-kube-dns/)

### Изменение параметров

Обратите внимание на следующие особенности:

1. При изменении данных параметров требуется выполнить `netplan apply` или аналог, форсирующий обновление DHCP lease.
2. Потребуется перезапуск всех подов hostNetwork (особенно `kube-dns`), чтобы перечитать новый `resolv.conf`.

### Особенности использования

При использовании опции `dhcpOptions` все DNS-запросы начнут идти через указанные DNS-серверы. Эти DNS-серверы **должны** разрешать внешние DNS-имена, а также при необходимости разрешать DNS-имена внутренних ресурсов.

**Не используйте** эту опцию, если указанные рекурсивные DNS-серверы не могут разрешать тот же список зон, что сможет разрешать рекурсивный DNS-сервер в подсети Yandex Cloud.

## Как назначить произвольный StorageClass используемым по умолчанию?

Чтобы назначить произвольный StorageClass используемым по умолчанию, выполните следующие шаги:

1. Добавьте на StorageClass аннотацию `storageclass.kubernetes.io/is-default-class='true'`:

   ```shell
   kubectl annotate sc $STORAGECLASS storageclass.kubernetes.io/is-default-class='true'
   ```

2. Укажите имя StorageClass'а в параметре [storageClass.default](configuration.html#parameters-storageclass-default) в настройках модуля `cloud-provider-yandex`. Обратите внимание, что после этого аннотация `storageclass.kubernetes.io/is-default-class='true'` снимется со StorageClass'а, который ранее был указан в настройках модуля как используемый по умолчанию.

   ```shell
   kubectl edit mc cloud-provider-yandex
   ```

## Добавление CloudStatic-узлов в кластер

В метаданные виртуальных машин, которые вы хотите включить в кластер в качестве узлов, добавьте (Изменить ВМ -> Метадата) ключ `node-network-cidr` со значением `nodeNetworkCIDR` для кластера.

`nodeNetworkCIDR` кластера можно узнать, воспользовавшись следующей командой:

```shell
kubectl -n kube-system get secret d8-provider-cluster-configuration -o json | jq --raw-output '.data."cloud-provider-cluster-configuration.yaml"' | base64 -d | grep '^nodeNetworkCIDR'
```
