---
title: "Модуль user-authn: примеры конфигурации"
---

## Пример конфигурации модуля

В примере представлена конфигурация модуля `user-authn` в Deckhouse Kubernetes Platform.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 1
  enabled: true
  settings:
    kubeconfigGenerator:
    - id: direct
      masterURI: https://159.89.5.247:6443
      description: "Direct access to kubernetes API"
    publishAPI:
      enabled: true
```

{% endraw %}

## Примеры настройки провайдера

### GitHub

В примере представлены настройки провайдера для интеграции с GitHub.

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: github
spec:
  type: Github
  displayName: My Company GitHub
  github:
    clientID: plainstring
    clientSecret: plainstring
```

В организации GitHub необходимо создать новое приложение.

Для этого выполните следующие шаги:
* перейдите в `Settings` -> `Developer settings` -> `OAuth Aps` -> `Register a new OAuth application` и в качестве `Authorization callback URL` укажите адрес `https://dex.<modules.publicDomainTemplate>/callback`.

Полученные `Client ID` и `Client Secret` укажите в Custom Resource [DexProvider](cr.html#dexprovider).

Если организация GitHub находится под управлением клиента, перейдите в `Settings` -> `Applications` -> `Authorized OAuth Apps` -> `<name of created OAuth App>` и нажмите `Send Request` для подтверждения. Попросите клиента подтвердить запрос, который придет к нему на email.

### GitLab

В примере представлены настройки провайдера для интеграции с GitLab.

```yaml
apiVersion: deckhouse.io/v1
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

Для этого выполните следующие шаги:
* **self-hosted**: перейдите в `Admin area` -> `Application` -> `New application` и в качестве `Redirect URI (Callback url)` укажите адрес `https://dex.<modules.publicDomainTemplate>/callback`, выберите scopes: `read_user`, `openid`;
* **cloud gitlab.com**: под главной учетной записью проекта перейдите в `User Settings` -> `Application` -> `New application` и в качестве `Redirect URI (Callback url)` укажите адрес `https://dex.<modules.publicDomainTemplate>/callback`, выберите scopes: `read_user`, `openid`;
* (для GitLab версии 16 и выше) включить опцию `Trusted`/`Trusted applications are automatically authorized on Gitlab OAuth flow` при создании приложения.

Полученные `Application ID` и `Secret` укажите в Custom Resource [DexProvider](cr.html#dexprovider).

### Atlassian Crowd

В примере представлены настройки провайдера для интеграции с Atlassian Crowd.

```yaml
apiVersion: deckhouse.io/v1
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

В соответствующем проекте Atlassian Crowd необходимо создать новое `Generic`-приложение.

Для этого выполните следующие шаги:
* перейдите в `Applications` -> `Add application`.

Полученные `Application Name` и `Password` укажите в Custom Resource [DexProvider](cr.html#dexprovider).

Группы CROWD укажите в lowercase-формате для Custom Resource `DexProvider`.

### Bitbucket Cloud

В примере представлены настройки провайдера для интеграции с Bitbucket.

```yaml
apiVersion: deckhouse.io/v1
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

Для этого выполните следующие шаги:
* перейдите в `Settings` -> `OAuth consumers` -> `New application` и в качестве `Callback URL` укажите адрес `https://dex.<modules.publicDomainTemplate>/callback`, разрешите доступ для `Account: Read` и `Workspace membership: Read`.

Полученные `Key` и `Secret` укажите в Custom Resource [DexProvider](cr.html#dexprovider).

### OIDC (OpenID Connect)

Аутентификация через OIDC-провайдера требует регистрации клиента (или создания приложения). Сделайте это по документации вашего провайдера (например, [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration) или [Blitz](https://docs.identityblitz.ru/latest/integration-guide/oidc-app-enrollment.html)).

Полученные в ходе выполнения инструкции `clientID` и `clientSecret` укажите в Custom Resource [DexProvider](cr.html#dexprovider).

Ниже можно ознакомиться с некоторыми примерами.

#### Okta

В примере представлены настройки провайдера для интеграции с Okta:

```yaml
apiVersion: deckhouse.io/v1
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

#### Blitz Identity Provider

На стороне провайдера Blitz Identity Provider при [регистрации приложения](https://docs.identityblitz.ru/latest/integration-guide/oidc-app-enrollment.html) необходимо указать URL для перенаправления пользователя после авторизации. При использовании `DexProvider` необходимо указать `https://dex.<publicDomainTemplate>/`, где `publicDomainTemplate` – [указанный](https://deckhouse.ru/documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) в модуле `global` шаблон DNS-имен кластера.

В примере представлены настройки провайдера для интеграции с Blitz Identity Provider:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: blitz
spec:
  displayName: Blitz Identity Provider
  oidc:
    basicAuthUnsupported: false
    claimMapping:
      email: email
      groups: your_claim # Claim для получения групп пользователя, группы пользователя настраиваются на стороне провайдера Blitz Identity Provider
    clientID: clientID
    clientSecret: clientSecret
    getUserInfo: true
    insecureSkipEmailVerified: true # Установить true, если нет необходимости в проверке email пользователя
    insecureSkipVerify: false
    issuer: https://yourdomain.idblitz.ru/blitz
    promptType: consent 
    scopes:
    - profile
    - openid
    userIDKey: sub
    userNameKey: email
  type: OIDC
```

Чтобы корректно отрабатывал выход из приложений (происходил отзыв токена и требовалась повторная авторизация), нужно установить `login` в значении параметра `promptType`.

Для обеспечения гранулированного доступа пользователя к приложениям необходимо:

* добавить параметр `allowedUserGroups` в `ModuleConfig` нужного приложения;
* добавить группы к пользователю (наименования групп должны совпадать как на стороне Blitz, так и на стороне Deckhouse).

Пример для Prometheus:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  settings:
    auth:
      allowedUserGroups:
        - adm-grafana-access
        - grafana-access
```

### LDAP

В примере представлены настройки провайдера для интеграции с Active Directory:

```yaml
apiVersion: deckhouse.io/v1
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

Для настройки аутентификации заведите в LDAP read-only-пользователя (service account).

Полученные путь до пользователя и пароль укажите в параметрах `bindDN` и `bindPW` Custom Resource [DexProvider](cr.html#dexprovider).
1. Если в LDAP настроен анонимный доступ на чтение, настройки можно не указывать.
2. В параметре `bindPW` укажите пароль в plain-виде. Стратегии с передачей хэшированных паролей не предусмотрены.

## Настройка OAuth2-клиента в Dex для подключения приложения

Этот вариант настройки подходит приложениям, которые имеют возможность использовать OAuth2-аутентификацию самостоятельно, без помощи `oauth2-proxy`.
Чтобы позволить подобным приложениям взаимодействовать с Dex, используется Custom Resource [`DexClient`](cr.html#dexclient).

{% raw %}

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

{% endraw %}

После создания такого ресурса в Dex будет зарегистрирован клиент с идентификатором (**clientID**) `dex-client-myname@mynamespace`.

Пароль доступа к клиенту (**clientSecret**) сохранится в секрете:
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

Придумайте пароль и укажите его хэш-сумму в поле `password`.

Для вычисления хэш-суммы пароля воспользуйтесь командой:

```shell
echo "$password" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
```

Также можно воспользоваться [онлайн-сервисом](https://bcrypt-generator.com/).

{% raw %}

```yaml
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@yourcompany.com
  password: $2a$10$etblbZ9yfZaKgbvysf1qguW3WULdMnxwWFrkoKpRH1yeWa5etjjAa
  ttl: 24h
```

{% endraw %}

## Пример добавления статического пользователя в группу

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: admins
spec:
  name: admins
  members:
    - kind: User
      name: admin
```

{% endraw %}

## Выдача прав пользователю или группе

Для настройки используются параметры в Custom Resource [`ClusterAuthorizationRule`](../../modules/140-user-authz/cr.html#clusterauthorizationrule).
