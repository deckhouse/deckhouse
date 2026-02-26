---
title: Обзор
permalink: ru/architecture/iam/
lang: ru
search: iam, identity and access management, управление идентификацией и доступом
---

В данном подразделе описывается архитектура подсистемы **IAM** (подсистемы идентификации и управления доступом) DKP.

Подсистема **IAM** отвечает за следующую функциональность в DKP:

* [аутентификация](../authentication.html),
* ролевая модель доступа,
* [мультитенантность](../multitenancy/),
* автоматическое назначение аннотаций и лейблов пространствам имён.

В подсистему **IAM** входят следующие модули, реализующие перечисленную выше функциональность:

* [user-authn](/modules/user-authn/) - аутентификация,
* [user-authz](/modules/user-authz/) - ролевая модель доступа,
* [multitenancy-manager](/modules/multitenancy-manager/) - мультитенантность,
* [namespace-configurator](/modules/namespace-configurator/) - автоматическое назначение аннотаций и лейблов пространствам имён.
