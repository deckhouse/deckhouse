---
title: Модуль user-authz
permalink: ru/architecture/iam/user-authz.html
lang: ru
search: user-authz, RBAC, ролевая модель, авторизация, мультитенантность
description: Архитектура модуля user-authz в Deckhouse Kubernetes Platform.
---

Модуль [`user-authz`](/modules/user-authz/) реализует ролевую модель управления доступом (RBAC) в Deckhouse Kubernetes Platform (DKP). Модуль создаёт кластерные роли для управления доступом пользователей и групп пользователей к ресурсам кластера, а в редакциях DKP BE, SE, SE+, EE, CSE Lite и CSE Pro дополнительно поддерживает авторизацию в режиме [мультитенантности](./multitenancy.html).

Модуль работает со следующими кастомными ресурсами API-группы `deckhouse.io`:

- [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) — задаёт правила доступа на уровне кластера;
- [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) — задаёт правила доступа в границах одного неймспейса.

В редакциях DKP BE, SE, SE+, EE, CSE Lite и CSE Pro дополнительно работает со следующими кастомными ресурсами API-группы `authorization.deckhouse.io`:

- BulkSubjectAccessReview — ресурс для массовой проверки прав доступа пользователя сразу по нескольким объектам или действиям;
- AccessibleNamespace — ресурс для получения списка неймспейсов, доступных конкретному пользователю.

Подробнее с настройками модуля и примерами его использования можно ознакомиться в [соответствующем разделе документации](/modules/user-authz/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`user-authz`](/modules/user-authz/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DKP изображены на следующей диаграмме:

![Архитектура модуля user-authz](../../images/architecture/iam/c4-l2-user-authz.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **User-authz-controller** (Deployment) — компонент состоит из одного контейнера **user-authz-controller** и следит за кастомными ресурсами ClusterAuthorizationRule и AuthorizationRule, создавая, обновляя и удаляя соответствующие им ClusterRoleBinding и RoleBinding.

Редакции DKP BE, SE, SE+, EE, CSE Lite и CSE Pro дополнительно включают следующие компоненты:

1. **User-authz-webhook** (DaemonSet) — опциональный компонент, реализующий для kube-apiserver [вебхук-режим авторизации](https://kubernetes.io/docs/reference/access-authn-authz/webhook/). По каждому запросу `SubjectAccessReview` компонент проверяет ограничения на доступ к неймспейсам, заданные в кастомном ресурсе ClusterAuthorizationRule ([мультитенантность](./multitenancy.html)), а решения по остальным запросам делегирует стандартному RBAC. Компонент запускается на всех мастер-узлах в режиме `hostNetwork`.

   Deckhouse-контроллер разворачивает этот компонент, если параметр [`.enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy) в настройках модуля принимает значение `true` (по умолчанию — `false`).

   Настройку вебхука для авторизации в kube-apiserver выполняет модуль [`control-plane-manager`](/modules/control-plane-manager/), если параметр [`.controlPlaneConfigurator.enabled`](/modules/user-authz/configuration.html#parameters-controlplaneconfigurator-enabled) в настройках модуля принимает значение `true` (по умолчанию).

   Состоит из следующих контейнеров:

   - **user-authz-webhook** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам компонента.

1. **Permission-browser-apiserver** (Deployment) — опциональный компонент, является [расширением API Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/), публикующее API-группу `authorization.deckhouse.io` с ресурсами BulkSubjectAccessReview и AccessibleNamespace.

   Кастомные ресурсы BulkSubjectAccessReview и AccessibleNamespace используются модулем [`console`](/modules/console/) для массовой проверки прав пользователя и получения списка доступных ему неймспейсов.

   Deckhouse-контроллер разворачивает этот компонент, если параметр [`.enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy) в настройках модуля принимает значение `true` (по умолчанию — `false`).

   Состоит из следующих контейнеров:

   - **permission-browser-apiserver** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам компонента.

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:

   - следит за кастомными ресурсами ClusterAuthorizationRule и AuthorizationRule;
   - управляет ресурсами ClusterRoleBinding и RoleBinding;
   - читает Namespace, RBAC-ресурсы и discovery API;
   - авторизует запросы на получение метрик.

С модулем взаимодействуют следующие внешние компоненты:

1. **Kube-apiserver**:

   - выполняет авторизацию запросов к кластерным ресурсам;
   - пересылает запросы к API-группе `authorization.deckhouse.io`.

1. **Prometheus-main** — сбор метрик компонентов user-authz-webhook и permission-browser-apiserver.
