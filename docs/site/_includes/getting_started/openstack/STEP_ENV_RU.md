{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Чтобы Deckhouse Kubernetes Platform смог управлять ресурсами в облаке {{ page.platform_name[page.lang] }}, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации](/documentation/v1/modules/030-cloud-provider-openstack/environment.html).

Скачать openrc-файл
- Выполните полученный shell-скрипт, в процессе выполнения которого произойдет создание значений переменных окружения (они будут использованы в параметрах `provider` в конфигурации Deckhouse Kubernetes Platform).

Создайте сервисный аккаунт и скачайте соответствующий openrc-файл. Данные из openrc-файла потребуются далее для заполнения секции `provider` в конфигурации Deckhouse Kubernetes Platform.
