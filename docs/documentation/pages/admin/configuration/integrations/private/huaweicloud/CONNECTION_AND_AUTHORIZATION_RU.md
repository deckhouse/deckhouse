---
title: Подключение и авторизация
permalink: ru/admin/integrations/private/huaweicloud/authorization.html
lang: ru
---

## Требования

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

Для корректной работы Deckhouse Kubernetes Platform (DKP) с Huawei Cloud необходимо:

- Убедиться, что на виртуальных машинах установлен пакет `cloud-init`.
- После запуска ВМ должны быть активны следующие службы:
  - `cloud-config.service`;
  - `cloud-final.service`;
  - `cloud-init.service`.

## Доступ к Huawei Cloud API

DKP использует API Huawei Cloud для управления ресурсами. Для настройки доступа необходимо создать IAM-пользователя и предоставить ему необходимые права.

### Создание группы пользователей

Для создания группы пользователей и назначения политик выполните шаги:

1. Перейдите в раздел «Identity and Access Management (IAM)».
1. Откройте вкладку «User Groups» и нажмите «Create User Group».
1. Укажите имя группы, например `deckhouse`, и нажмите «OK».
1. Выберите созданную группу и перейдите на вкладку «Permissions».
1. Нажмите «Authorize» и выберите следующие политики:
   - `ECS Admin`
   - `VPC Administrator`
   - `NAT Admin`
   - `ELB FullAccess`
   - `DEW KeypairFullAccess`
1. Подтвердите действия, нажав «Next», затем «OK», и завершите настройку кнопкой «Finish».

### Создание IAM-пользователя

Для создания IAM-пользователя выполните шаги:

1. Перейдите во вкладку «Users» и нажмите «Create User».
1. Введите имя пользователя, например `deckhouse`.
1. В параметре «Access type» выберите `Programmatic access`, убедитесь, что `Management console access` отключён.
1. В параметре «Credential Type» выберите `Access key`.
1. Нажмите «Next», выберите ранее созданную группу, затем нажмите «Create».
1. Скачайте «Access Key ID» и «Secret Access Key». Эти данные необходимы для подключения к API Huawei Cloud и не могут быть восстановлены позже.

{% alert level="info" %}
Убедитесь, что сохраненные ключи надёжно защищены, так как они используются для подключения к API облака.
{% endalert %}

## JSON-политики

Далее приведено содержание политик в формате JSON:

{% offtopic title="Политика «ECS Admin»" %}

```json
  {
  "Version": "1.1",
  "Statement": [
  {
      "Action": [
      "ecs:*:*",
      "evs:*:get",
      "evs:*:list",
      "evs:volumes:create",
      "evs:volumes:delete",
      "evs:volumes:attach",
      "evs:volumes:detach",
      "evs:volumes:manage",
      "evs:volumes:update",
      "evs:volumes:use",
      "evs:volumes:uploadImage",
      "evs:snapshots:create",
      "vpc:*:get",
      "vpc:*:list",
      "vpc:networks:create",
      "vpc:networks:update",
      "vpc:subnets:update",
      "vpc:subnets:create",
      "vpc:ports:*",
      "vpc:routers:get",
      "vpc:routers:update",
      "vpc:securityGroups:*",
      "vpc:securityGroupRules:*",
      "vpc:floatingIps:*",
      "vpc:publicIps:*",
      "ims:images:create",
      "ims:images:delete",
      "ims:images:get",
      "ims:images:list",
      "ims:images:update",
      "ims:images:upload"
      ],
      "Effect": "Allow"
  }
  ]
  }
```

{% endofftopic %}

{% offtopic title="Политика «VPC Administrator»" %}

```json
  {
      "Version": "1.1",
      "Statement": [
          {
              "Action": [
                  "vpc:vpcs:*",
                  "vpc:routers:*",
                  "vpc:networks:*",
                  "vpc:subnets:*",
                  "vpc:ports:*",
                  "vpc:privateIps:*",
                  "vpc:peerings:*",
                  "vpc:routes:*",
                  "vpc:lbaas:*",
                  "vpc:vpns:*",
                  "ecs:*:get",
                  "ecs:*:list",
                  "elb:*:get",
                  "elb:*:list"
              ],
              "Effect": "Allow"
          }
      ]
  }
```

{% endofftopic %}

{% offtopic title="Политика «NAT Admin»" %}

```json
  {
      "Version": "1.1",
      "Statement": [
          {
              "Action": [
                  "nat:*:*",
                  "vpc:*:*"
              ],
              "Effect": "Allow"
          }
      ]
  }
```

{% endofftopic %}

{% offtopic title="Политика «DEW KeypairFullAccess»" %}

```json
  {
      "Version": "1.1",
      "Statement": [
          {
              "Action": [
                  "kps:domainKeypairs:*",
                  "ecs:serverKeypairs:*"
              ],
              "Effect": "Allow"
          }
      ]
  }
```

{% endofftopic %}

{% offtopic title="Политика «ELB FullAccess»" %}

```json
  {
    "Version": "1.1",
    "Statement": [
        {
            "Action": [
                "elb:*:*",
                "vpc:*:get*",
                "vpc:*:list*",
                "ecs:*:get*",
                "ecs:*:list*"
            ],
            "Effect": "Allow"
        }
    ]
  }
```

{% endofftopic %}
