---
title: Use-роль
description: Роль гранулярной ролевой модели для назначения прав пользователям в одном неймспейсе, то есть доступа к namespaced-ресурсам модулей и стандартным namespaced-ресурсам Kubernetes.
lang: ru
---

Use-роли относятся к гранулярной [ролевой модели](glossary.html#rbac). Они предназначены для пользователей, как правило разработчиков приложений, которые не управляют кластером и его модулями, но создают Ingress-ресурсы, настраивают аутентификацию приложений или собирают логи в своём [неймспейсе](glossary.html#namespace).

Модуль [`user-authz`](/modules/user-authz/) создаёт роли `d8:use:role:viewer`, `d8:use:role:user`, `d8:use:role:manager` и `d8:use:role:admin`. Use-роль назначается только с помощью RoleBinding. Права администраторов назначаются [manage-ролями](glossary.html#manage-role).

Подробнее в разделе [«Гранулярная модель авторизации»](admin/configuration/access/authorization/rbac-experimental.html).
