---
title: "Cloud provider — OpenStack: подготовка окружения"
description: "Настройка Openstack для работы облачного провайдера Deckhouse."
---

{% include notice_envinronment.liquid %}

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

Чтобы Deckhouse мог управлять ресурсами в облаке OpenStack, ему необходимо подключиться к OpenStack API.  
Перечень API-сервисов OpenStack, доступ до которых необходим для развертывания, доступен в разделе [настройки](./configuration.html#список-необходимых-сервисов-openstack).  
Доступы пользователя, необходимые для подключения к OpenStack API, находятся в openrc-файле (OpenStack RC file).

Информация о получении openrc-файла с помощью стандартного веб-интерфейса OpenStack и о способах его использования доступна в [документации OpenStack](https://docs.openstack.org/ocata/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html#download-and-source-the-openstack-rc-file).

Если вы используете OpenStack API облачного провайдера, интерфейс получения openrc-файла может быть другим.
