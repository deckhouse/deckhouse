## Схемы размещения

Данный раздел описывает возможные схемы размещения узлов кластера в инфраструктуре {{ site.data.admin.cloud-types.types[page.cloud_type].name }} и связанные с ними настройки. От выбора схемы (layout) зависят принципы сетевого взаимодействия, наличие публичных IP-адресов, маршрутизация исходящего трафика и способ подключения к узлам.

### Standard

Создается внутренняя сеть кластера со шлюзом в публичную сеть, узлы не имеют публичных IP-адресов. Для master-узла заказывается плавающий IP-адрес.

{% alert level="warning" %}
Если провайдер не поддерживает SecurityGroups, все приложения, запущенные на узлах с Floating IP, будут доступны по белому IP-адресу.
Например, `kube-apiserver` на master-узлах будет доступен на порту `6443`. Чтобы избежать этого, рекомендуется использовать схему размещения [SimpleWithInternalNetwork](#simplewithinternalnetwork), либо [Standard](#standard) с bastion-узлом.
{% endalert %}

![resources](../../../../images/cloud-provider-openstack/openstack-standard.png)
<!--- Исходник: https://docs.google.com/drawings/d/1hjmDn2aJj3ru3kBR6Jd6MAW3NWJZMNkend_K43cMN0w/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: Standard
standard:
  internalNetworkCIDR: 192.168.199.0/24         # Обязательный параметр.
  internalNetworkDNSServers:                    # Обязательный параметр.
  - 8.8.8.8
  - 4.2.2.2
  internalNetworkSecurity: true|false           # Необязательный параметр, по умолчанию true.
  externalNetworkName: shared                   # Обязательный параметр.
  bastion:
    zone: ru2-b                                 # Необязательный параметр.
    volumeType: fast-ru-2b                      # Необязательный параметр.
    instanceClass:
      flavorName: m1.large                      # Обязательный параметр.
      imageName: ubuntu-20-04-cloud-amd64       # Обязательный параметр.
      rootDiskSize: 50                          # Обязательный параметр, по умолчанию 50 ГБ.
      additionalTags:
        severity: critical                      # Необязательный параметр.
        environment: production                 # Необязательный параметр.
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 50
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
    additionalTags:
      severity: critical
      environment: production
  # Обязательный параметр. Карта типов томов для сертификатов etcd и Kubernetes
  # (всегда используйте самый быстрый диск, предоставленный поставщиком).
  volumeTypeMap:
    # Если указан rootDiskSize, этот тип тома также будет
    # использоваться для главного корневого тома.
    ru-1a: fast-ru-1a
    ru-1b: fast-ru-1b
    ru-1c: fast-ru-1c
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 20
    # Необязательный параметр, по умолчанию false. Определяет, требуется ли
    # конфигурационный диск во время процесса начальной загрузки виртуальной
    # машины. Это необходимо, если в сети нет DHCP, который используется в
    # качестве шлюза по умолчанию.
    configDrive: false
    # Обязательный параметр, шлюз этой сети будет использован как шлюз по умолчанию.
    # Совпадает с cloud.prefix в ресурсе ClusterConfiguration.
    mainNetwork: kube
    additionalNetworks:                         # Необязательный параметр.
    - office
    - shared
    # Необязательный параметр, если существуют сети с отключенной защитой
    # портов, необходимо указать их имена.
    networksWithSecurityDisabled:
    - office
    # Необязательный параметр, список сетевых пулов, в которых можно заказать
    # плавающие IP-адреса.
    floatingIPPools:
    - public
    - shared
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  zones:
  - ru-1a
  - ru-1b
sshPublicKey: "<SSH_PUBLIC_KEY>"
tags:
  project: cms
  owner: default
provider:
  ...
```

### StandardWithNoRouter

Создается внутренняя сеть кластера без доступа в публичную сеть. Все узлы, включая master-узел, создаются с двумя интерфейсами:
один — в публичную сеть, другой — во внутреннюю сеть. Данная схема размещения должна использоваться, если необходимо, чтобы
все узлы кластера были доступны напрямую.

{% alert level="warning" %}

- В данной конфигурации не поддерживается LoadBalancer. Это связано с тем, что в OpenStack нельзя заказать Floating IP для
  сети без роутера, соответственно, нельзя заказать балансировщик с Floating IP. Если заказывать internal loadbalancer, у которого
  virtual IP создается в публичной сети, он все равно доступен только с узлов кластера.
- В данной конфигурации необходимо явно указывать название внутренней сети в `additionalNetworks` при создании [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) в кластере.

{% endalert %}

![resources](../../../../images/cloud-provider-openstack/openstack-standardwithnorouter.png)
<!--- Исходник: https://docs.google.com/drawings/d/1gkuJhyGza0bXB2lcjdsQewWLEUCjqvTkkba-c5LtS_E/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: StandardWithNoRouter
standardWithNoRouter:
  internalNetworkCIDR: 192.168.199.0/24         # Обязательный параметр.
  externalNetworkName: ext-net                  # Обязательный параметр.
  # Необязательный параметр, указывает, включен ли DHCP в указанной внешней сети
  # (по умолчанию true).
  externalNetworkDHCP: false
  # Необязательный параметр, по умолчанию true.
  internalNetworkSecurity: true|false
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 50
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  # Обязательный параметр. Карта типов томов для сертификатов etcd и Kubernetes
  # (всегда используйте самый быстрый диск, предоставляемый поставщиком).
  volumeTypeMap:
    # Если указан rootDiskSize, этот тип тома также будет
    # использоваться для главного корневого тома.
    nova: ceph-ssd
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                         # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64          # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 20
    # Необязательный параметр, по умолчанию false. Определяет, требуется ли
    # конфигурационный диск во время процесса начальной загрузки виртуальной
    # машины. Это необходимо, если в сети нет DHCP, который используется в
    # качестве шлюза по умолчанию.
    configDrive: false
    # Обязательный параметр, шлюз этой сети будет использован как шлюз по умолчанию.
    # Совпадает с cloud.prefix в ресурсе ClusterConfiguration.
    mainNetwork: kube
    additionalNetworks:                           # Необязательный параметр.
    - office
    - shared
    # Необязательный параметр. Если существуют сети с отключенной защитой
    # портов, необходимо указать их имена.
    networksWithSecurityDisabled:
    - office
    # Необязательный параметр. Список сетевых пулов, в которых можно заказать
    # плавающие IP-адреса.
    floatingIPPools:
    - public
    - shared
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  # Требуется, если указан параметр rootDiskSize. Карта типов томов для главного корневого тома узла.
  volumeTypeMap:
    nova: ceph-ssd
sshPublicKey: "<SSH_PUBLIC_KEY>"
provider:
  ...
```

### Simple

Master-узел и узлы кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер Kubernetes с уже имеющимися виртуальными машинами.

{% alert level="warning" %}
В данной конфигурации не поддерживается LoadBalancer. Это связано с тем, что в OpenStack нельзя заказать Floating IP для
сети без роутера, соответственно, нельзя заказать балансировщик с Floating IP. Если заказывать internal loadbalancer, у которого
virtual IP создается в публичной сети, он все равно доступен только с узлов кластера.
{% endalert %}

![resources](../../../../images/cloud-provider-openstack/openstack-simple.png)
<!--- Исходник: https://docs.google.com/drawings/d/1l-vKRNA1NBPIci3Ya8r4dWL5KA9my7_wheFfMR38G10/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: Simple
simple:
  externalNetworkName: ext-net                  # Обязательный параметр.
  # Необязательный параметр, по умолчанию true.
  externalNetworkDHCP: false
  # Необязательный параметр, по умолчанию VXLAN, также может быть DirectRouting
  # или DirectRoutingWithPortSecurityEnabled.
  podNetworkMode: VXLAN
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 50
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  # Обязательный параметр. Карта типов томов для сертификатов etcd и Kubernetes
  # (всегда используйте самый быстрый диск, предоставляемый поставщиком).
  volumeTypeMap:
    # Если указан rootDiskSize, этот тип тома также будет
    # использоваться для главного корневого тома.
    nova: ceph-ssd
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 20
    # Необязательный параметр, по умолчанию false. Определяет, требуется ли
    # конфигурационный диск во время процесса начальной загрузки виртуальной
    # машины. Это необходимо, если в сети нет DHCP, который используется в
    # качестве шлюза по умолчанию.
    configDrive: false
    # Обязательный параметр, сеть будет использована как шлюз по умолчанию.
    # Совпадает с названием заранее созданной сети.
    mainNetwork: kube
    additionalNetworks:                         # Необязательный параметр.
    - office
    - shared
    # Необязательный параметр. Если существуют сети с отключенной защитой
    # портов, необходимо указать их имена.
    networksWithSecurityDisabled:
    - office
    # Необязательный параметр. Список сетевых пулов, в которых можно заказать
    # плавающие IP-адреса.
    floatingIPPools:
    - public
    - shared
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
sshPublicKey: "<SSH_PUBLIC_KEY>"
provider:
  ...
```

### SimpleWithInternalNetwork

Master-узел и узлы кластера подключаются к существующей сети. Данная схема размещения может понадобиться, если необходимо
объединить кластер Kubernetes с уже имеющимися виртуальными машинами.

{% alert level="warning" %}
В данной схеме размещения не происходит управление SecurityGroups, а подразумевается, что они были ранее созданы.
Для настройки политик безопасности необходимо явно указывать `additionalSecurityGroups` в [OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration) для masterNodeGroup и других nodeGroups, а также `additionalSecurityGroups` при создании [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) в кластере.
{% endalert %}

![resources](../../../../images/cloud-provider-openstack/openstack-simplewithinternalnetwork.png)
<!--- Исходник: https://docs.google.com/drawings/d/1H9HGOn4abpmZwIhpwwdZSSO9izvyOZakG8HpmmzZZEo/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: SimpleWithInternalNetwork
simpleWithInternalNetwork:
  # Обязательный параметр, все узлы кластера должны находиться в одной подсети.
  internalSubnetName: pivot-standard
  # Необязательный параметр, по умолчанию DirectRoutingWithPortSecurityEnabled,
  # также может быть DirectRouting или VXLAN.
  podNetworkMode: DirectRoutingWithPortSecurityEnabled
  # Необязательный параметр. Если задан, будет использоваться для конфигурации
  # балансировщика нагрузки по умолчанию и для главного плавающего IP-адреса.
  externalNetworkName: ext-net
  # Необязательный параметр, по умолчанию true.
  masterWithExternalFloatingIP: false
masterNodeGroup:
  replicas: 3
  instanceClass:
    flavorName: m1.large                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр.
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 50
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
  # Обязательный параметр. Карта типов томов для сертификатов etcd и Kubernetes
  # (всегда используйте самый быстрый диск, предоставляемый поставщиком).
  volumeTypeMap:
    # Если указан rootDiskSize, этот тип тома также будет
    # использоваться для главного корневого тома.
    nova: ceph-ssd
nodeGroups:
- name: front
  replicas: 2
  instanceClass:
    flavorName: m1.small                        # Обязательный параметр.
    imageName: ubuntu-18-04-cloud-amd64         # Обязательный параметр
    # Необязательный параметр. Если не указан — используется локальный диск.
    rootDiskSize: 20
    # Необязательный параметр, по умолчанию false. Определяет, требуется ли
    # конфигурационный диск во время процесса начальной загрузки виртуальной
    # машины. Это необходимо, если в сети нет DHCP, который используется в
    # качестве шлюза по умолчанию.
    configDrive: false
    # Обязательный параметр. Сеть будет использована как шлюз по умолчанию.
    # Совпадает с названием заранее созданной сети.
    mainNetwork: kube
    additionalNetworks:                         # Необязательный параметр.
    - office
    - shared
    # Необязательный параметр. Если существуют сети с отключенной защитой
    # портов, необходимо указать их имена.
    networksWithSecurityDisabled:
    - office
    # Необязательный параметр. Список сетевых пулов, в которых можно заказать
    # плавающие IP-адреса.
    floatingIPPools:
    - public
    - shared
    # Необязательный параметр, дополнительные группы безопасности.
    additionalSecurityGroups:
    - sec_group_1
    - sec_group_2
sshPublicKey: "<SSH_PUBLIC_KEY>"
provider:
  ...
```

## Конфигурация

Интеграции с {{ site.data.admin.cloud-types.types[page.cloud_type].name }} осуществляется с помощью [ресурса OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration), который описывает конфигурацию облачного кластера в {{ site.data.admin.cloud-types.types[page.cloud_type].name }} и используется облачным провайдером, если управляющий слой (control plane) кластера размещён в облаке. Отвечающий за интеграцию модуль DKP настраивается автоматически, исходя из выбранной схемы размещения.

Выполните следующую команду, чтобы изменить конфигурацию в работающем кластере:

```shell
d8 system edit provider-cluster-configuration
```

{% alert level="info" %}
После изменения параметров узлов необходимо выполнить команду `dhctl converge`, чтобы изменения вступили в силу.
{% endalert %}

Количество и параметры процесса заказа машин в облаке настраиваются в кастомном ресурсе [NodeGroup](/modules/node-manager/cr.html#nodegroup), в котором также указывается название используемого для этой группы узлов инстанс-класса (параметр `cloudInstances.classReference`).
Инстанс-класс для облачного провайдера {{ site.data.admin.cloud-types.types[page.cloud_type].name }} — это custom resource [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass), в котором указываются конкретные параметры самих машин.

{% alert level="warning" %}
При изменении настроек модуля **пересоздания существующих объектов Machines в кластере НЕ происходит** (новые объекты Machine будут создаваться с новыми параметрами). Пересоздание происходит только при изменении параметров [NodeGroup](/modules/node-manager/cr.html#nodegroup) и [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass).
{% endalert %}

### Примеры конфигурации

Ниже представлены два простых примера конфигурации облачного провайдера OpenStack.

#### Пример 1

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: test
spec:
  flavorName: m1.large
```

#### Пример 2

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-openstack
spec:
  version: 1
  enabled: true
  settings:
    connection:
      authURL: https://test.tests.com:5000/v3/
      domainName: default
      tenantName: default
      username: jamie
      password: nein
      region: SomeRegion
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

### Список необходимых сервисов

Список сервисов {{ site.data.admin.cloud-types.types[page.cloud_type].name }}, необходимых для работы Deckhouse Kubernetes Platform в {{ site.data.admin.cloud-types.types[page.cloud_type].name }}:

| Сервис                           | Версия API |
|:---------------------------------|:----------:|
| Identity (Keystone)              | v3         |
| Compute (Nova)                   | v2         |
| Network (Neutron)                | v2         |
| Block Storage (Cinder)           | v3         |
| Load Balancing (Octavia) &#8432; | v2         |

&#8432;  Если нужно заказывать Load Balancer.

{% if page.cloud_type == 'vk-private' or page.cloud_type == 'vk' %}
Адреса и порты API можно узнать [в официальной документации](https://cloud.vk.com/docs/tools-for-using-services/api/rest-api/endpoints).
{% endif %}

### Настройка LoadBalancer

{% alert level="warning" %}
Для корректного определения клиентского IP-адреса необходимо использовать LoadBalancer с поддержкой Proxy Protocol.
{% endalert %}

#### Пример IngressNginxController

Ниже представлен простой пример конфигурации [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller):

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

### Настройка и политики безопасности на узлах кластера

Вариантов, зачем может понадобиться ограничить или, наоборот, расширить входящий или исходящий трафик на виртуальных
машинах кластера, может быть множество. Например:

* Разрешить подключение к узлам кластера с виртуальных машин из другой подсети.
* Разрешить подключение к портам статического узла для работы приложения.
* Ограничить доступ к внешним ресурсам или другим ВМ в облаке по требованию службы безопасности.

Для всего этого следует применять дополнительные группы безопасности (security groups). Можно использовать только группы безопасности, предварительно
созданные в облаке.

#### Установка дополнительных групп безопасности (security groups) на статических и master-узлах

Данный параметр можно задать либо при создании кластера, либо в уже существующем кластере. В обоих случаях дополнительные
группы безопасности указываются в [OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration):

* для master-узлов — в секции `masterNodeGroup` в поле `additionalSecurityGroups`;
* для статических узлов — в секции `nodeGroups` в конфигурации, описывающей желаемую nodeGroup, а также в поле `additionalSecurityGroups`.

Поле `additionalSecurityGroups` представляет собой массив строк с именами групп безопасности.

#### Установка дополнительных групп безопасности (security groups) на ephemeral-узлах

Необходимо прописать параметр `additionalSecurityGroups` для всех [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) в кластере, которым нужны дополнительные
групп безопасности.

### Как загрузить образ в {{ site.data.admin.cloud-types.types[page.cloud_type].name }}

1. Скачайте последний стабильный образ Ubuntu 18.04:

   ```shell
   curl -L https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img --output ~/ubuntu-18-04-cloud-amd64
   ```

2. Подготовьте openrc-файл, который содержит credentials для обращения к API {{ site.data.admin.cloud-types.types[page.cloud_type].name }}.

   > Интерфейс получения openrc-файла может отличаться в зависимости от провайдера {{ site.data.admin.cloud-types.types[page.cloud_type].name }}. Если провайдер предоставляет
   > стандартный интерфейс для {{ site.data.admin.cloud-types.types[page.cloud_type].name }}, скачать openrc-файл можно [по инструкции](https://docs.openstack.org/ocata/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html#download-and-source-the-openstack-rc-file).

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
   openstack volume type list
   ```

   Пример вывода:

   ```console
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

6. Создайте образ и передайте в него в качестве свойств тип диска, который будет использоваться (если {{ site.data.admin.cloud-types.types[page.cloud_type].name }} не поддерживает локальные диски или если эти диски не подходят для работы):

   ```shell
   openstack image create --private --disk-format qcow2 --container-format bare \
     --file /ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=dp1-high-iops ubuntu-18-04-cloud-amd64
   ```

7. Проверьте, что образ успешно создан:

   ```shell
   openstack image show ubuntu-18-04-cloud-amd64
   ```

   Пример вывода:

   ```console
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

### Проверка поддержки провайдером группы безопасности (security groups)

Выполните команду `openstack security group list`. Если в ответ вы не получите ошибок, это значит, что [группы безопасности](https://docs.openstack.org/nova/pike/admin/security-groups.html) поддерживаются.

### Настройка работы онлайн-изменений размера дисков

При успешном изменении размера диска с помощью VK Cloud API, Cinder не передает информацию об этом изменении в Nova, в результате чего диск внутри гостевой операционной системы остается прежнего размера.

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

Более подробная информация в [документации OpenStack-Ansible](https://bugs.launchpad.net/openstack-ansible/+bug/1902914)

### Использование rootDiskSize

#### Диски в {{ site.data.admin.cloud-types.types[page.cloud_type].name }}

Диск узла может быть локальным или сетевым. В терминологии {{ site.data.admin.cloud-types.types[page.cloud_type].name }} локальный диск — это ephemeral disk, а сетевой — persistent disk (cinder storage). Первый удаляется вместе с ВМ, а второй остается в облаке, когда ВМ удаляется.

* Для master-узла предпочтительнее сетевой диск, чтобы узел мог мигрировать между гипервизорами.
* Для ephemeral-узла предпочтительнее локальный диск, чтобы сэкономить на стоимости. Не все облачные провайдеры поддерживают использование локальных дисков. Если локальные диски не поддерживаются, для ephemeral-узлов придется использовать сетевые диски.

| Локальный диск (ephemeral)    | Сетевой диск (persistent)                    |
| ----------------------------- | -------------------------------------------- |
| Удаляется вместе с ВМ         | Остается в облаке и может переиспользоваться |
| Дешевле                       | Дороже                                       |
| Подходит для ephemeral-узлов  | Подходит для master-узлов                    |

#### Параметр rootDiskSize

В OpenStackInstanceClass есть [параметр `rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize), и в {{ site.data.admin.cloud-types.types[page.cloud_type].name }} flavor есть параметр размера диска.

Какой диск будет заказан в зависимости от комбинации параметров, указано в таблице:

|                              | flavor disk size = 0                 | flavor disk size > 0                              |
| ---------------------------- | ------------------------------------ | ------------------------------------------------- |
| **`rootDiskSize` не указан** | ❗️*Необходимо задать размер*. Без указания размера будет ошибка создания ВМ. | Локальный диск с размером из flavor               |
| **`rootDiskSize` указан**    | Сетевой диск размером `rootDiskSize`                                         | ❗ Сетевой (rootDiskSize) и локальный (из flavor). Избегайте использования этого варианта, так как облачный провайдер будет взимать плату за оба диска. |

{% if page.cloud_type != 'selectel' %}

> При создании узлов с типом CloudEphemeral в облаке Selectel, для создания узла в зоне отличной от зоны A, необходимо заранее создать flavor с диском необходимого размера. Параметр [`rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) в этом случае указывать не нужно.

{% endif %}

##### Рекомендация для master-узлов и бастиона — сетевой диск

- Используйте flavor с нулевым размером диска.
- Задайте [`rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) в OpenStackInstanceClass.
- Проконтролируйте тип диска. Тип диска будет взят из образа ОС, если он задан. Если нет, тип диска будет взят из [`volumeTypeMap`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-masternodegroup-volumetypemap).

##### Рекомендация для ephemeral-узлов — локальный диск

- Используйте flavor с заданным размером диска.
- Не используйте [параметр `rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) в OpenStackInstanceClass.
- Проконтролируйте тип диска. Тип диска будет взят из образа ОС, если он [задан](#как-переопределить-тип-диска-по-умолчанию-cloud-провайдера). Если нет, будет использоваться тип диска по умолчанию облачного провайдера.

#### Как проверить объем диска в flavor

Выполните следующую команду:

```shell
openstack flavor show m1.medium-50g -c disk
```

Пример вывода:

```console
+-------+-------+
| Field | Value |
+-------+-------+
| disk  | 50    |
+-------+-------+
```

### Как переопределить тип диска по умолчанию облачного провайдера

Если у облачного провайдера доступно несколько типов, вы можете указать тип диска по умолчанию для образа. Для этого необходимо указать название типа диска в метаданных образа. Тогда все ВМ, создаваемые из этого образа, будут использовать указанный тип сетевого диска.

Также вы можете создать новый образ {{ site.data.admin.cloud-types.types[page.cloud_type].name }} и загрузить его.

Пример:

```shell
openstack volume type list
openstack image set ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=VOLUME_NAME
```

{% if page.cloud_type != 'vk-private' and page.cloud_type != 'vk' %}

### Оффлайн-изменение размера диска

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
{% endif %}
