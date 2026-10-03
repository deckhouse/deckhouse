{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Подготовьте окружение {{ page.platform_name[page.lang] }}, чтобы Deckhouse Platform мог управлять ресурсами в облаке. Полная инструкция приведена [на странице подготовки окружения](/modules/cloud-provider-openstack/environment.html) модуля `cloud-provider-openstack`.

[Создайте сервисный аккаунт](https://docs.selectel.ru/cloud-servers/tools/openstack-cli/configure-openstack-cli/#add-service-user-for-os) и [скачайте соответствующий openrc-файл](https://docs.selectel.ru/cloud-servers/tools/openstack-cli/configure-openstack-cli/#download-rc-file-for-os). Данные из openrc-файла потребуются далее для заполнения секции `provider` в конфигурации Deckhouse Platform.

Чтобы создать узел с типом `CloudEphemeral` в зоне Selectel, отличной от зоны A, заранее создайте flavor с диском нужного размера. Параметр [rootDiskSize](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) в этом случае не указывайте.

{% offtopic title="Пример создания flavor..." %}
```shell
openstack flavor create c4m8d50 --ram 8192 --disk 50 --vcpus 4 --private
```
{% endofftopic %}
