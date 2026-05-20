---
title: "Модуль service-with-healthchecks: настройки"
---

{% alert level="info" %}

Чтобы создаваемые балансировщики ServiceWithHealthchecks работали, необходимо выполнение следующих условий:

* В сетевой политике пользовательского проекта, в котором будет создаваться ServiceWithHealthchecks, должно присутствовать правило, разрешающее входящий трафик из всех подов неймспейса `d8-service-with-healthchecks`:
  
  ```yaml
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: d8-service-with-healthchecks
  ```

  Подробнее о сетевых политиках — в разделе [«Настройка сетевых политик»](/products/kubernetes-platform/documentation/v1/admin/configuration/network/policy/configuration.html).

* Кластерная роль, которая используется в ClusterRoleBinding и RoleBinding при назначении прав пользователям и сервисным аккаунтам, для ресурса ServiceWithHealthchecks должна быть расширена следующими правилами:

  * `get`
  * `list`
  * `watch`
  * `create`
  * `update`
  * `patch`
  * `delete`.

  Подробнее — в разделе [«Выдача прав пользователям и сервисным аккаунтам»](/products/kubernetes-platform/documentation/latest/admin/configuration/access/authorization/granting.html).

{% endalert %}

{% alert level="warning" %}
После включения модуля не происходит автоматическая замена имеющихся ресурсов типа Service на ServiceWithHealthcheck. Для замены имеющихся сервисов на использование ServiceWithHealthcheck выполните следующие действия:

* Создайте ресурсы ServiceWithHealthcheck с такими же именами и параметрами, как существующие ресурсы Service, которые нужно заменить. При создании ServiceWithHealthcheck укажите обязательные параметры [`healthchecks`](cr.html#servicewithhealthchecks-v1alpha1-spec-healthcheck).
* Удалите ресурсы Service, которые требуется заменить ServiceWithHealthcheck.
{% endalert %}

<!-- SCHEMA -->