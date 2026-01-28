{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Чтобы Deckhouse Kubernetes Platform смог управлять ресурсами в облаке {{ page.platform_name[page.lang] }}, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации](/modules/cloud-provider-openstack/environment.html).

[Создайте сервисный аккаунт](https://docs.selectel.ru/cloud-servers/tools/openstack-cli/configure-openstack-cli/#add-service-user-for-docker) и [скачайте соответствующий openrc-файл](https://docs.selectel.ru/cloud-servers/tools/openstack-cli/configure-openstack-cli/#download-rc-file-for-docker). Данные из openrc-файла потребуются далее для заполнения секции `provider` в конфигурации Deckhouse Kubernetes Platform.

Обратите внимание, что при создании узлов с типом `CloudEphemeral` в облаке Selectel, для создания узла в зоне отличной от зоны A, необходимо заранее создать flavor с диском необходимого размера. Параметр [rootDiskSize](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) в этом случае указывать не нужно.

{% offtopic title="Пример создания flavor..." %}
```shell
openstack flavor create c4m8d50 --ram 8192 --disk 50 --vcpus 4 --private
```
{% endofftopic %}
