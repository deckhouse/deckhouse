{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Чтобы Deckhouse Kubernetes Platform смог управлять ресурсами в облаке {{ page.platform_name[page.lang] }}, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации](/documentation/v1/modules/030-cloud-provider-openstack/environment.html).

[Создайте сервисный аккаунт](https://docs.selectel.ru/cloud/servers/tools/openstack/#%D1%81%D0%BE%D0%B7%D0%B4%D0%B0%D1%82%D1%8C-%D1%81%D0%B5%D1%80%D0%B2%D0%B8%D1%81%D0%BD%D0%BE%D0%B3%D0%BE-%D0%BF%D0%BE%D0%BB%D1%8C%D0%B7%D0%BE%D0%B2%D0%B0%D1%82%D0%B5%D0%BB%D1%8F) и [скачайте соответствующий openrc-файл](https://docs.selectel.ru/cloud/servers/tools/openstack/#configure-authorization). Данные из openrc-файла потребуются далее для заполнения секции `provider` в конфигурации Deckhouse Kubernetes Platform.
