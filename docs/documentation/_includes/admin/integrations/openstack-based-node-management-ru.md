{% alert level="warning" %}
Управление CloudEphemeral-узлами переводится с Machine Controller Manager (MCM) на Cluster API (CAPI). Существующие [NodeGroup](/modules/node-manager/cr.html#nodegroup) продолжают использовать MCM, а новые по умолчанию создаются с использованием CAPI. Порядок миграции существующих групп описан в разделе [«Как мигрировать группы узлов на Cluster API (CAPI)»](../../../../faq.html#как-мигрировать-группы-узлов-на-cluster-api-capi).
{% endalert %}

## Настройка групп узлов

Количество машин и параметры их заказа в облаке задаются в кастомном ресурсе [NodeGroup](/modules/node-manager/cr.html#nodegroup). В нём также указывается название используемого для этой группы узлов инстанс-класса (параметр `cloudInstances.classReference`). Инстанс-класс для облачного провайдера {{ site.data.admin.cloud-types.types[page.cloud_type].name }} — это custom resource [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass), в котором указываются конкретные параметры самих машин.

{% alert level="warning" %}
При изменении настроек модуля **пересоздания существующих объектов Machines в кластере НЕ происходит** (новые объекты Machine будут создаваться с новыми параметрами). Пересоздание происходит только при изменении параметров [NodeGroup](/modules/node-manager/cr.html#nodegroup) и [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass).
{% endalert %}

### Примеры конфигурации

Ниже представлены два примера настройки узлов облачного провайдера OpenStack.

#### Пример 1

В следующем манифесте приведён первый пример инстанс-класса.

```yaml
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: test
spec:
  flavorName: m1.large
```

#### Пример 2

В следующем манифесте приведён второй пример инстанс-класса.

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
      password: <PASSWORD>
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

Замените `<PASSWORD>` на пароль пользователя OpenStack.

{% if page.cloud_type == 'selectel' %}

## Создание прерываемых узлов в Selectel

Подробная инструкция по созданию прерываемых узлов в Selectel и ограничения параметра [`preemptible`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-preemptible) приведены в разделе [«Создание прерываемых узлов в Selectel»](/modules/cloud-provider-openstack/faq.html#создание-прерываемых-узлов-в-selectel).

{% endif %}
