## Список необходимых сервисов OpenStack

Для работы Deckhouse Platform должны быть доступны следующие сервисы {{ site.data.admin.cloud-types.types[page.cloud_type].name }}:

| Сервис                     |                         Версия API                         |
| :------------------------- | :--------------------------------------------------------: |
| Identity (Keystone)        |    [v3](https://docs.openstack.org/api-ref/identity/v3/)   |
| Compute (Nova)             |     [v2.1](https://docs.openstack.org/api-ref/compute/)    |
| Network (Neutron)          |     [v2.0](https://docs.openstack.org/api-ref/network/)    |
| Block Storage (Cinder)     | [v3](https://docs.openstack.org/api-ref/block-storage/v3/) |
| Load Balancing (Octavia) * |   [v2](https://docs.openstack.org/api-ref/load-balancer/)  |

\* Требуется, если в кластере необходимо заказывать балансировщики нагрузки.

{% if page.cloud_type == 'vk-private' or page.cloud_type == 'vk' %}
Адреса и порты API можно узнать [в официальной документации](https://cloud.vk.com/docs/tools-for-using-services/api/rest-api/endpoints).
{% endif %}

## Доступ к API

Для управления ресурсами в {{ site.data.admin.cloud-types.types[page.cloud_type].name }} с помощью Deckhouse Platform необходимо подключиться к {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API.

Доступы пользователя, необходимые для подключения к {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API, находятся в openrc-файле (OpenStack RC file).

Информация о получении openrc-файла с помощью стандартного веб-интерфейса {{ site.data.admin.cloud-types.types[page.cloud_type].name }} и о способах его использования доступна [в документации {{ site.data.admin.cloud-types.types[page.cloud_type].name }}](https://docs.openstack.org/ocata/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html#download-and-source-the-openstack-rc-file).

Если вы используете {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API cloud-провайдера, интерфейс получения openrc-файла может быть другим.

## Проверка поддержки провайдером группы безопасности (security groups)

Выполните команду `openstack security group list`. Если в ответ вы не получите ошибок, это значит, что [группы безопасности](https://docs.openstack.org/nova/pike/admin/security-groups.html) поддерживаются.

## Как загрузить образ в {{ site.data.admin.cloud-types.types[page.cloud_type].name }}

1. Скачайте последний стабильный образ Ubuntu 18.04:

   ```shell
   curl -L https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img --output ~/ubuntu-18-04-cloud-amd64
   ```

1. Подготовьте openrc-файл с учётными данными для доступа к API {{ site.data.admin.cloud-types.types[page.cloud_type].name }}.

   > Интерфейс получения openrc-файла может отличаться в зависимости от провайдера {{ site.data.admin.cloud-types.types[page.cloud_type].name }}. Если провайдер предоставляет
   > стандартный интерфейс для {{ site.data.admin.cloud-types.types[page.cloud_type].name }}, скачать openrc-файл можно [по инструкции](https://docs.openstack.org/ocata/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html#download-and-source-the-openstack-rc-file).

1. Выберите способ запуска клиента OpenStack:

   - Установите клиент локально, согласно [инструкции в документации OpenStack](https://docs.openstack.org/newton/user-guide/common/cli-install-openstack-command-line-clients.html).

   - Запустите контейнер с клиентом, подключив к нему openrc-файл и скачанный образ Ubuntu:

   ```shell
   docker run -ti --rm -v ~/ubuntu-18-04-cloud-amd64:/ubuntu-18-04-cloud-amd64 -v ~/.openrc:/openrc jmcvea/openstack-client
   ```

1. Инициализируйте переменные окружения из openrc-файла:

   ```shell
   source /openrc
   ```

1. Получите список доступных типов дисков:

   ```shell
   openstack volume type list
   ```

   Пример вывода:

   ```console
   +--------------------------------------+---------------+-----------+
   | ID                                   | Name          | Is Public |
   +--------------------------------------+---------------+-----------+
   | 8d39c9db-0293-48c0-8d44-015a2f6788ff | ko1-high-iops | True      |
   | bf800b7c-9ae0-4cda-b9c5-fae283b3e9fd | dp1-high-iops | True      |
   | 74101409-a462-4f03-872a-7de727a178b8 | ko1-ssd       | True      |
   | eadd8860-f5a4-45e1-ae27-8c58094257e0 | dp1-ssd       | True      |
   | 48372c05-c842-4f6e-89ca-09af3868b2c4 | ssd           | True      |
   | a75c3502-4de6-4876-a457-a6c4594c067a | ms1           | True      |
   | ebf5922e-42af-4f97-8f23-716340290de2 | dp1           | True      |
   | a6e853c1-78ad-4c18-93f9-2bba317a1d13 | ceph          | True      |
   +--------------------------------------+---------------+-----------+
   ```

1. Создайте образ и передайте в него в качестве свойств тип диска, который будет использоваться (если {{ site.data.admin.cloud-types.types[page.cloud_type].name }} не поддерживает локальные диски или если эти диски не подходят для работы):

   ```shell
   openstack image create --private --disk-format qcow2 --container-format bare \
     --file /ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=dp1-high-iops ubuntu-18-04-cloud-amd64
   ```

1. Проверьте, что образ успешно создан:

   ```shell
   openstack image show ubuntu-18-04-cloud-amd64
   ```

   Пример вывода:

   ```console
   +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
   | Field            | Value                                                                                                                                                                                                                                                                                     |
   +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
   | checksum         | 3443a1fd810f4af9593d56e0e144d07d                                                                                                                                                                                                                                                          |
   | container_format | bare                                                                                                                                                                                                                                                                                      |
   | created_at       | 2020-01-10T07:23:48Z                                                                                                                                                                                                                                                                      |
   | disk_format      | qcow2                                                                                                                                                                                                                                                                                     |
   | file             | /v2/images/01998f40-57cc-4ce3-9642-c8654a6d14fc/file                                                                                                                                                                                                                                      |
   | id               | 01998f40-57cc-4ce3-9642-c8654a6d14fc                                                                                                                                                                                                                                                      |
   | min_disk         | 0                                                                                                                                                                                                                                                                                         |
   | min_ram          | 0                                                                                                                                                                                                                                                                                         |
   | name             | ubuntu-18-04-cloud-amd64                                                                                                                                                                                                                                                                  |
   | owner            | bbf506e3ece54e21b2acf1bf9db4f62c                                                                                                                                                                                                                                                          |
   | properties       | cinder_img_volume_type='dp1-high-iops', direct_url='rbd://b0e441fc-c317-4acf-a606-cf74683978d2/images/01998f40-57cc-4ce3-9642-c8654a6d14fc/snap', locations='[{u'url': u'rbd://b0e441fc-c317-4acf-a606-cf74683978d2/images/01998f40-57cc-4ce3-9642-c8654a6d14fc/snap', u'metadata': {}}]' |
   | protected        | False                                                                                                                                                                                                                                                                                     |
   | schema           | /v2/schemas/image                                                                                                                                                                                                                                                                         |
   | size             | 343277568                                                                                                                                                                                                                                                                                 |
   | status           | active                                                                                                                                                                                                                                                                                    |
   | tags             |                                                                                                                                                                                                                                                                                           |
   | updated_at       | 2020-05-01T17:18:34Z                                                                                                                                                                                                                                                                      |
   | virtual_size     | None                                                                                                                                                                                                                                                                                      |
   | visibility       | private                                                                                                                                                                                                                                                                                   |
   +------------------+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
   ```

## Замена образа ОС

Образ ОС узлов задаётся параметром [`imageName`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-imagename) ресурса OpenStackInstanceClass для узлов типа `CloudEphemeral` и в секции `instanceClass` ресурса OpenStackClusterConfiguration для узлов типа `CloudPermanent` и master-узлов. Чтобы перевести узлы кластера на новый образ, выполните шаги из раздела [«Как заменить образ ОС в облачном кластере?»](../../../../faq.html#как-заменить-образ-ос-в-облачном-кластере).
