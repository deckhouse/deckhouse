---
title: "Модуль user-authn"
description: "Единая система аутентификации Deckhouse Platform, интегрированная с Kubernetes и веб-интерфейсами Deckhouse Platform."
---

Модуль отвечает за единую систему аутентификации, интегрированную с Kubernetes и веб-интерфейсами, используемыми в других модулях, например, Grafana и Dashboard.

## Основные возможности

Модуль предоставляет следующие возможности:

- единый вход в веб-интерфейсы модулей Deckhouse Platform (DP) и в API Kubernetes через Dex;
- вход через внешние провайдеры: GitHub, GitLab, Bitbucket Cloud, Atlassian Crowd, LDAP (с SSO по Kerberos), OIDC и SAML 2.0;
- статические пользователи и группы с парольной политикой, блокировкой учётных записей и двухфакторной аутентификацией;
- аутентификация для любого веб-приложения в кластере с помощью DexAuthenticator и OAuth2-клиенты для приложений с поддержкой OIDC (DexClient);
- генерация kubeconfig в веб-интерфейсе Deckhouse для доступа через `kubectl` с OIDC-токенами;
- базовая аутентификация в API Kubernetes через один провайдер Crowd, OIDC или LDAP.

## Компоненты

Модуль состоит из следующих компонентов:

- [`dex`](https://github.com/dexidp/dex) — федеративный OpenID Connect провайдер, поддерживающий работу со статическими пользователями и с возможностью подключения к различным внешним провайдерам аутентификации, например, SAML, GitLab, GitHub.
- `dex-authenticator` (на основе [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy)) выполняет аутентификацию запросов к приложению, опубликованному через Ingress или через Gateway API модуля `alb`.

Управление статическими пользователями осуществляется с помощью ресурсов [User](cr.html#user) и [Group](cr.html#group):

- в объекте User хранится информация о пользователе, включая email и хеш пароля (пароль в открытом виде не сохраняется);
- в объекте Group задаётся список пользователей, объединённых в группу для управления правами доступа.

Операции управления пользователями — создание и удаление пользователей, управление группами, сброс пароля, сброс 2FA, блокировка и разблокировка — можно выполнять с помощью группы команд [`d8 iam`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam) или объекта [UserOperation](cr.html#useroperation). Объект UserOperation одноразовый и автоматически удаляется через [`retention.userOperations`](configuration.html#parameters-retention-useroperations) (по умолчанию 24 часа).

Поддерживаются следующие внешние провайдеры/протоколы аутентификации:

- GitHub;
- GitLab;
- BitBucket Cloud;
- Crowd;
- LDAP;
- OIDC;
- SAML 2.0.

Одновременно можно подключить более одного внешнего провайдера аутентификации.

## Возможности интеграции

OIDC-аутентификацию в `kube-apiserver` автоматически настраивает модуль [`control-plane-manager`](/modules/control-plane-manager/). Подробнее — [в разделе «Настройка kube-apiserver»](faq.html#настройка-kube-apiserver).

{% alert level="warning" %}
При включении модуля веб-интерфейсы модулей DP выполняют аутентификацию пользователей через Dex вместо HTTP Basic Auth.
{% endalert %}

### Базовая аутентификация в API Kubernetes

[Базовая аутентификация](https://en.wikipedia.org/wiki/Basic_access_authentication) в API Kubernetes доступна для провайдеров Crowd, OIDC и LDAP (параметр `enableBasicAuth` в секции [crowd](cr.html#dexprovider-v1-spec-crowd-enablebasicauth), [oidc](cr.html#dexprovider-v1-spec-oidc-enablebasicauth) или [ldap](cr.html#dexprovider-v1-spec-ldap-enablebasicauth)). Её можно включить только для одного провайдера в кластере. Для провайдеров Crowd и OIDC базовая аутентификация выполняется в обход [ограничений идентичностей провайдера](usage.html#ограничение-идентичностей-провайдера), поэтому включить её может только SuperAdmin.

> К API Kubernetes можно подключаться и [через другие поддерживаемые внешние провайдеры](#генерация-kubeconfig).

### Интеграция с приложениями

Чтобы добавить аутентификацию в любое веб-приложение в кластере, создайте [DexAuthenticator](cr.html#dexauthenticator) в неймспейсе приложения и добавьте аннотации аутентификации в его Ingress или HTTPRoute. DexAuthenticator позволяет ограничить группы и email пользователей, которым разрешён вход, и сети, из которых разрешена аутентификация.

Если приложение само поддерживает OIDC, вместо этого создайте [DexClient](cr.html#dexclient) в его неймспейсе. Модуль регистрирует OAuth2-клиент в Dex и создаёт в том же неймспейсе Secret `dex-client-<NAME>` с секретом клиента. DexClient позволяет ограничить группы и email пользователей, которым разрешён вход, и указать клиентов, токенам которых доверяет приложение (`trustedPeers`).

### Генерация kubeconfig

После входа пользователь может сгенерировать kubeconfig для kubectl и других утилит Kubernetes в [веб-интерфейсе Deckhouse](/products/kubernetes-platform/documentation/v1/user/web/ui.html).
В kubeconfig используется OIDC-токен. Если провайдер аутентификации поддерживает продление сессий, kubeconfig также содержит refresh-токен, и сессия продлевается автоматически. Если при продлении провайдер сообщает, что пользователя больше нет или у него отозван доступ, Dex отклоняет refresh-токен с ошибкой `invalid_grant`, и пользователю нужно войти заново.

Параметр [`kubeconfigGenerator`](configuration.html#parameters-kubeconfiggenerator) добавляет другие способы доступа к API-серверу, каждый со своим адресом и сертификатом CA, например для доступа через Bastion-хост или VPN.

## Дополнительные возможности Dex

В модуле используется Dex с дополнительными возможностями:

* группы статических пользователей (объект [Group](cr.html#group));
* двухфакторная аутентификация (2FA) статических пользователей (параметр [`staticUsers2FA`](configuration.html#parameters-staticusers2fa));
* парольная политика для статических пользователей: сложность, история паролей, ротация и блокировка после неудачных попыток входа (параметр [`passwordPolicy`](configuration.html#parameters-passwordpolicy));
* принудительная смена пароля при следующем входе после сброса пароля администратором;
* блокировка после неудачных попыток входа для пользователей LDAP и Atlassian Crowd (параметр [`passwordPolicy.lockout.applyToConnectors`](configuration.html#parameters-passwordpolicy-lockout-applytoconnectors));
* ограничение частоты неудачных попыток входа с одного IP-адреса (параметр [`rateLimit`](configuration.html#parameters-ratelimit));
* блокировка и разблокировка пользователей любого провайдера с помощью объекта [UserOperation](cr.html#useroperation), в том числе без парольной политики;
* списки разрешённых пользователей и групп в [DexClient](cr.html#dexclient) и [DexAuthenticator](cr.html#dexauthenticator), которые Dex проверяет при входе и при каждом запросе с refresh-токеном;
* ограничение идентичностей, которые может утверждать провайдер (параметр [`spec.allowedIdentities`](cr.html#dexprovider-v1-spec-allowedidentities) DexProvider);
* вход по Kerberos (SPNEGO) через провайдер LDAP без формы ввода пароля, с проверкой билетов из заголовка `Authorization: Negotiate` по keytab сервиса;
* refresh-токены для провайдера SAML;
* собственный сертификат CA для провайдера GitLab (параметр [`gitlab.rootCAData`](cr.html#dexprovider-v1-spec-gitlab-rootcadata)).

## Отказоустойчивый режим

В отказоустойчивом режиме ([`highAvailability`](configuration.html#parameters-highavailability)) Dex и другие компоненты модуля запускаются в нескольких репликах, поэтому вход продолжает работать при отказе одного пода. DexAuthenticator использует ту же настройку, если в нём не задан собственный параметр [`spec.highAvailability`](cr.html#dexauthenticator-v1-spec-highavailability).
