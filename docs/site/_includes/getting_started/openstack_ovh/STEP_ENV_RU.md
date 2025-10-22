{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Чтобы Deckhouse Kubernetes Platform смог управлять ресурсами в облаке {{ page.platform_name[page.lang] }}, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации](/modules/cloud-provider-openstack/environment.html).

Создайте сервисный аккаунт и скачайте соответствующий openrc-файл. Данные из openrc-файла потребуются далее для заполнения секции `provider` в конфигурации Deckhouse Kubernetes Platform.
