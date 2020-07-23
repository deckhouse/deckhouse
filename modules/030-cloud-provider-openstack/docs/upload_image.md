---
title: "Загрузка image в OpenStack" 
sidebar: modules-cloud-provider-openstack
hide_sidebar: false
---

## Загрузка image в openstack

1. Скачиваем последний стабильный образ ubuntu 18.04
```
curl -L https://cloud-images.ubuntu.com/bionic/current/bionic-server-cloudimg-amd64.img --output ~/ubuntu-18-04-cloud-amd64
```
2. Подготавливаем OpenStack RC (openrc) файл, который содержит credentials для обращения к api openstack.
> Интерфейс получения openrc файла может отличаться в зависмости от провайдера OpenStack. Если провайдер предоставляет
> стандартный интерфейс для OpenStack, то скачать openrc файл можно по [инструкции](https://docs.openstack.org/zh_CN/user-guide/common/cli-set-environment-variables-using-openstack-rc.html)
3. Либо устанавливаем OpenStack cli по [инструкции](https://docs.openstack.org/newton/user-guide/common/cli-install-openstack-command-line-clients.html).
   Либо можно запустить docker контейнер, прокинув внутрь openrc файл и скаченный локально образ ubuntu
```
docker run -ti --rm -v ~/ubuntu-18-04-cloud-amd64:/ubuntu-18-04-cloud-amd64 -v ~/.mcs-openrc:/openrc jmcvea/openstack-client
```
4. Инициализируем переменные окружения из openrc файла
```
source /openrc
```
5. Получаем список доступных типов дисков
```
/ # openstack volume type list
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
6. Создаём image, передаём в образ в качестве свойств тип диска, который будет использоваться, если OpenStack не поддерживает локальные диски или эти диски не подходят для работы 
```
openstack image create --private --disk-format qcow2 --container-format bare --file /ubuntu-18-04-cloud-amd64 --property cinder_img_volume_type=dp1-high-iops ubuntu-18-04-cloud-amd64
```
7. Проверяем, что image успешно создан
```
/ # openstack image show ubuntu-18-04-cloud-amd64
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
