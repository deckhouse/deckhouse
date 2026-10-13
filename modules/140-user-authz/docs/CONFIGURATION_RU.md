---
title: "Модуль user-authz: настройки"
---

{% alert level="warning" %}
Не создавайте поды и ReplicaSet'ы напрямую: это второстепенные объекты, их создают контроллеры (Deployment, StatefulSet, Job и другие). Уровни доступа `User`, `PrivilegedUser`, `Editor` и `ClusterEditor` не позволяют создавать и изменять поды, а создавать и изменять ReplicaSet'ы не позволяет ни один уровень ниже `SuperAdmin`.
{% endalert %}

В режиме мультитенантности поля `allowAccessToSystemNamespaces`, `namespaceSelector` и `limitNamespaces` ресурса ClusterAuthorizationRule применяет вебхук авторизации. Если вебхук недоступен, запросы, которые он должен проверить, **отклоняются**. Вебхук не проверяет компоненты control plane, kubelet и ServiceAccount'ы неймспейса `kube-system` и неймспейсов, имена которых начинаются с `d8-`. Подробнее — в разделе [«Упрощённая ролевая модель»](./#упрощённая-ролевая-модель).

Права доступа настраиваются ролями [гранулярной модели](./#гранулярная-ролевая-модель) в объектах RoleBinding и ClusterRoleBinding или [кастомными ресурсами](cr.html) упрощённой модели.

<!-- SCHEMA -->
