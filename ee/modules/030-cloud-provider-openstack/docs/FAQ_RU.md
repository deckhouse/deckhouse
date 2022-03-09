---
title: "Cloud provider — OpenStack: FAQ"
---

## Как настроить LoadBalancer?

**Внимание!!! Для корректного определения клиентского IP необходимо использовать LoadBalancer с поддержкой Proxy Protocol.**

### Пример IngressNginxController

```yaml
apiVersion: deckhouse.io/v1
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

* Разрешить подключение к узлам кластера с виртуальных машин из другой подсети
* Разрешить подключение к портам статического узла для работы приложения
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
security groups. Смотри [параметры модуля cloud-provider-openstack](../../modules/030-cloud-provider-openstack/configuration.html).

## Как поднять гибридный кластер?

Hybrid-кластер представляет собой объединённые в один кластер bare metal узлы и узлы openstack. Для создания такого кластера
необходимо наличие L2 сети между всеми узлами кластера.

1. Удалить flannel из kube-system: `kubectl -n kube-system delete ds flannel-ds`;
2. Включите и [настройте](configuration.html#параметры) модуль.
3. Создайте один или несколько custom resource [OpenStackInstanceClass](cr.html#openstackinstanceclass).
4. Создайте один или несколько custom resource [NodeManager](../../modules/040-node-manager/cr.html#nodegroup) для управления количеством и процессом заказа машин в облаке.

**Важно!** Cloud-controller-manager синхронизирует состояние между OpenStack и Kubernetes, удаляя из Kubernetes те узлы, которых нет в OpenStack. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому если узел Kubernetes запущен не с параметром `--cloud-provider=external`, то он автоматически игнорируется (Deckhouse прописывает `static://` на узлы в `.spec.providerID`, а cloud-controller-manager такие узлы игнорирует).

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

1. Скачиваем последний стабильный образ Ubuntu 18.04

    ```shell
    curl -L https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img --output ~/ubuntu-18-04-cloud-amd64
    ```

2. Подготавливаем OpenStack openrc-файл, который содержит credentials для обращения к API OpenStack.

    > Интерфейс получения openrc-файла может отличаться в зависимости от провайдера OpenStack. Если провайдер предоставляет
    > стандартный интерфейс для OpenStack, то скачать openrc-файл можно по [инструкции](https://docs.openstack.org/zh_CN/user-guide/common/cli-set-environment-variables-using-openstack-rc.html)

3. Либо устанавливаем OpenStack cli по [инструкции](https://docs.openstack.org/newton/user-guide/common/cli-install-openstack-command-line-clients.html).
   Либо можно запустить docker контейнер, прокинув внутрь openrc-файл и скаченный локально образ ubuntu

    ```shell
    docker run -ti --rm -v ~/ubuntu-18-04-cloud-amd64:/ubuntu-18-04-cloud-amd64 -v ~/.mcs-openrc:/openrc jmcvea/openstack-client
    ```

4. Инициализируем переменные окружения из openrc-файла

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

## Как проверить поддерживает ли провайдер SecurityGroups?

Достаточно выполнить команду `openstack security group list`. Если в ответ вы не получите ошибок, то это значит, что [Security Groups](https://docs.openstack.org/nova/pike/admin/security-groups.html) поддерживаются.

## Как настроить работу ONLINE ресайз дисков

OpenStack API успешно рапортует о ресайзе, но Cinder никак не оповещает Nova о том, что диск поресайзился, поэтому диск внутри гостевой ОС остаётся старого размера.

Для устранения проблемы необходимо прописать в `cinder.conf` параметры доступа к Nova API. Например, так:

```ini
[nova]
interface = admin
insecure = {{ keystone_service_internaluri_insecure | bool }}
auth_type = {{ cinder_keystone_auth_plugin }}
auth_url = {{ keystone_service_internaluri }}/v3
password = {{ nova_service_password }}
project_domain_id = default
project_name = service
region_name = {{ nova_service_region }}
user_domain_id = default
username = {{ nova_service_user_name }}
```

https://bugs.launchpad.net/openstack-ansible/+bug/1902914
