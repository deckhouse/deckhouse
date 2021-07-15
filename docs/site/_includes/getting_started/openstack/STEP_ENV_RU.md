Чтобы **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}** смог управлять ресурсами в облаке OpenStack, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации провайдера](https://docs.openstack.org/keystone/pike/admin/cli-keystone-manage-services.html). Здесь мы представим краткую последовательность необходимых действий для получения авторизационных данных на примере облачных сервисов [Mail.ru Cloud Solutions](https://mcs.mail.ru/):
- Перейдите по [ссылке](https://mcs.mail.ru/app/project/keys/);
- На открывшейся странице перейдите во вкладку «API ключи»;
- Нажмите на кнопку «Скачать openrc версии 3»;
- Выполните полученный shell-скрипт, в процессе выполнения которого произойдет создание значений переменных окружения (они будут использованы в параметрах `provider` в конфигурации Deckhouse Platform).
