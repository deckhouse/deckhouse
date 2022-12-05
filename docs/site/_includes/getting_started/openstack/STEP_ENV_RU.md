{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Чтобы Deckhouse Platform смог управлять ресурсами в облаке {{ page.platform_name[page.lang] }}, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации](/documentation/v1/modules/030-cloud-provider-openstack/environment.html). Здесь мы представим краткую последовательность необходимых действий (выполняйте их на **[персональном компьютере](step2.html#процесс-установки)**) получения авторизационных данных на примере облачных сервисов [Mail.ru Cloud Solutions](https://mcs.mail.ru/):
- Перейдите по [ссылке](https://mcs.mail.ru/app/project/keys/);
- На открывшейся странице перейдите во вкладку «API ключи»;
- Нажмите на кнопку «Скачать openrc версии 3»;
- Выполните полученный shell-скрипт, в процессе выполнения которого произойдет создание значений переменных окружения (они будут использованы в параметрах `provider` в конфигурации Deckhouse Platform).
