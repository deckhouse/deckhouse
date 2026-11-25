## Хранилище

### Устойчивость Cinder CSI при повторной аутентификации в OpenStack

CSI-драйвер Cinder поддерживает повторную аутентификацию в OpenStack с обновлением сервисного каталога. Это повышает устойчивость операций с томами в подах `cinder-csi-plugin`, работающих длительное время без перезапуска.

При истечении срока действия токена или получении ответа `401 Unauthorized` драйвер повторно проходит аутентификацию в OpenStack и обновляет данные, используемые для обращения к сервисам OpenStack API. Благодаря этому операции с PersistentVolume, включая подключение и отключение томов, продолжают выполняться без необходимости перезапуска подов CSI-драйвера.

Такое поведение особенно важно для кластеров, где поды `cinder-csi-plugin` работают без перезапуска длительное время, а сервисный каталог OpenStack может изменяться между первичной аутентификацией и последующими обращениями к API.

### Настройка работы онлайн-изменений размера дисков

При успешном изменении размера диска с помощью OpenStack API, Cinder не передает информацию об этом изменении в Nova, в результате чего диск внутри гостевой операционной системы остается прежнего размера.

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

Если в инстанс-классе группы задан параметр [`rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize), root-диск создаётся в Cinder. В облаках, где типы дисков привязаны к зонам доступности, укажите тип диска для каждой зоны в параметре [`volumeTypeMap`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-volumetypemap) ресурса OpenStackInstanceClass. Иначе заказ узла в зоне, где тип диска по умолчанию недоступен, завершается ошибкой `HTTP 400 Availability zone '…' is invalid`.

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
| **`rootDiskSize` не указан** | ❗Необходимо задать размер. Без указания размера создать ВМ не удастся. | Локальный диск с размером из flavor               |
| **`rootDiskSize` указан**    | Сетевой диск размером `rootDiskSize`                                         | ❗ Сетевой (rootDiskSize) и локальный (из flavor). Избегайте использования этого варианта, так как облачный провайдер будет взимать плату за оба диска. |

{% if page.cloud_type != 'selectel' %}

> При создании узлов с типом CloudEphemeral в облаке Selectel, для создания узла в зоне отличной от зоны A, необходимо заранее создать flavor с диском необходимого размера. Параметр [`rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) в этом случае указывать не нужно.

{% endif %}

**Рекомендация для master-узлов и бастиона — сетевой диск**

- Используйте flavor с нулевым размером диска.
- Задайте [`rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) в OpenStackInstanceClass.
- Проконтролируйте тип диска. Тип диска будет взят из образа ОС, если он задан. Если нет, тип диска будет взят из [`volumeTypeMap`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-masternodegroup-volumetypemap).

**Рекомендация для ephemeral-узлов — локальный диск**

- Используйте flavor с заданным размером диска.
- Не используйте [параметр `rootDiskSize`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) в OpenStackInstanceClass.
- Проконтролируйте тип диска. Тип диска будет взят из образа ОС, если он [задан](#как-переопределить-тип-диска-по-умолчанию-облачного-провайдера). Если нет, будет использоваться тип диска по умолчанию облачного провайдера.

#### Проверка объема диска в flavor

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

Если у облачного провайдера доступно несколько типов, вы можете задать тип диска по умолчанию для образа. Для этого необходимо указать название типа диска в метаданных образа. Тогда все ВМ, создаваемые из этого образа, будут использовать указанный тип сетевого диска.

Также вы можете создать новый образ {{ site.data.admin.cloud-types.types[page.cloud_type].name }} и загрузить его.

Пример:

```shell
openstack volume type list
openstack image set ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=VOLUME_NAME
```

{% if page.cloud_type != 'vk-private' and page.cloud_type != 'vk' %}

### Оффлайн-изменение размера диска

Если при изменении размера диска возникает следующая ошибка, выполните такие действия:

- уменьшите количество реплик StatefulSet до 0;
- дождитесь изменения размера дисков;
- восстановите количество реплик, которое было до начала операции.

```text
Warning  VolumeResizeFailed     5s (x11 over 41s)  external-resizer cinder.csi.openstack.org                                   
resize volume "pvc-555555-ab66-4f8d-947c-296520bae4c1" by resizer "cinder.csi.openstack.org" failed: 
rpc error: code = Internal desc = Could not resize volume "bb5a275b-3f30-4916-9480-9efe4b6dfba5" to size 2: 
Expected HTTP response code [202] when accessing 
[POST https://public.infra.myfavourite-cloud-provider.ru:8776/v3/555555555555/volumes/bb5a275b-3f30-4916-9480-9efe4b6dfba5/action], but got 406 instead
{"computeFault": {"message": "Version 3.42 is not supported by the API. Minimum is 3.0 and maximum is 3.27.", "code": 406}}
```
{% endif %}

## Настройка LoadBalancer

{% alert level="warning" %}
Для корректного определения клиентского IP-адреса необходимо использовать LoadBalancer с поддержкой Proxy Protocol.
{% endalert %}

Рекомендуется ограничивать список узлов, добавляемых в пул балансировщика, с помощью аннотации [`loadbalancer.openstack.org/node-selector`](https://github.com/kubernetes/cloud-provider-openstack/blob/master/docs/openstack-cloud-controller-manager/using-openstack-cloud-controller-manager.md#load-balancer).

Без ограничения по `node-selector` cloud-controller-manager может использовать в качестве таргетов балансировщика все подходящие узлы кластера. В результате добавление или удаление узлов, не связанных с обслуживаемой балансировщиком нагрузкой, может приводить к обновлению состава пула балансировщика. В крупных или часто изменяющихся кластерах такие обновления могут происходить регулярно, а в некоторых конфигурациях сопровождаться кратковременными нарушениями существующих соединений.

В поле `annotations` соответствующей конфигурации инлета ресурса [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) можно указать следующие аннотации:

* `loadbalancer.openstack.org/node-selector` — выбирает узлы, которые будут использоваться как таргеты LoadBalancer.
* `loadbalancer.openstack.deckhouse.io/load-balancer-id` — указывает OpenStack CCM использовать заранее созданный Octavia-балансировщик.
* `loadbalancer.openstack.deckhouse.io/load-balancer-address` — указывает OpenStack CCM привязать заранее выделенный floating IP к создаваемому им балансировщику.

Deckhouse Platform автоматически добавит указанные аннотации в сгенерированный объект Service типа LoadBalancer.

Если используется аннотация `loadbalancer.openstack.deckhouse.io/load-balancer-id`, балансировщик должен соответствовать следующим требованиям:

* находиться в подсети кластера;
* иметь состояние `ACTIVE`.

Если используется `loadbalancer.openstack.deckhouse.io/load-balancer-id` для подключения заранее созданного балансировщика с произвольным именем, привяжите floating IP к его VIP-порту до создания кластера. В этом случае не указывайте аннотацию `loadbalancer.openstack.deckhouse.io/load-balancer-address`.

Если используется только `loadbalancer.openstack.deckhouse.io/load-balancer-address`, floating IP должен соответствовать следующим требованиям:

* не быть привязанным к порту;
* находиться в floating-сети, настроенной для OpenStack CCM.

Если указанный floating IP недоступен, OpenStack CCM не сможет назначить внешний IP-адрес объекту Service.

Не добавляйте аннотации `loadbalancer.openstack.deckhouse.io/load-balancer-id` и `loadbalancer.openstack.deckhouse.io/load-balancer-address` к прикладным ресурсам Ingress. Указывайте их только в конфигурации IngressNginxController: Deckhouse Platform добавит их в созданный объект Service, который обрабатывает `openstack-cloud-controller-manager`.

### IngressNginxController с заранее созданным балансировщиком

В примере ниже:

* поды Ingress-контроллера размещаются на frontend-узлах;
* `loadbalancer.openstack.org/node-selector` ограничивает пул балансировщика этими же узлами;
* `loadbalancer.openstack.deckhouse.io/load-balancer-id` указывает заранее созданный Octavia-балансировщик, к VIP-порту которого уже привязан floating IP.

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
      loadbalancer.openstack.deckhouse.io/load-balancer-id: "df7c6f73-8c68-4a11-a3e2-6268a655ce9b"
      loadbalancer.openstack.org/node-selector: "node-role.deckhouse.io/frontend="
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

### IngressNginxController с заранее выделенным floating IP

В этом примере OpenStack CCM создает балансировщик и привязывает к нему указанный свободный floating IP:

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
      loadbalancer.openstack.deckhouse.io/load-balancer-address: "203.0.113.10"
      loadbalancer.openstack.org/node-selector: "node-role.deckhouse.io/frontend="
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
