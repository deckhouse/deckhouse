---
title: Подсистема IAM
permalink: ru/architecture/iam/
lang: ru
search: iam, identity and access management, управление идентификацией и доступом
description: Архитектура подсистемы Identity and Access Management в Deckhouse Kubernetes Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

В данном подразделе описывается архитектура подсистемы IAM (Identity and Access Management, идентификация и управление доступом) платформы Deckhouse Kubernetes Platform (DKP).

Подсистема IAM отвечает за следующие функции в DKP:

* [аутентификация пользователей](authentication.html);
* ролевая модель управления доступом (RBAC);
* [мультитенантность](multitenancy.html);
* автоматическое назначение аннотаций и лейблов неймспейсам.

В подсистему IAM входят следующие модули, реализующие описанные выше функции:

* [`user-authn`](/modules/user-authn/) — аутентификация пользователей;
* [`user-authz`](/modules/user-authz/) — ролевая модель управления доступом;
* [`multitenancy-manager`](/modules/multitenancy-manager/) — мультитенантность;
* [`namespace-configurator`](/modules/namespace-configurator/) — автоматическое назначение аннотаций и лейблов неймспейсам.
