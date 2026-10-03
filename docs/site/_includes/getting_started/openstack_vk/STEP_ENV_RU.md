{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Подготовьте окружение {{ page.platform_name[page.lang] }}, чтобы Deckhouse Platform мог управлять ресурсами в облаке. Полная инструкция приведена [на странице подготовки окружения](/modules/cloud-provider-openstack/environment.html) модуля `cloud-provider-openstack`.

Чтобы получить данные для авторизации, выполните следующие действия на **персональном компьютере**:

1. Откройте [страницу ключей проекта](https://mcs.mail.ru/app/project/keys/) в VK Cloud.
1. Перейдите на вкладку «Доступ по API».
1. Нажмите «Скачать openrc версии 3».
1. Выполните скачанный shell-скрипт. Он задаёт переменные окружения, значения которых используются в параметрах `provider` конфигурации Deckhouse Platform.
