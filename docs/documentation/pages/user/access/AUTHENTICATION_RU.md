---
title: "Аутентификация"
permalink: ru/user/access/authentication.html
lang: ru
description: "Deckhouse Kubernetes Platform. Использование аутентификации."
---

Аутентификация — это процесс проверки подлинности пользователя. В Deckhouse Kubernetes Platform (DKP) реализована сквозная аутентификация, которая позволяет выполнять проверку пользователя при доступе к любым интерфейсам DKP и ресурсам кластера. Пользователь кластера также может использовать DKP для включения аутентификации в своем приложении.

В зависимости от конфигурации DKP, при аутентификации может использоваться как внутренняя база данных, так и внешние источники (провайдеры) аутентификации. Подключение внешнего провайдера аутентификации позволяет использовать учетные данные, например, LDAP, GitLab, GitHub и т.д, для доступа. Также, подключение внешнего провайдера аутентификации позволяет использовать единые учетные данные для аутентификации в нескольких кластерах DKP.

С точки зрения пользователя кластера или разработчика приложения, не имеет значения то, как администратор настроил аутентификацию в DKP — интерфейс аутентификации для пользователя и способы включения аутентификации для приложения будут одинаковые. 

> Чтобы использовать аутентификацию в DKP, необходима [настройка](../../admin/configuration/access/authentication.html).


## Интерфейс

Интерфейс аутентификации открывается при первом обращении к ресурсу, для которого включена аутентификация — DKP перенаправляет пользователя на страницу аутентификации. Если пользователь уже аутентифицирован (например во внешнем провайдере аутентификации), то DKP перенаправит запрос обратно к ресурсу, к которому первоначально обращался пользователь, обогатив запрос данными аутентификации. Если аутентификация не пройдена, пользователь увидит интерфейс аутентификации.

Пример интерфейса аутентификации в DKP:

![Пример интерфейса аутентификации](../../images/user/access/authentication/web-auth-example.png)

Интерфейс аутентификации предлагает выбрать метод аутентификации, если их настроено несколько. Если настроен только один внешний провайдер аутентификации, то пользователь сразу попадет на страницу аутентификации этого провайдера. Если в DKP созданы [локальные пользователи](../../admin/configuration/access/authentication.html#локальная-аутентификация), то DKP предложит ввести логин и пароль.

Пример интерфейса аутентификации в DKP с вводом логина и пароля:

![Пример интерфейса аутентификации с вводом логина и пароля](../../images/user/access/authentication/web-auth-example2.png)


## Включение аутентификации в веб-приложении

> Для работы аутентификации в приложении, аутентификация должна быть настроена на уровне Deckhouse Kubernetes Platform.

В DKP можно включить аутентификацию для приложения двумя способами
В зависимости от того, умеет приложение обрабатывать запросы на аутентификацию (выступать OIDC-клиентом) или нет, в DKP можно включить аутентификацию для приложения двумя способами. Оба они рассматриваются далее.

### Настройка аутентификации для приложения, которое не умеет обрабатывать запросы на аутентификацию

Аутентификация в приложении, которое не умеет самостоятельно обрабатывать запросы на аутентификацию, реализуется с помощью специального прокси-сервера. Он обрабатывает запросы на аутентификацию, также выполняет функции авторизации, скрывая от приложения детали этих процессов.

Чтобы включить аутентификацию для приложения, развернутого в DKP выполните следующие шаги:

1. Создайте объект [DexAuthenticator](TODO) в пространстве имен приложения.

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

2. Добавьте в Ingress-ресурс приложения следующие аннотации:

   - `nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in`
   - `nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email`
   - `nginx.ingress.kubernetes.io/auth-url: https://<NAME>-dex-authenticator.<NS>.svc.{{ C_DOMAIN }}/dex-authenticator/auth`, где:
      - `NAME` — значение параметра `metadata.name` ресурса `DexAuthenticator`;
      - `NS` — значение параметра `metadata.namespace` ресурса `DexAuthenticator`;
      - `C_DOMAIN` — домен кластера (параметр [clusterDomain](../../installing/configuration.html#clusterconfiguration-clusterdomain) ресурса `ClusterConfiguration`).

   Пример (для DexAuthenticator с именем `app-name`, в пространстве имен `app-ns`):

   ```yaml
   annotations:
     nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
     nginx.ingress.kubernetes.io/auth-url: https://app-name-dex-authenticator.app-ns.svc.cluster.local/dex-authenticator/auth
     nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
   ```

### Настойка аутентификации для приложения, которое умеет обрабатывать запросы на аутентификацию

Приложения, которые умеют самостоятельно обрабатывать запросы на аутентификацию и выступать OIDC-клиентом, напрямую взаимодействуют с DKP для аутентификации пользователей.

Чтобы включить аутентификацию для приложения, развернутого в DKP создайте объект [DexClient](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/user-authn/cr.html#dexclient) в пространстве имен приложения.

После появления объекта DexClient, в пространстве имен будет создан набор компонентов, необходимых для работы аутентификации:
- В системе аутентификации DKP будет зарегистрирован клиент с идентификатором (`clientID`) `dex-client-<NAME>@<NAMESPACE>`, где `<NAME>` и `<NAMESPACE>` — `metadata.name` и `metadata.namespace` объекта DexClient соответственно.
- В соответствующем пространстве имен будет создан секрет (Secret) `dex-client-<NAME>` (где `<NAME>` — `metadata.name` объекта DexClient), содержащий пароль доступа к клиенту (clientSecret).


Пример DexClient:
   
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

Пароль доступа к клиенту (`clientSecret`) сохранится в секрете. Пример:
   
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
