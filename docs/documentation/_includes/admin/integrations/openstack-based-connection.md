Чтобы Deckhouse Kubernetes Platform могла управлять ресурсами в {{ site.data.admin.cloud-types.types[page.cloud_type].name }}, необходимо необходимо подключиться к {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API.

Перечень API-сервисов {{ site.data.admin.cloud-types.types[page.cloud_type].name }}, доступ до которых необходим для развертывания, доступен в разделе [настройки](./configuration.html#список-необходимых-сервисов-openstack).  
Доступы пользователя, необходимые для подключения к {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API, находятся в openrc-файле (OpenStack RC file).

Информация о получении openrc-файла с помощью стандартного веб-интерфейса {{ site.data.admin.cloud-types.types[page.cloud_type].name }} и о способах его использования доступна [в документации {{ site.data.admin.cloud-types.types[page.cloud_type].name }}](https://docs.openstack.org/ocata/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html#download-and-source-the-openstack-rc-file).

Если вы используете {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API cloud-провайдера, интерфейс получения openrc-файла может быть другим.
