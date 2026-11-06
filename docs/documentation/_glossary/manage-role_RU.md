---
title: Manage-роль
description: Роль гранулярной ролевой модели для администрирования Deckhouse Platform или одной из её подсистем — cluster-wide-ресурсов, модулей подсистемы и их системных неймспейсов.
lang: ru
---

Manage-роли относятся к гранулярной [ролевой модели](glossary.html#rbac). Manage-роль даёт доступ к cluster-wide-ресурсам Kubernetes, к ModuleConfig и cluster-wide-ресурсам модулей своей [подсистемы](glossary.html#subsystem), а также к [системным неймспейсам](glossary.html#system-namespace), в которых работают эти модули. Доступ к неймспейсам пользовательских приложений она не даёт.

Название manage-роли имеет вид `d8:manage:<SUBSYSTEM>:<ACCESS_LEVEL>`, например `d8:manage:networking:manager`. Подсистема `all` охватывает все модули. Права в неймспейсе назначаются [use-ролями](glossary.html#use-role).

Подробнее в разделе [«Гранулярная модель авторизации»](admin/configuration/access/authorization/rbac-experimental.html).
