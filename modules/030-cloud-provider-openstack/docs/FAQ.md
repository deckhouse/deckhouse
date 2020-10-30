---
title: "Сloud provider — OpenStack: FAQ"
---

## Как настроить LoadBalancer?

**Внимание!!! Для корректного определения клиентского IP необходимо использовать LoadBalancer с поддержкой Proxy Protocol.**

### Пример IngressNginxController

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancerWithProxyProtocol
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.org/proxy-protocol: "true"
      loadbalancer.openstack.org/timeout-member-connect: "2000"
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
```

## Как настроить политики безопасности на узлах кластера?

Вариантов, зачем может понадобиться ограничить или наоборот расширить входящий или исходящий трафик на виртуальных
машинах кластера, может быть множество, например:

* Разрешить подключение к нодам кластера с виртуальных машин из другой подсети
* Разрешить подключение к портам статической ноды для работы приложения
* Ограничить доступ к внешним ресурсам или другим вм в облаке по требования службу безопасности

Для всего этого следует применять дополнительные security groups. Можно использовать только security groups, предварительно
созданные в облаке.

### Установка дополнительных security groups на мастерах и статических нодах

Данный параметр можно задать либо при создании кластера, либо в уже существующем кластере. В обоих случаях дополнительные
security groups указываются в `OpenStackClusterConfiguration`:

* Для мастеров — в секции `masterNodeGroup` в поле `additionalSecurityGroups`.
* Для статических нод — в секции `nodeGroups` в конфигурации, описывающей желаемую nodeGroup, также в поле `additionalSecurityGroups`.

Поле `additionalSecurityGroups` представляет собой массив строк с именами security groups.

### Установка дополнительных security groups на эфемерных нодах

Необходимо прописать параметр `additionalSecurityGroups` для всех OpenStackInstanceClass в кластере, которым нужны дополнительные
security groups. Смотри [параметры модуля cloud-provider-openstack](/modules/030-cloud-provider-openstack/configuration.html).

## Как поднять гибридный кластер?

Hybrid кластер представляет собой объединённые в один кластер bare metal ноды и ноды openstack. Для создания такого кластера
необходимо наличие L2 сети между всеми нодами кластера.

1. Удалить flannel из kube-system: `kubectl -n kube-system delete ds flannel-ds`;
2. Включите и [настройте](configuration.html#параметры) модуль.
3. Создайте один или несколько custom resource [OpenStackInstanceClass](cr.html#openstackinstanceclass).
4. Создайте один или несколько custom resource [NodeManager](/modules/040-node-manager/cr.html#nodegroup) для управления количеством и процессом заказа машин в облаке.

**Важно!** Cloud-controller-manager синхронизирует состояние между OpenStack и Kubernetes, удаляя из Kubernetes те узлы, которых нет в OpenStack. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому если узел Kubernetes запущен не с параметром `--cloud-provider=external`, то он автоматически игнорируется (Deckhouse прописывает `static://` в ноды в в `.spec.providerID`, а cloud-controller-manager такие узлы игнорирует).

### Параметры конфигурации

> **Внимание!** При изменении конфигурационных параметров приведенных в этой секции (параметров, указываемых в ConfigMap deckhouse) **перекат существующих Machines НЕ производится** (новые Machines будут создаваться с новыми параметрами). Перекат происходит только при изменении параметров `NodeGroup` и `OpenStackInstanceClass`. См. подробнее в документации модуля [node-manager](/modules/040-node-manager/faq.html#как-перекатить-эфемерные-машины-в-облаке-с-новой-конфигурацией).
Для настройки аутентификации с помощью модуля `user-authn` необходимо в Crowd'е проекта создать новое `Generic` приложение.

* `connection` - Параметры подключения к api cloud provider'a
  * `authURL` — OpenStack Identity API URL.
  * `caCert` — если OpenStack API имеет self-signed сертификат, можно указать CA x509 сертификат, использовавшийся для подписи.
    * Формат — строка. Сертификат в PEM формате.
    * Опциональный параметр.
  * `domainName` — имя домена.
  * `tenantName` — имя проекта.
    * Не может использоваться вместе с `tenantID`.
  * `tenantID` — id проекта.
    * Не может использоваться вместе с `tenantName`.
  * `username` — имя пользователя с полными правами на проект.
  * `password` — пароль к пользователю.
  * `region` — регион OpenStack, где будет развёрнут кластер.
* `internalNetworkNames` — имена сетей, подключённые к виртуальной машине, и используемые cloud-controller-manager для проставления InternalIP в `.status.addresses` в Node API объект.
  * Формат — массив строк. Например,

      ```yaml
      internalNetworkNames:
      - KUBE-3
      - devops-internal
      ```

* `externalNetworkNames` — имена сетей, подключённые к виртуальной машине, и используемые cloud-controller-manager для проставления ExternalIP в `.status.addresses` в Node API объект.
  * Формат — массив строк. Например,

      ```yaml
      externalNetworkNames:
      - KUBE-3
      - devops-internal
      ```

* `podNetworkMode` - определяет способ организации трафика в той сети, которая используется для коммуникации между подами (обычно это internal сеть, но бывают исключения).
  * Допустимые значение:
    * `DirectRouting` – между узлами работает прямая маршрутизация.
    * `DirectRoutingWithPortSecurityEnabled` - между узлами работает прямая маршрутизация, но только если в OpenStack явно разрешить на Port'ах диапазон адресов используемых во внутренней сети.
      * **Внимание!** Убедитесь, что у `username` есть доступ на редактирование AllowedAddressPairs на Port'ах, подключенных в сеть `internalNetworkName`. Обычно, в OpenStack, такого доступа нет, если сеть имеет флаг `shared`.
    * `VXLAN` – между узлами НЕ работает прямая маршрутизация, необходимо использовать VXLAN.
  * Опциональный параметр. По-умолчанию `DirectRoutingWithPortSecurityEnabled`.
* `instances` — параметры instances, которые используются при создании виртуальных машин:
  * `sshKeyPairName` — имя OpenStack ресурса `keypair`, который будет использоваться при заказе instances.
    * Обязательный парамер.
    * Формат — строкa.
  * `securityGroups` — Список securityGroups, которые нужно прикрепить к заказанным instances. Используется для задания firewall правил по отношению к заказываемым instances.
    * Опциональный параметр.
    * Формат — массив строк.
  * `imageName` - имя образа.
    * Опциональный параметр.
    * Формат — строкa.
  * `mainNetwork` - путь до network, которая будет подключена к виртуальной машине, как основная сеть (шлюз по умолчанию).
    * Опциональный параметр.
    * Формат — строкa.
  * `additionalNetworks` - список сетей, которые будут подключены к инстансу.
    * Опциональный параметр.
    * Формат — массив строк.
* `loadBalancer` - параметры Load Balancer
  * `subnetID` - ID Neutron subnet, в котором создать load balancer virtual IP.
    * Формат — строка.
    * Опциональный параметр.
  * `floatingNetworkID` - ID external network, который будет использоваться для заказа floating ip
    * Формат — строка.
    * Опциональный параметр.
* `zones` - список зон, в котором по умолчанию заказывать инстансы. Может быть переопределён индивидуально для каждой NodeGroup'ы
  * Формат — массив строк.
* `tags` - словарь тегов, которые будут на всех заказываемых инстансах
  * Опциональный параметр.
  * Формат — ключ-значение.

#### Пример

```yaml
cloudProviderOpenstack: |
  connection:
    authURL: https://test.tests.com:5000/v3/
    domainName: default
    tenantName: default
    username: jamie
    password: nein
    region: HetznerFinland
  externalNetworkNames:
  - public
  internalNetworkNames:
  - kube
  instances:
    sshKeyPairName: my-ssh-keypair
    securityGroups:
    - default
    - allow-ssh-and-icmp
  zones:
  - zone-a
  - zone-b
  tags:
    project: cms
    owner: default
```

### Подключение storage в гибридном кластере

Если вам требуются PersistentVolumes на нодах, подключаемых к кластеру из openstack, то необходимо создать StorageClass с нужным OpenStack volume type. Получить список типов можно командой `openstack volume type list`.
Например, для volume type `ceph-ssd`:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
provisioner: csi-cinderplugin # обязательно такой
parameters:
  type: ceph-ssd
volumeBindingMode: WaitForFirstConsumer
```

## Как загрузить image в OpenStack?

1. Скачиваем последний стабильный образ ubuntu 18.04

    ```shell
    curl -L https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img --output ~/ubuntu-18-04-cloud-amd64
    ```

2. Подготавливаем OpenStack RC (openrc) файл, который содержит credentials для обращения к api openstack.

    > Интерфейс получения openrc файла может отличаться в зависмости от провайдера OpenStack. Если провайдер предоставляет
    > стандартный интерфейс для OpenStack, то скачать openrc файл можно по [инструкции](https://docs.openstack.org/zh_CN/user-guide/common/cli-set-environment-variables-using-openstack-rc.html)

3. Либо устанавливаем OpenStack cli по [инструкции](https://docs.openstack.org/newton/user-guide/common/cli-install-openstack-command-line-clients.html).
   Либо можно запустить docker контейнер, прокинув внутрь openrc файл и скаченный локально образ ubuntu

    ```shell
    docker run -ti --rm -v ~/ubuntu-18-04-cloud-amd64:/ubuntu-18-04-cloud-amd64 -v ~/.mcs-openrc:/openrc jmcvea/openstack-client
    ```

4. Инициализируем переменные окружения из openrc файла

    ```shell
    source /openrc
    ```

5. Получаем список доступных типов дисков

    ```shell
    / # openstack volume type list
    +--------------------------------------+---------------+-----------+
    | ID                                   | Name          | Is Public |
    +--------------------------------------+---------------+-----------+
    | 8d39c9db-0293-48c0-8d44-015a2f6788ff | ko1-high-iops | True      |
    | bf800b7c-9ae0-4cda-b9c5-fae283b3e9fd | dp1-high-iops | True      |
    | 74101409-a462-4f03-872a-7de727a178b8 | ko1-ssd       | True      |
    | eadd8860-f5a4-45e1-ae27-8c58094257e0 | dp1-ssd       | True      |
    | 48372c05-c842-4f6e-89ca-09af3868b2c4 | ssd           | True      |
    | a75c3502-4de6-4876-a457-a6c4594c067a | ms1           | True      |
    | ebf5922e-42af-4f97-8f23-716340290de2 | dp1           | True      |
    | a6e853c1-78ad-4c18-93f9-2bba317a1d13 | ceph          | True      |
    +--------------------------------------+---------------+-----------+
    ```

6. Создаём image, передаём в образ в качестве свойств тип диска, который будет использоваться, если OpenStack не поддерживает локальные диски или эти диски не подходят для работы

    ```shell
    openstack image create --private --disk-format qcow2 --container-format bare --file /ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=dp1-high-iops ubuntu-18-04-cloud-amd64
    ```

7. Проверяем, что image успешно создан

    ```text
    / # openstack image show ubuntu-18-04-cloud-amd64
    +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
    | Field            | Value                                                                                                                                                                                                                                                                                    |
    +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
    | checksum         | 3443a1fd810f4af9593d56e0e144d07d                                                                                                                                                                                                                                                          |
    | container_format | bare                                                                                                                                                                                                                                                                                      |
    | created_at       | 2020-01-10T07:23:48Z                                                                                                                                                                                                                                                                      |
    | disk_format      | qcow2                                                                                                                                                                                                                                                                                     |
    | file             | /v2/images/01998f40-57cc-4ce3-9642-c8654a6d14fc/file                                                                                                                                                                                                                                      |
    | id               | 01998f40-57cc-4ce3-9642-c8654a6d14fc                                                                                                                                                                                                                                                      |
    | min_disk         | 0                                                                                                                                                                                                                                                                                         |
    | min_ram          | 0                                                                                                                                                                                                                                                                                         |
    | name             | ubuntu-18-04-cloud-amd64                                                                                                                                                                                                                                                                  |
    | owner            | bbf506e3ece54e21b2acf1bf9db4f62c                                                                                                                                                                                                                                                          |
    | properties       | cinder_img_volume_type='dp1-high-iops', direct_url='rbd://b0e441fc-c317-4acf-a606-cf74683978d2/images/01998f40-57cc-4ce3-9642-c8654a6d14fc/snap', locations='[{u'url': u'rbd://b0e441fc-c317-4acf-a606-cf74683978d2/images/01998f40-57cc-4ce3-9642-c8654a6d14fc/snap', u'metadata': {}}]' |
    | protected        | False                                                                                                                                                                                                                                                                                     |
    | schema           | /v2/schemas/image                                                                                                                                                                                                                                                                         |
    | size             | 343277568                                                                                                                                                                                                                                                                                 |
    | status           | active                                                                                                                                                                                                                                                                                    |
    | tags             |                                                                                                                                                                                                                                                                                           |
    | updated_at       | 2020-05-01T17:18:34Z                                                                                                                                                                                                                                                                      |
    | virtual_size     | None                                                                                                                                                                                                                                                                                      |
    | visibility       | private                                                                                                                                                                                                                                                                                   |
    +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
    ```
