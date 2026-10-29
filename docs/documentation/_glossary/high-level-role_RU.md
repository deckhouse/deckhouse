---
title: Высокоуровневая роль
description: Роль упрощённой ролевой модели Deckhouse Platform, которую модуль [`user-authz`](/modules/user-authz/) предоставляет поверх RBAC Kubernetes. User, PrivilegedUser, Editor, Admin, ClusterEditor, ClusterAdmin и SuperAdmin.
lang: ru
---

Высокоуровневые роли относятся к упрощённой [ролевой модели](glossary.html#rbac). Модуль [`user-authz`](/modules/user-authz/) создаёт их как набор ClusterRole и назначает пользователям и группам с помощью [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) и [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule). Каждая роль расширяет предыдущую: `User` просматривает поды, логи и Deployment, `Admin` управляет RBAC в [неймспейсе](glossary.html#namespace), `ClusterAdmin` имеет полный доступ к cluster-wide-ресурсам.

Рекомендуемая гранулярная ролевая модель вместо высокоуровневых ролей использует [use-роли](glossary.html#use-role) и [manage-роли](glossary.html#manage-role). Модели не совместимы по ресурсам, автоматическая конвертация невозможна, но их можно использовать одновременно: права обеих моделей суммируются.

Подробнее в разделе [«Упрощённая модель авторизации»](admin/configuration/access/authorization/rbac-current.html).
