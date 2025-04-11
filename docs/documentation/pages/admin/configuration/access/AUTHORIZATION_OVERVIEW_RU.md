---
title: "Общие сведения об авторизации в Deckhouse Kubernetes Platform"
permalink: ru/admin/access/authorization-overview.html
lang: ru
---

В Deckhouse Kubernetes Platform авторизация реализована на основе стандартного механизма RBAC (Role-Based Access Control) Kubernetes. Это позволяет гибко настраивать права доступа для различных пользователей, групп и сервисных аккаунтов, обеспечивая безопасность и контроль над операциями в кластере.

В Deckhouse Kubernetes Platform используется две ролевые модели доступа, основанные на базе стандартного механизма RBAC Kubernetes:

- [Текущая](../access/authorization-rbac-current.html). Реализует role-based-подсистему сквозной авторизации, расширяя функционал стандартного механизма RBAC за счет использования кастомных ресурсов, таких как [ClusterAuthorizationRule](../../reference/cr/clusterauthorizationrule/) и [AuthorizationRule](../../reference/cr/authorizationrule/).

- [Экспериментальная](../access/authorization-rbac-experimental.html). Эта модель предполагает использование стандартного для RBAC Kubernetes подхода для настройки доступа: с помощью создания ресурсов `RoleBinding` или `ClusterRoleBinding`, в которых используются роли, подготовленные модулем [user-authz](../../reference/mc/user-authz/). В отличии от текущей модели в экспериментальной не используются  кастомные ресурсы [ClusterAuthorizationRule](../../reference/cr/clusterauthorizationrule/) и [AuthorizationRule](../../reference/cr/authorizationrule/).

Обе модели реализуются с помощью модуля [user-authz](../../reference/mc/user-authz/).

Выбор ролевой модели зависит от ваших потребностей. В этом разделе представлена информация о настройке авторизации с использованием обеих моделей, а также рекомендации по их применению в различных сценариях.
