---
title: "Cloud provider — OpenStack: FAQ"
---

## Как настроить LoadBalancer?

> **Внимание!** Для корректного определения клиентского IP-адреса необходимо использовать LoadBalancer с поддержкой Proxy Protocol.

### Пример IngressNginxController

Ниже представлен простой пример конфигурации `IngressNginxController`:

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

Вариантов, зачем может понадобиться ограничить или, наоборот, расширить входящий или исходящий трафик на виртуальных
машинах кластера, может быть множество. Например:

* Разрешить подключение к узлам кластера с виртуальных машин из другой подсети.
* Разрешить подключение к портам статического узла для работы приложения.
* Ограничить доступ к внешним ресурсам или другим ВМ в облаке по требованию службы безопасности.

Для всего этого следует применять дополнительные группы безопасности (security groups). Можно использовать только группы безопасности, предварительно
созданные в облаке.

### Установка дополнительных групп безопасности (security groups) на статических и master-узлах

Данный параметр можно задать либо при создании кластера, либо в уже существующем кластере. В обоих случаях дополнительные
группы безопасности указываются в `OpenStackClusterConfiguration`:

* для master-узлов — в секции `masterNodeGroup` в поле `additionalSecurityGroups`;
* для статических узлов — в секции `nodeGroups` в конфигурации, описывающей желаемую nodeGroup, а также в поле `additionalSecurityGroups`.

Поле `additionalSecurityGroups` представляет собой массив строк с именами групп безопасности.

### Установка дополнительных групп безопасности (security groups) на ephemeral-узлах

Необходимо прописать параметр `additionalSecurityGroups` для всех `OpenStackInstanceClass` в кластере, которым нужны дополнительные
групп безопасности. Подробнее — [параметры модуля `cloud-provider-openstack`](../../modules/030-cloud-provider-openstack/configuration.html).

## Как поднять гибридный кластер?

Гибридный кластер представляет собой кластер, в котором могут быть как узлы bare metal, так и узлы OpenStack. Для создания такого кластера
необходимо наличие L2-сети между всеми узлами кластера.

Чтобы поднять гибридный кластер, выполните следующие шаги:

1. Удалите flannel из kube-system: `kubectl -n kube-system delete ds flannel-ds`.
2. Включите и [настройте](configuration.html#параметры) модуль.
3. Создайте один или несколько custom resource [OpenStackInstanceClass](cr.html#openstackinstanceclass).
4. Создайте один или несколько custom resource [NodeManager](../../modules/040-node-manager/cr.html#nodegroup) для управления количеством и процессом заказа машин в облаке.

> **Важно!** Cloud-controller-manager синхронизирует состояние между OpenStack и Kubernetes, удаляя из Kubernetes те узлы, которых нет в OpenStack. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому, если узел Kubernetes запущен не с параметром `--cloud-provider=external`, он автоматически игнорируется (Deckhouse прописывает `static://` на узлы в `.spec.providerID`, а cloud-controller-manager такие узлы игнорирует).

### Подключение storage в гибридном кластере

Если вам требуются PersistentVolumes на узлах, подключаемых к кластеру из OpenStack, необходимо создать StorageClass с нужным OpenStack volume type. Получить список типов можно с помощью команды `openstack volume type list`.

Например, для volume type `ceph-ssd`:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
provisioner: csi-cinderplugin # Обязательно должно быть так.
parameters:
  type: ceph-ssd
volumeBindingMode: WaitForFirstConsumer
```

## Как загрузить образ в OpenStack?

1. Скачайте последний стабильный образ Ubuntu 18.04:

   ```shell
   curl -L https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img --output ~/ubuntu-18-04-cloud-amd64
   ```

2. Подготовьте OpenStack openrc-файл, который содержит credentials для обращения к API OpenStack.

   > Интерфейс получения openrc-файла может отличаться в зависимости от провайдера OpenStack. Если провайдер предоставляет
   > стандартный интерфейс для OpenStack, скачать openrc-файл можно [по инструкции](https://docs.openstack.org/ocata/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html#download-and-source-the-openstack-rc-file).

3. Либо установите OpenStack-клиента [по инструкции](https://docs.openstack.org/newton/user-guide/common/cli-install-openstack-command-line-clients.html).

   Также можно запустить контейнер, смонтировать в него openrc-файл и скачанный локально образ Ubuntu:

   ```shell
   docker run -ti --rm -v ~/ubuntu-18-04-cloud-amd64:/ubuntu-18-04-cloud-amd64 -v ~/.openrc:/openrc jmcvea/openstack-client
   ```

4. Инициализируйте переменные окружения из openrc-файла:

   ```shell
   source /openrc
   ```

5. Получите список доступных типов дисков:

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

6. Создайте образ и передайте в него в качестве свойств тип диска, который будет использоваться (если OpenStack не поддерживает локальные диски или если эти диски не подходят для работы):

   ```shell
   openstack image create --private --disk-format qcow2 --container-format bare \
     --file /ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=dp1-high-iops ubuntu-18-04-cloud-amd64
   ```

7. Проверьте, что образ успешно создан:

   ```text
   / # openstack image show ubuntu-18-04-cloud-amd64
   +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
   | Field            | Value                                                                                                                                                                                                                                                                                     |
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

## Как проверить, поддерживает ли провайдер группы безопасности (security groups)?

Достаточно выполнить команду `openstack security group list`. Если в ответ вы не получите ошибок, это значит, что [группы безопасности](https://docs.openstack.org/nova/pike/admin/security-groups.html) поддерживаются.

## Как настроить работу онлайн-изменения размера дисков?

OpenStack API успешно рапортует об изменении размера диска, но Cinder никак не оповещает Nova о том, что диск изменился, поэтому диск внутри гостевой ОС остается старого размера.

Для устранения проблемы необходимо прописать в `cinder.conf` параметры доступа к Nova API. Например, так:

{% raw %}

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

{% endraw %}

[Источник...](https://bugs.launchpad.net/openstack-ansible/+bug/1902914)

## Как использовать `rootDiskSize` и когда он предпочтителен?

### Диски в OpenStack

Диск узла может быть локальным или сетевым. В терминологии OpenStack локальный диск — это ephemeral disk, а сетевой — persistent disk (cinder storage). Первый удаляется вместе с ВМ, а второй остается в облаке, когда ВМ удаляется.

* Для master-узла предпочтительнее сетевой диск, чтобы узел мог мигрировать между гипервизорами.
* Для ephemeral-узла предпочтительнее локальный диск, чтобы сэкономить на стоимости. Не все cloud-провайдеры поддерживают использование локальных дисков. Если локальные диски не поддерживаются, для ephemeral-узлов придется использовать сетевые диски.

| Локальный диск (ephemeral)    | Сетевой диск (persistent)                    |
| ----------------------------- | -------------------------------------------- |
| Удаляется вместе с ВМ         | Остается в облаке и может переиспользоваться |
| Дешевле                       | Дороже                                       |
| Подходит для ephemeral-узлов  | Подходит для master-узлов                    |

### Параметр `rootDiskSize`

В `OpenStackInstanceClass` есть параметр `rootDiskSize`, и в OpenStack flavor есть параметр размера диска.

Какой диск закажется в зависимости от комбинации параметров, указано в таблице:

|                              | flavor disk size = 0                 | flavor disk size > 0                              |
| ---------------------------- | ------------------------------------ | ------------------------------------------------- |
| **`rootDiskSize` не указан** | ❗️*Необходимо задать размер*. Без указания размера будет ошибка создания ВМ. | Локальный диск с размером из flavor               |
| **`rootDiskSize` указан**    | Сетевой диск размером `rootDiskSize`                                         | ❗ Сетевой (rootDiskSize) и локальный (из flavor). Избегайте использования этого варианта, так как cloud-провайдер будет взимать плату за оба диска. |

#### Рекомендация для master-узлов и бастиона — сетевой диск

- Используйте flavor с нулевым размером диска.
- Задайте `rootDiskSize` в `OpenStackInstanceClass`.
- Проконтролируйте тип диска. Тип диска будет взят из образа ОС, если он [задан](#как-переопределить-тип-диска-по-умолчанию-cloud-провайдера). Если нет, тип диска будет взят из [volumeTypeMap](cluster_configuration.html#openstackclusterconfiguration-masternodegroup-volumetypemap).

#### Рекомендация для ephemeral-узлов — локальный диск

- Используйте flavor с заданным размером диска.
- Не используйте параметр `rootDiskSize` в OpenStackInstanceClass.
- Проконтролируйте тип диска. Тип диска будет взят из образа ОС, если он [задан](#как-переопределить-тип-диска-по-умолчанию-cloud-провайдера). Если нет, будет использоваться тип диска по умолчанию cloud-провайдера.

### Как проверить объем диска в flavor?

```shell
# openstack flavor show m1.medium-50g -c disk
+-------+-------+
| Field | Value |
+-------+-------+
| disk  | 50    |
+-------+-------+
```

## Как переопределить тип диска по умолчанию cloud-провайдера?

Если у cloud-провайдера доступно несколько типов, вы можете указать тип диска по умолчанию для образа. Для этого необходимо указать название типа диска в метаданных образа. Тогда все ВМ, создаваемые из этого образа, будут использовать указанный тип сетевого диска.

Также вы можете создать новый образ OpenStack [следующим образом](#как-загрузить-образ-в-openstack).

Пример:

```shell
openstack volume type list
openstack image set ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=VOLUME_NAME
```

## Оффлайн-изменение размера диска

Некоторые облачные провайдеры (например, VK Cloud) могут не поддерживать онлайн-изменение дисков.
Если при изменении размера диска вы получаете следующую ошибку, необходимо уменьшить количество реплик StatefulSet до 0, подождать изменения размера дисков
и вернуть обратно количество реплик, которое было до начала операции.

```text
Warning  VolumeResizeFailed     5s (x11 over 41s)  external-resizer cinder.csi.openstack.org                                   
resize volume "pvc-555555-ab66-4f8d-947c-296520bae4c1" by resizer "cinder.csi.openstack.org" failed: 
rpc error: code = Internal desc = Could not resize volume "bb5a275b-3f30-4916-9480-9efe4b6dfba5" to size 2: 
Expected HTTP response code [202] when accessing 
[POST https://public.infra.myfavourite-cloud-provider.ru:8776/v3/555555555555/volumes/bb5a275b-3f30-4916-9480-9efe4b6dfba5/action], but got 406 instead
{"computeFault": {"message": "Version 3.42 is not supported by the API. Minimum is 3.0 and maximum is 3.27.", "code": 406}}
```
