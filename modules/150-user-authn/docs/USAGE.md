---
title: "Модуль user-authn: примеры конфигурации"
---

## Пример конфигурации модуля

{% raw %}
```yaml
  userAuthnEnabled: "true"
  userAuthn: |
    kubeconfigGenerator:
    - id: direct
      masterURI: https://159.89.5.247:6443
      description: "Direct access to kubernetes API"
    publishAPI:
      enable: true
```
{% endraw %}

## Пример CR `DexAuthenticator`

{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexAuthenticator
metadata:
  name: my-cool-app # поды аутентификатора будут иметь префикс my-cool-app
  namespace: my-cool-namespace # неймспейс, в котором будет развернут dex-authenticator
spec:
  applicationDomain: "my-app.kube.my-domain.com" # домен, на котором висит ваше приложение
  sendAuthorizationHeader: false # отправлять ли `Authorization: Bearer` header приложению, полезно в связке с auth_request в nginx
  applicationIngressCertificateSecretName: "ingress-tls" # имя секрета с tls сертификатом
  applicationIngressClassName: "nginx"
  keepUsersLoggedInFor: "720h"
  allowedGroups:
  - everyone
  - admins
  whitelistSourceRanges:
  - 1.1.1.1
  - 192.168.0.0/24
```
{% endraw %}

После появления CR `DexAuthenticator` в кластере, в указанном namespace'е появятся необходимые deployment, service, ingress, secret.
Чтобы подключить своё приложение к dex, достаточно будет добавить в Ingress-ресурс вашего приложения следующие аннотации:

{% raw %}
```yaml
annotations:
  nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
  nginx.ingress.kubernetes.io/auth-url: https://my-cool-app-dex-authenticator.my-cool-namespace.svc.{{ домен вашего кластера, например | cluster.local }}/dex-authenticator/auth
  nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
```
{% endraw %}

### Настройка ограничений на основе CIDR

В DexAuthenticator нет встроенной системы управления разрешением аутентификации на основе IP адреса пользователя. Вместо этого вы можете воспользоваться аннотациями для Ingress-ресурсов:

* Если нужно ограничить доступ по IP и оставить прохождение аутентификации в dex, добавьте аннотацию с указанием разрешенных CIDR через запятую:
```yaml
nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1`
```
* Если вы хотите, чтобы пользователи из указанных сетей были освобождены от прохождения аутентификации в dex, а пользователи из остальных сетей были обязаны аутентифицироваться в dex - добавьте следующую аннотацию:
```yaml
nginx.ingress.kubernetes.io/satisfy: "any"
```

## Примеры настройки провайдера
### Github

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: github
spec:
  type: Github
  displayName: My Company Github
  github:
    clientID: plainstring
    clientSecret: plainstring
```

В организации GitHub необходимо создать новое приложение.

Для этого необходимо перейти в `Settings` -> `Developer settings` -> `OAuth Aps` -> `Register a new OAuth application` и в качестве `Authorization callback URL` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`.

Полученные `Client ID` и `Client Secret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

В том случае, если организация Github находится под управлением клиента, необходимо перейти в `Settings` -> `Applications` -> `Authorized OAuth Apps` -> `<name of created OAuth App>` и запросить подтверждение нажатием на `Send Request`. После попросить клиента подтвердить запрос, который придет к нему на email.

### GitLab
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: gitlab
spec:
  type: Gitlab
  displayName: Dedicated Gitlab
  gitlab:
    baseURL: https://gitlab.example.com
    clientID: plainstring
    clientSecret: plainstring
    groups:
    - administrators
    - users
```
В GitLab проекта необходимо создать новое приложение.

Для этого необходимо:
* **self-hosted**: перейти в `Admin area` -> `Application` -> `New application` и в качестве `Redirect URI (Callback url)` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, scopes выбрать: `read_user`, `openid`;
* **cloud gitlab.com**: под главной учетной записью проекта перейти в [`User Settings`](https://gitlab.com/profile/) -> `Application` -> `New application` и в качестве `Redirect URI (Callback url)` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, scopes выбрать: `read_user`, `openid`.

Полученные `Application ID` и `Secret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

### Atlassian Crowd
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: crowd
spec:
  type: Crowd
  displayName: Crowd
  crowd:
    baseURL: https://crowd.example.com/crowd
    clientID: plainstring
    clientSecret: plainstring
    enableBasicAuth: true
    groups:
    - administrators
    - users
```

В соответствующем проекте Atlassian Crowd, необходимо создать новое `Generic` приложение.

Для этого необходимо перейти в `Applications` -> `Add application`.

Полученные `Application Name` и `Password` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

### Bitbucket Cloud
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: gitlab
spec:
  type: BitbucketCloud
  displayName: Bitbucket
  bitbucketCloud:
    clientID: plainstring
    clientSecret: plainstring
    includeTeamGroups: true
    teams:
    - administrators
    - users
```

Для настройки аутентификации необходимо в Bitbucket в меню команды создать нового OAuth consumer.

Для этого необходимо перейти в `Settings` -> `OAuth consumers` -> `New application` и в качестве `Callback URL` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, разрешить доступ только для `Account: Read`.

Полученные `Key` и `Secret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

### OIDC (OpenID Connect)
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: okta
spec:
  type: OIDC
  displayName: My Company Okta
  oidc:
    issuer: https://my-company.okta.com
    clientID: plainstring
    clientSecret: plainstring
    insecureSkipEmailVerified: true
    getUserInfo: true
```

Для настройки аутентификации необходимо проконсультироваться с документацией
вашего провайдера для создания приложения.

Полученные в ходе выполнения инструкции `clientID` и `clientSecret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

### LDAP
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DexProvider
metadata:
  name: active-directory
spec:
  type: LDAP
  displayName: Active Directory
  ldap:
    host: ad.example.com:636
    insecureSkipVerify: true

    bindDN: cn=Administrator,cn=users,dc=example,dc=com
    bindPW: admin0!

    usernamePrompt: Email Address

    userSearch:
      baseDN: cn=Users,dc=example,dc=com
      filter: "(objectClass=person)"
      username: userPrincipalName
      idAttr: DN
      emailAttr: userPrincipalName
      nameAttr: cn

    groupSearch:
      baseDN: cn=Users,dc=example,dc=com
      filter: "(objectClass=group)"
      userMatchers:
      - userAttr: DN
        groupAttr: member
      nameAttr: cn
```

Для настройки аутентификации необходимо завести в LDAP read-only пользователя (service account).

Полученные путь до пользователя и пароль необходимо указать в полях `bindDN` и `bindPW` custom resource [DexProvider](cr.html#dexprovider).
1. Если в LDAP настроен анонимный доступ на чтение, настройки можно не указывать.
2. В поле `bindPW` необходимо указывать пароль в plain-виде. Стратегии с передачей хешированных паролей не предусмотрены.

## Настройка OAuth2 клиента в dex для подключения приложения

Данный вариант настройки подходит приложением, которые имеют возможность использовать oauth2-аутентификацию самостоятельно без помощи oauth2-proxy.
Чтобы позволить подобным приложениям взаимодействовать с dex используется Custom Resource [`DexClient`](cr.html#dexclient).

{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
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
{% endraw %}

После создание такого ресурса, в dex будет зарегистрирован клиент с идентификатором (clientID) - `dex-client-myname:mynamespace`

Пароль для доступа к клиенту (clientSecret) будет сохранен в секрете:
{% raw %}
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
{% endraw %}

## Пример создания статического пользователя
{% raw %}
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: User
metadata:
  name: admin
spec:
  email: admin@yourcompany.com
  password: $2a$10$etblbZ9yfZaKgbvysf1qguW3WULdMnxwWFrkoKpRH1yeWa5etjjAa
  userID: some-unique-user-id
  groups:
  - Everyone
  - admins
```
{% endraw %}

## Настройка kube-apiserver

Для работы модулей dashboard и kubeconfig-generator в кластере необходимо настроить kube-apiserver. Для этого предусмотрен специальный модуль [control-plane-configurator](/modules/160-control-plane-configurator/).

{% offtopic title="Аргументы kube-apiserver, которые будут настроены" %}

* --oidc-client-id=kubernetes
* --oidc-groups-claim=groups
* --oidc-issuer-url=https://dex.%addonsPublicDomainTemplate%/
* --oidc-username-claim=email

В случае использования самоподписанных сертификатов для dex будет добавлен ещё один аргумент, а так же в под с apiserver будет смонтирован файл с CA:

* --oidc-ca-file=/etc/kubernetes/oidc-ca.crt
{% endofftopic %}


<!--## Connect an authentication provider using config values-->
<!--## Deploy DexAuthenticator CRD leads to the creation of oauth2 proxy Deployment with various parameters, Ingress, Service and Oauth2 client for accessing dex-->
<!--## Specifying Atlassian Crowd provider with enableBasicAuth option set to true leads to the creation of Deployment with crowd-basic-auth-proxy-->
<!--## Enabling of publishAPI option leads to the creation of Ingress object for apiserver connection with desired ingress-shim annotation-->
<!--## Switching on Control Plane Configurator for the module should add special Configmap to the cluster and generate necessary values-->
<!--## Specifying KubeconfigGenerator in module settings adds parameters to KubeconfigGenerator Configmap-->
<!--## Deploy of DexClient CRD must register oauth2 client entry to dex.-->

