---
title: "Модуль user-authn: примеры конфигурации"
---

## Пример конфигурации модуля

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
      enable: true
```

{% endraw %}

## Примеры настройки провайдера

### GitHub

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

Для этого необходимо перейти в `Settings` -> `Developer settings` -> `OAuth Aps` -> `Register a new OAuth application` и в качестве `Authorization callback URL` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`.

Полученные `Client ID` и `Client Secret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

В том случае, если организация GitHub находится под управлением клиента, необходимо перейти в `Settings` -> `Applications` -> `Authorized OAuth Apps` -> `<name of created OAuth App>` и запросить подтверждение нажатием на `Send Request`. После попросить клиента подтвердить запрос, который придет к нему на email.

### GitLab

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

Для этого необходимо:
* **self-hosted**: перейти в `Admin area` -> `Application` -> `New application` и в качестве `Redirect URI (Callback url)` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, scopes выбрать: `read_user`, `openid`;
* **cloud gitlab.com**: под главной учетной записью проекта перейти в `User Settings` -> `Application` -> `New application` и в качестве `Redirect URI (Callback url)` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, scopes выбрать: `read_user`, `openid`;
* (для GitLab версии 16 и выше) включить опцию `Trusted`/`Trusted applications are automatically authorized on Gitlab OAuth flow` при создании приложения.

Полученные `Application ID` и `Secret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

### Atlassian Crowd

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

Для этого необходимо перейти в `Applications` -> `Add application`.

Полученные `Application Name` и `Password` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

Группы CROWD указываются в lowercase-формате для custom resource `DexProvider`.

### Bitbucket Cloud

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

Для этого необходимо перейти в `Settings` -> `OAuth consumers` -> `New application` и в качестве `Callback URL` указать адрес `https://dex.<modules.publicDomainTemplate>/callback`, разрешить доступ для `Account: Read` и `Workspace membership: Read`.

Полученные `Key` и `Secret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

### OIDC (OpenID Connect)

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

Аутентификация через OIDC-провайдера требует регистрации клиента (или создания приложения). Сделайте это по документации вашего провайдера (например, [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration)).

Полученные в ходе выполнения инструкции `clientID` и `clientSecret` необходимо указать в custom resource [DexProvider](cr.html#dexprovider).

### LDAP

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

Для настройки аутентификации необходимо завести в LDAP read-only-пользователя (service account).

Полученные путь до пользователя и пароль необходимо указать в полях `bindDN` и `bindPW` custom resource [DexProvider](cr.html#dexprovider).
1. Если в LDAP настроен анонимный доступ на чтение, настройки можно не указывать.
2. В поле `bindPW` необходимо указывать пароль в plain-виде. Стратегии с передачей хэшированных паролей не предусмотрены.

## Настройка OAuth2-клиента в Dex для подключения приложения

Данный вариант настройки подходит приложениям, которые имеют возможность использовать oauth2-аутентификацию самостоятельно, без помощи oauth2-proxy.
Чтобы позволить подобным приложениям взаимодействовать с Dex, используется custom resource [`DexClient`](cr.html#dexclient).

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

Пароль для доступа к клиенту (**clientSecret**) будет сохранен в Secret'е:
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
echo "$password" | htpasswd -inBC 10 "" | tr -d ':\n' | sed 's/$2y/$2a/'
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
