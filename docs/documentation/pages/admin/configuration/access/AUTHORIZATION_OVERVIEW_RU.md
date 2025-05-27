---
title: "Обзор"
permalink: ru/admin/access/authorization-overview.html
lang: ru
---

В Deckhouse Kubernetes Platform авторизация реализована на основе стандартного механизма RBAC (Role-Based Access Control) Kubernetes. Это позволяет гибко управлять правами доступа для различных пользователей, групп и сервисных аккаунтов, обеспечивая безопасность и контроль над операциями в кластере.

Платформа поддерживает две ролевые модели:

- [Текущая](../access/authorization-rbac-current.html). Подсистема сквозной авторизации расширяет стандартный RBAC-механизм за счёт пользовательских ресурсов — [ClusterAuthorizationRule](../../reference/cr/clusterauthorizationrule/) и [AuthorizationRule](../../reference/cr/authorizationrule/).
- [Экспериментальная](../access/authorization-rbac-experimental.html). Эта модель также предполагает использование стандартного механизма RBAC. Доступ настраивается с помощью создания ресурсов `RoleBinding` или `ClusterRoleBinding`, в которых используются роли, подготовленные модулем [user-authz](../../reference/mc/user-authz/).

Обе модели поддерживаются модулем [user-authz](../../reference/mc/user-authz/). Выбор модели зависит от требований безопасности и сценариев использования.
