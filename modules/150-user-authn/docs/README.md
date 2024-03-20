---
title: "The user-authn module"
search: kube config generator
webIfaces:
- name: kubeconfig
  urlInfo: faq.html#how-can-i-generate-a-kubeconfig-and-access-kubernetes-api
---

The module sets up a unified authentication system integrated with Kubernetes and Web interfaces used in other modules (Grafana, Dashboard, etc.).

It consists of the following components:
- [dex](https://github.com/dexidp/dex) — is a federated OpenID Connect provider that acts as an identity service for static users and can be linked to one or more ID providers (e.g., SAML providers, GitHub, and Gitlab);
- `kubeconfig-generator` (in fact, [dex-k8s-authenticator](https://github.com/mintel/dex-k8s-authenticator)) — is a helper web application that (being authorized with dex) generates kubectl commands for creating and modifying a kubeconfig;
- `dex-authenticator` (in fact, [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy)) — is an application that gets NGINX Ingress (auth_request) requests and authenticates them with Dex.

Static users are managed using the [`User`](cr.html#user) custom resource. It contains all the user-related data, including the password.

The following external authentication protocols/providers are supported:
- GitHub
- GitLab
- BitBucket Cloud
- Crowd
- LDAP
- OIDC

You can use several external authentication providers simultaneously.

## Integration features

### Базовая аутентификация в API Kubernetes

[Базовая аутентификация](https://en.wikipedia.org/wiki/Basic_access_authentication) в API Kubernetes на данный момент доступна только для провайдера Crowd (с включением параметра [`enableBasicAuth`](cr.html#dexprovider-v1-spec-crowd-enablebasicauth)).

> К API Kubernetes можно подключаться и [через другие поддерживаемые внешние провайдеры](#веб-интерфейс-для-генерации-готовых-kubeconfigов).

### Интеграция с приложениями

Чтобы обеспечить аутентификацию в любом веб-приложении, работающем в Kubernetes, можно создать ресурс [_DexAuthenticator_](cr.html#dexauthenticator) в пространстве имен (_Namespace_) приложения и добавить несколько аннотаций к ресурсу _Ingress_.
Это позволит:
* ограничить список групп, которым разрешен доступ;
* ограничить список адресов, с которых разрешена аутентификация;
* интегрировать приложение в единую систему аутентификации, если приложение поддерживает OIDC. Для этого в Kubernetes создается ресурс [_DexClient_](cr.html#dexclient) в _Namespace_ приложения. В том же _Namespace_ создается секрет с данными для подключения в Dex по OIDC.

После такой интеграции можно:
* ограничить перечень групп, которым разрешено подключаться;
* указать перечень клиентов, OIDC-токенам которых можно доверять (`trustedPeers`).

### Веб-интерфейс для генерации готовых kubeconfig-файлов

Модуль позволяет автоматически создавать конфигурацию для kubectl или других утилит Kubernetes. 

Пользователь получит набор команд для настройки kubectl после авторизации в веб-интерфейсе генератора. Эти команды можно скопировать и вставить в консоль для использования kubectl.
Механизм аутентификации для kubeconfig использует OIDC-токен. OIDC-сессия может продлеваться автоматически, если использованный в Dex провайдер аутентификации поддерживает продление сессий. Для этого в kubeconfig указывается `refresh token`.

Дополнительно можно настроить несколько адресов `kube-apiserver` и сертификаты ЦС (CA) для каждого из них. Например, это может потребоваться, если доступ к кластеру Kubernetes осуществляется через VPN или прямое подключение.

## Публикация API kubernetes через Ingress

Компонент kube-apiserver без дополнительных настроек доступен только во внутренней сети кластера. Этот модуль решает проблему простого и безопасного доступа к API Kubernetes извне кластера. При этом API-сервер публикуется на специальном домене (подробнее см. [раздел о служебных доменах в документации](../../deckhouse-configure-global.html)).

При настройке можно указать:
* перечень сетевых адресов, с которых разрешено подключение;
* перечень групп, которым разрешен доступ к API-серверу;
* Ingress-контроллер, на котором производится аутентификация.

По умолчанию будет сгенерирован специальный сертификат ЦС (CA) и автоматически настроен генератор kubeconfig.

## Расширения от Фланта

Модуль использует модифицированную версию Dex для поддержки:
* групп для статических учетных записей пользователей и провайдера Bitbucket Cloud (параметр [`bitbucketCloud`](cr.html#dexprovider-v1-spec-bitbucketcloud));
* передачи параметра `group` клиентам;
* механизма `obsolete tokens`, который позволяет избежать состояния гонки при продлении токена OIDC-клиентом.

## Отказоустойчивый режим

Модуль поддерживает режим высокой доступности `highAvailability`. При его включении аутентификаторы, отвечающие на `auth request`-запросы, развертываются с учетом требуемой избыточности для обеспечения непрерывной работы. В случае отказа любого из экземпляров аутентификаторов пользовательские аутентификационные сессии не прерываются.

## High availability mode

The module also supports the `highAvailability` mode. When this mode is enabled, all components responding to the `auth requests` are deployed with the redundancy required to operate continuously without failure. Thus, the user authentication sessions are kept alive even if any of the authenticator instances fail.
