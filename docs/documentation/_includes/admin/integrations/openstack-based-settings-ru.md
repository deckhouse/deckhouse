## Конфигурация

Интеграция с {{ site.data.admin.cloud-types.types[page.cloud_type].name }} осуществляется с помощью
[ресурса OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration),
который описывает конфигурацию облачного кластера в
{{ site.data.admin.cloud-types.types[page.cloud_type].name }} и используется облачным провайдером, если управляющий слой (control plane) кластера размещён в облаке. Отвечающий за интеграцию модуль Deckhouse Platform настраивается автоматически, исходя из выбранной схемы размещения.

Выполните следующую команду, чтобы изменить конфигурацию в работающем кластере:

```shell
d8 system edit provider-cluster-configuration
```

{% alert level="info" %}
После изменения параметров узлов необходимо выполнить
команду `dhctl converge`, чтобы применить изменения.
{% endalert %}
