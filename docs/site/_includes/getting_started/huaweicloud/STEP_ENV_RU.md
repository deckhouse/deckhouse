{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Для взаимодействия с ресурсами в облаке Huawei Cloud компоненты Deckhouse используют API Huawei Cloud. Для настройки этого подключения требуется создать пользователя в сервисе Huawei Cloud IAM и назначить ему соответствующие права доступа.

## Настройка IAM через веб-интерфейс

Для настройки IAM через веб-интерфейс создайте новую группу пользователей и назначьте ей необходимые права. Для этого выполните следующие шаги:

1. Перейдите в раздел «Identity and Access Management (IAM)».
1. Откройте страницу «User Groups» и нажмите «Create User Group».
1. В поле «Name» укажите имя группы (например, `deckhouse`).
1. Нажмите «OK» для создания группы.
1. Выберите созданную группу из списка.
1. На вкладке «Permissions» нажмите «Authorize».
1. Укажите следующие политики: «ECS Admin», «VPC Administrator», «NAT Admin», «ELB FullAccess», «DEW KeypairFullAccess».
1. Нажмите «Next», затем «OK» и завершите настройку, нажав «Finish».

Добавьте нового пользователя. Для этого выполните следующие шаги:

1. Перейдите на страницу «Users» в разделе IAM и нажмите «Create User».
1. В поле «Username» введите имя пользователя (например, `deckhouse`).
1. Установите «Access type» в значение «Programmatic access» и убедитесь, что «Management console access» отключен.
1. Выберите «Access key» в качестве «Credential Type».
1. Нажмите «Next».
1. Выберите ранее созданную группу пользователей.
1. Нажмите «Create», чтобы завершить создание пользователя.
1. Нажмите «OK», чтобы загрузить `Access Key ID` и `Secret Access Key`. Убедитесь, что вы сохранили эти данные в надёжном месте, так как они понадобятся для доступа к API.

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
