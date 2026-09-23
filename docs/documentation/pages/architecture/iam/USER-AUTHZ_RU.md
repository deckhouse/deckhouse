---
title: Модуль user-authz
permalink: ru/architecture/iam/user-authz.html
lang: ru
search: user-authz, RBAC, ролевая модель, авторизация, мультитенантность
description: Архитектура модуля user-authz в Deckhouse Platform.
---

Модуль [`user-authz`](/modules/user-authz/) реализует ролевую модель управления доступом (RBAC) в Deckhouse Platform (DP). Модуль создаёт кластерные роли для управления доступом пользователей и групп пользователей к ресурсам кластера, а в редакциях DP Ultimate, CSE Core и CSE Pro дополнительно поддерживает авторизацию в режиме [мультитенантности](./multitenancy.html).

Подробнее с ролевой моделью управления доступа можно ознакомиться [в соответствующем разделе описания модуля](/modules/user-authz/#гранулярная-ролевая-модель).

Модуль работает со следующими кастомными ресурсами API-группы `deckhouse.io`:

- [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) — задаёт правила доступа на уровне кластера;
- [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule) — задаёт правила доступа в границах одного неймспейса.

В редакциях DP Ultimate, CSE Core и CSE Pro дополнительно работает со следующими кастомными ресурсами API-группы `authorization.deckhouse.io`:

- BulkSubjectAccessReview — ресурс для массовой проверки прав доступа пользователя сразу по нескольким объектам или действиям;
- AccessibleNamespace — ресурс для получения списка неймспейсов, доступных конкретному пользователю;
- WhoCan — ресурс для поиска пользователей, групп и ServiceAccount'ов, которым разрешено выполнить заданное действие;
- SubjectAccessReport — ресурс для получения полного списка прав, предоставленных субъекту;
- RoleAccessReport — ресурс для получения полного списка ресурсов и действий, которые предоставляет роль.

Подробнее с настройками модуля и примерами его использования можно ознакомиться в [соответствующем разделе документации](/modules/user-authz/).

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`user-authz`](/modules/user-authz/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DP изображены на следующей диаграмме:

![Архитектура модуля user-authz](../../images/architecture/iam/c4-l2-user-authz.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **User-authz-controller** (Deployment) — компонент состоит из одного контейнера **user-authz-controller** и следит за кастомными ресурсами ClusterAuthorizationRule и AuthorizationRule, создавая, обновляя и удаляя соответствующие им ClusterRoleBinding и RoleBinding.

Редакции DP Ultimate, CSE Core и CSE Pro дополнительно включают следующие компоненты:

1. **User-authz-webhook** (DaemonSet) — опциональный компонент, реализующий для kube-apiserver [вебхук-режим авторизации](https://kubernetes.io/docs/reference/access-authn-authz/webhook/), включённый в цепочку авторизации между встроенными авторайзерами Node и RBAC. Компонент запускается на всех мастер-узлах в режиме `hostNetwork`.

   Настройку вебхука для авторизации в kube-apiserver выполняет модуль [`control-plane-manager`](/modules/control-plane-manager/), если параметр [`.controlPlaneConfigurator.enabled`](/modules/user-authz/configuration.html#parameters-controlplaneconfigurator-enabled) в настройках модуля принимает значение `true` (по умолчанию). При этом модуль [`control-plane-manager`](/modules/control-plane-manager/) создаёт AuthorizationConfiguration, в параметре `matchConditions` которого исключаются следующие субъекты из проверки вебхуком:

   - основные системные учётные записи control plane, например `kubernetes-admin`;
   - идентификаторы узлов `system:node:*`;
   - ServiceAccount'ы из неймспейсов `kube-system` и `d8-*`.

   Вебхук при получении запроса SubjectAccessReview проверяет ограничения на доступ к неймспейсам, заданные в кастомном ресурсе ClusterAuthorizationRule ([мультитенантность](./multitenancy.html)). При этом вебхук не ограничивает субъекта, у которого есть доступ RBAC независимо от ClusterAuthorizationRule. Таким образом обеспечивается работа работа двух моделей описания доступов.

   Компонент явно запрещает запросы, по всем остальным запросам он не выносит решения (`no opinion`), и решение передаётся дальше авторайзеру RBAC.
   User-authz-webhook настроен по принципу fail-closed, поэтому если вебхук недоступен или не успевает ответить за это время, kube-apiserver запрещает все неисключённые запросы, а не передаёт их RBAC.

   Используются следующие параметры fail-closed для вебхука:

   - параметр `failurePolicy` установлен в `Deny`;
   - таймаут 3 секунды;
   - кеширование решений раздельно для авторизованных и неавторизованных запросов (`authorizedTTL: 5m` / `unauthorizedTTL: 30s`).

   Deckhouse-контроллер разворачивает этот компонент, если параметр [`.enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy) в настройках модуля принимает значение `true` (по умолчанию — `false`).

   Состоит из следующих контейнеров:

   - **user-authz-webhook** — основной контейнер;
   - **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищённого доступа к метрикам компонента.

1. **Permission-browser-apiserver** (Deployment) — опциональный компонент, является [расширением API Kubernetes](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/), публикующее кастомные ресурсы API-группы `authorization.deckhouse.io`.

   Кастомные ресурсы BulkSubjectAccessReview, AccessibleNamespace, WhoCan, SubjectAccessReport и RoleAccessReport API-группы `authorization.deckhouse.io` используются модулем [`console`](/modules/console/) и обеспечивают расширенные возможности по определению прав пользователей, ресурсов или действий.

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
