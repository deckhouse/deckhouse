{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Подготовьте окружение {{ page.platform_name[page.lang] }}, чтобы Deckhouse Platform мог управлять ресурсами в облаке. Полная инструкция приведена [на странице подготовки окружения](/modules/cloud-provider-openstack/environment.html) модуля `cloud-provider-openstack`.

Создайте сервисный аккаунт и скачайте соответствующий openrc-файл. Данные из openrc-файла потребуются далее для заполнения секции `provider` в конфигурации Deckhouse Platform.
