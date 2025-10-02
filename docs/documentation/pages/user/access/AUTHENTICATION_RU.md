---
title: "Аутентификация"
permalink: ru/user/access/authentication.html
lang: ru
description: "Deckhouse Kubernetes Platform. Использование аутентификации."
search: user authentication, authentication methods, user access control, user management, login methods, аутентификация пользователей, методы аутентификации пользователей, контроль доступа, управление пользователями, методы входа
---

## Обзор

Аутентификация — это процесс проверки подлинности пользователя. Deckhouse Kubernetes Platform (DKP) поддерживает сквозную аутентификацию пользователей при доступе к интерфейсам платформы и ресурсам кластера. Этот механизм также может использоваться для включения аутентификации в собственных приложениях, развёрнутых в кластере.

DKP позволяет настраивать аутентификацию как на основе внутренней базы данных, так и с использованием внешних провайдеров — например, LDAP, GitLab или GitHub. Это даёт возможность использовать единую аутентификацию сразу в нескольких кластерах DKP.

С точки зрения пользователя кластера или разработчика приложения, не имеет значения то, как администратор настроил аутентификацию в DKP — интерфейс аутентификации для пользователя и способы включения аутентификации для приложения будут одинаковые.

{% alert level="info" %}

Чтобы использовать аутентификацию в DKP, необходима [настройка](../../admin/configuration/access/authentication/).

{% endalert %}

## Интерфейс

Интерфейс аутентификации открывается при первом обращении к ресурсу, для которого включена аутентификация — DKP перенаправляет пользователя на страницу аутентификации. Если пользователь уже аутентифицирован (например во внешнем провайдере аутентификации), то DKP перенаправит запрос обратно к ресурсу, к которому первоначально обращался пользователь, обогатив запрос данными аутентификации. Если аутентификация не пройдена, пользователь увидит интерфейс аутентификации.

Пример интерфейса аутентификации в DKP:

![Пример интерфейса аутентификации](../../images/user/access/authentication/web-auth-example.png)

Интерфейс аутентификации предлагает выбрать метод аутентификации, если их настроено несколько. Если настроен только один внешний провайдер аутентификации, то пользователь сразу попадет на страницу аутентификации этого провайдера. Если в DKP созданы [локальные пользователи](../../admin/configuration/access/authentication/local.html), то DKP предложит ввести логин и пароль.

Пример интерфейса аутентификации в DKP с вводом логина и пароля:

![Пример интерфейса аутентификации с вводом логина и пароля](../../images/user/access/authentication/web-auth-example2.png)

## Включение аутентификации в веб-приложении

> Для работы аутентификации в приложении, аутентификация должна быть настроена на уровне Deckhouse Kubernetes Platform.

В DKP можно включить аутентификацию для приложения двумя способами
В зависимости от того, умеет приложение обрабатывать запросы на аутентификацию (выступать OIDC-клиентом) или нет, в DKP можно включить аутентификацию для приложения двумя способами. Оба способа рассматриваются далее.

### Аутентификация через прокси (без поддержки OIDC)

Аутентификация в приложении, которое не умеет самостоятельно обрабатывать запросы на аутентификацию, реализуется с помощью специального прокси-сервера. Он обрабатывает запросы на аутентификацию, также выполняет функции авторизации, скрывая от приложения детали этих процессов.

Чтобы включить аутентификацию для приложения, развернутого в DKP выполните следующие шаги:

1. Создайте объект DexAuthenticator в пространстве имен приложения.

   После появления объекта DexAuthenticator, в пространстве имен будет создан набор компонентов, необходимых для работы аутентификации:
   * Deployment, содержащий контейнеры с прокси-сервером аутентификации/авторизации и хранилищем данных Redis;
   * Сервис (Service), ведущий на прокси-сервер аутентификации/авторизации;
   * Ingress-ресурс (Ingress), который принимает запросы по адресу `https://<applicationDomain>/dex-authenticator` и отправляет их в сторону сервиса;
   * Секреты (Secret), необходимые для доступа к системе аутентификации DKP.

   Пример DexAuthenticator:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: DexAuthenticator
   metadata:
     # Префикс имени подов Dex authenticator.
     # Например, если префикс имени `app-name`, то поды Dex authenticator будут вида `app-name-dex-authenticator-7f698684c8-c5cjg`.
     name: app-name
     # Namespace, в котором будет развернут Dex authenticator.
     namespace: app-ns
   spec:
     # Домен вашего приложения. Запросы на него будут перенаправляться для прохождения аутентификацию в Dex.
     applicationDomain: "app-name.kube.my-domain.com"
     # Отправлять ли `Authorization: Bearer` header приложению. Полезно в связке с auth_request в NGINX.
     sendAuthorizationHeader: false
     # Имя Secret'а с SSL-сертификатом.
     applicationIngressCertificateSecretName: "ingress-tls"
     # Название Ingress-класса, которое будет использоваться в создаваемом для Dex authenticator Ingress-ресурсе.
     applicationIngressClassName: "nginx"
     # Время, на протяжении которого пользовательская сессия будет считаться активной.
     keepUsersLoggedInFor: "720h"
     # Список групп, пользователям которых разрешено проходить аутентификацию.
     allowedGroups:
     - everyone
     - admins
     # Список адресов и сетей, с которых разрешено проходить аутентификацию.
     whitelistSourceRanges:
     - 1.1.1.1/32
     - 192.168.0.0/24
   ```

   Обратите внимание на следующие возможности при настройке аутентификации:
   * В параметре `applicationDomain` DexAuthenticator указывается основной домен приложения. Дополнительные домены можно указать в параметре `additionalApplications.domain`;
   * Параметры `whitelistSourceRanges` и `additionalApplications.whitelistSourceRanges` позволяют открыть возможность аутентификации в приложении только для указанного списка IP-адресов;

     О настройке авторизации читайте в разделе [Авторизация](../../admin/configuration/access/authorization/) документации. Все параметры `DexAuthenticator` описаны в разделе [Справка](/modules/user-authn/configuration.html).

1. Добавьте в Ingress-ресурс приложения следующие аннотации:

   - `nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in`
   - `nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email`
   - `nginx.ingress.kubernetes.io/auth-url: https://<NAME>-dex-authenticator.<NS>.svc.{{ C_DOMAIN }}/dex-authenticator/auth`, где:
      - `NAME` — значение параметра `metadata.name` ресурса `DexAuthenticator`;
      - `NS` — значение параметра `metadata.namespace` ресурса `DexAuthenticator`;
      - `C_DOMAIN` — домен кластера (параметр [clusterDomain](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) ресурса `ClusterConfiguration`).

   Пример (для DexAuthenticator с именем `app-name`, в пространстве имен `app-ns`):

   ```yaml
   annotations:
     nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
     nginx.ingress.kubernetes.io/auth-url: https://app-name-dex-authenticator.app-ns.svc.cluster.local/dex-authenticator/auth
     nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
   ```

### Аутентификация для приложений с поддержкой OIDC

Приложения, которые умеют самостоятельно обрабатывать запросы на аутентификацию и выступать OIDC-клиентом, могут напрямую взаимодействовать с системой аутентификации DKP. В этом случае приложение самостоятельно перенаправляет пользователя на страницу входа и обрабатывает полученные OIDC-токены.

Чтобы включить аутентификацию для такого приложения, выполните следующие шаги:

1. Создайте объект [DexClient](/modules/user-authn/cr.html#dexclient) в пространстве имен приложения.

   После создания объекта DexClient, Deckhouse выполнит следующие действия:

   * В системе аутентификации DKP будет зарегистрирован OIDC-клиент с идентификатором (`clientID`) вида:  
     `dex-client-<NAME>@<NAMESPACE>`  
     где `<NAME>` и `<NAMESPACE>` — это `metadata.name` и `metadata.namespace` объекта DexClient;
   * Будет автоматически сгенерирован `clientSecret` и сохранён в виде секрета `dex-client-<NAME>` в том же пространстве имён;
   * Пользователь сможет использовать этот `clientID` и `clientSecret` в своём приложении для настройки OIDC.

1. Укажите допустимые redirect-URI.
   Эти URI определяют, куда провайдер (Dex) может перенаправить пользователя после успешной аутентификации.

1. Ограничьте доступ по группам, если необходимо.  
   Используйте параметр `allowedGroups`, чтобы указать, какие группы пользователей имеют право входа в приложение через этот клиент.

1. (Опционально). Укажите список доверенных клиентов (`trustedPeers`), если вы хотите разрешить делегирование аутентификации между приложениями.

   Пример объекта DexClient:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: DexClient
   metadata:
     name: myname
     namespace: mynamespace
   spec:
     redirectURIs:
     - https://app.example.com/callback
     - https://app.example.com/callback-reserve
     allowedGroups:
     - Everyone
     - admins
     trustedPeers:
     - opendistro-sibling
     ```

1. Получите `clientSecret`.
   Секрет будет создан автоматически:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: dex-client-myname
     namespace: mynamespace
   type: Opaque
   data:
     clientSecret: c2VjcmV0
   ```

1. Настройте своё приложение как OIDC-клиент.
   Используйте `clientID`, `clientSecret`, `redirectURIs`, а также адрес Dex как провайдера. Адрес Dex (`https://dex.<publicDomainTemplate>`) можно получить с помощью команды:

   ```console
   d8 k -n d8-user-authn get ingress dex -o jsonpath="{.spec.rules[*].host}"
   ```
