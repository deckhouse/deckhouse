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
  version: 2
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
  # Опционально: временно отключить провайдер, не удаляя CR
  # enabled: false
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
  displayName: Dedicated GitLab
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
* **self-hosted**: перейдите в `Admin area` -> `Application` -> `New application` и в качестве `Redirect URI (Callback URL)` укажите адрес `https://dex.<modules.publicDomainTemplate>/callback`, выберите scopes: `read_user`, `openid`;
* **cloud gitlab.com**: под главной учетной записью проекта перейдите в `User Settings` -> `Application` -> `New application` и в качестве `Redirect URI (Callback URL)` укажите адрес `https://dex.<modules.publicDomainTemplate>/callback`, выберите scopes: `read_user`, `openid`;
* (для GitLab версии 16 и выше) включить опцию `Trusted`/`Trusted applications are automatically authorized on GitLab OAuth flow` при создании приложения.

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
  name: bitbucket
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

#### Keycloak

После выбора `realm` для настройки, добавления пользователя в [Users](https://www.keycloak.org/docs/latest/server_admin/index.html#assembly-managing-users_server_administration_guide) и создания клиента в разделе [Clients](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide) с включенной [аутентификацией](https://www.keycloak.org/docs/latest/server_admin/index.html#capability-config), которая необходима для генерации `clientSecret`, выполните следующие шаги:

* Создайте в разделе [Client scopes](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes) `scope` с именем `groups`, и назначьте ему предопределенный маппинг `groups` («Client scopes» → «Client scope details» → «Mappers» → «Add predefined mappers»).
* В созданном ранее клиенте добавьте данный `scope` [во вкладке Client scopes](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes_linking) («Clients → «Client details» → «Client Scopes» → «Add client scope»).
* В полях «Valid redirect URIs», «Valid post logout redirect URIs» и «Web origins» [конфигурации клиента](https://www.keycloak.org/docs/latest/server_admin/#general-settings) укажите `https://dex.<publicDomainTemplate>/*`, где `publicDomainTemplate` – это [указанный](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) шаблон DNS-имен кластера в модуле `global`.

В примере представлены настройки провайдера для интеграции с Keycloak:

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: keycloak
spec:
  type: OIDC
  displayName: My Company Keycloak
  oidc:
    issuer: https://keycloak.my-company.com/realms/myrealm # Используйте имя вашего realm
    clientID: plainstring
    clientSecret: plainstring
    insecureSkipEmailVerified: true
    getUserInfo: true
    scopes:
      - openid
      - profile
      - email
      - groups
```

Если в Keycloak не используется подтверждение учетных записей по email, для корректной работы с ним в качестве провайдера аутентификации внесите изменения в настройку [`Client scopes`](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes_linking) одним из следующих способов:

* Удалите маппинг `Email verified` («Client Scopes» → «Email» → «Mappers»).
  Это необходимо для корректной обработки значения `true` в поле [`insecureSkipEmailVerified`](cr.html#dexprovider-v1-spec-oidc-insecureskipemailverified) и правильной выдачи прав пользователям с неподтвержденным email.

* Если отредактировать или удалить маппинг `Email verified` невозможно, создайте отдельный Client Scope с именем `email_dkp` (или любым другим) и добавьте в него два маппинга:
  * `email`: «Client Scopes» → `email_dkp` → «Add mapper» → «From predefined mappers» → `email`;
  * `email verified`: «Client Scopes» → `email_dkp` → «Add mapper» → «By configuration» → «Hardcoded claim». Укажите следующие поля:
    * «Name»: `email verified`;
    * «Token Claim Name»: `emailVerified`;
    * «Claim value»: `true`;
    * «Claim JSON Type»: `boolean`.
  
  После этого в клиенте, зарегистрированном для кластера DKP, в разделе «Clients» для `Client scopes` замените значение `email` на `email_dkp`.

  В ресурсе DexProvider укажите параметр `insecureSkipEmailVerified: true` и в поле `.spec.oidc.scopes` замените название Client Scope на `email_dkp`, следуя примеру:

  ```yaml
      scopes:
        - openid
        - profile
        - email_dkp
        - groups
  ```

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

На стороне провайдера Blitz Identity Provider при [регистрации приложения](https://docs.identityblitz.ru/latest/integration-guide/oidc-app-enrollment.html) необходимо указать URL для перенаправления пользователя после авторизации. При использовании `DexProvider` необходимо указать `https://dex.<publicDomainTemplate>/`, где `publicDomainTemplate` – [указанный](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) в модуле `global` шаблон DNS-имен кластера.

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

    enableBasicAuth: true

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

#### Настройка базовой аутентификации

Чтобы включить доступ к Kubernetes API с использованием базовой аутентификации (Basic Authentication) по учетным записям LDAP:

1. Убедитесь, что в конфигурации модуля `user-authn` включен параметр [`publishAPI`](configuration.html#parameters-publishapi).
1. Установите параметр [`enableBasicAuth: true`](/modules/user-authn/cr.html#dexprovider-v1-spec-oidc-enablebasicauth) в ресурсе DexProvider для LDAP.

> **Внимание**. В кластере может быть только один провайдер аутентификации с включенным параметром [`enableBasicAuth`](/modules/user-authn/cr.html#dexprovider-v1-spec-oidc-enablebasicauth).

После настройки пользователи смогут обращаться к Kubernetes API с помощью `kubectl`, используя свой логин и пароль в LDAP .

Пример `kubeconfig` для пользователя:

```yaml
apiVersion: v1
kind: Config
clusters:
- name: my-cluster
  cluster:
    server: https://api.example.com
    # Путь к CA сертификату или insecure-skip-tls-verify: true
    certificate-authority: /path/to/ca.crt
users:
- name: ldap-user
  user:
    username: janedoe@example.com
    password: userpassword
contexts:
- name: default
  context:
    cluster: my-cluster
    user: ldap-user
current-context: default
```

#### Kerberos (SPNEGO) SSO для LDAP

Dex поддерживает аутентификацию без отображения формы ввода логина/пароля, которая реализуется с помощью механизма Kerberos (SPNEGO) для LDAP‑коннектора. При использовании этого механизма браузер, доверяющий хосту Dex, отправляет `Authorization: Negotiate …`, Dex валидирует Kerberos‑билет по keytab, пропускает форму вводу логина/пароля, сопоставляет principal с LDAP‑именем, получает группы и завершает OIDC‑поток.

Минимальный пример (расширение спецификации LDAP‑провайдера):

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
    bindDN: cn=Administrator,cn=users,dc=example,dc=com
    bindPW: admin0!
    userSearch:
      baseDN: cn=Users,dc=example,dc=com
      username: sAMAccountName
      idAttr: uid
      emailAttr: mail
      nameAttr: cn
    groupSearch:
      baseDN: cn=Users,dc=example,dc=com
      nameAttr: cn
      userMatchers:
      - userAttr: uid
        groupAttr: memberUid
    kerberos:
      enabled: true
      keytabSecretName: dex-kerberos-keytab   # Секрет в неймспейсе `d8-user-authn` с ключом 'krb5.keytab'.
      expectedRealm: EXAMPLE.COM              # Опционально, проверка realm (без учёта регистра).
      usernameFromPrincipal: sAMAccountName   # localpart|sAMAccountName|userPrincipalName
      fallbackToPassword: false               # По умолчанию false; если true — при отсутствии/ошибке заголовка `Authorization: Negotiate` будет показана форма ввода логина/пароля.
```

Примечания:

* Секрет `dex-kerberos-keytab` должен находиться в неймспейсе `d8-user-authn` и содержать ключ `krb5.keytab`.
* Один под Dex может обслуживать несколько LDAP+Kerberos провайдеров. У каждого — свой keytab. `krb5.conf` не требуется (Dex проверяет билеты офлайн по keytab).
Для настройки аутентификации заведите в LDAP read-only-пользователя (service account).
Полученные путь до пользователя и пароль укажите в параметрах `bindDN` и `bindPW` кастомного ресурса [DexProvider](cr.html#dexprovider). В параметре `bindPW` укажите пароль в открытом виде (plain text). Стратегии с передачей хешированных паролей не предусмотрены.
Если в LDAP настроен анонимный доступ на чтение, настройки можно не указывать.

### SAML

В примере представлены настройки провайдера для интеграции с SAML 2.0 Identity Provider (например, AD FS, Okta, Keycloak).

```yaml
apiVersion: deckhouse.io/v1
kind: DexProvider
metadata:
  name: saml-provider
spec:
  type: SAML
  displayName: Корпоративный SAML
  saml:
    ssoURL: https://saml-idp.example.com/saml/sso
    ca: |
      -----BEGIN CERTIFICATE-----
      MIIFaDC...
      -----END CERTIFICATE-----
    entityIssuer: https://dex.example.com/callback
    ssoIssuer: https://saml-idp.example.com
    usernameAttr: name
    emailAttr: email
    groupsAttr: groups
    nameIDPolicyFormat: persistent
```

Для настройки SAML Identity Provider:

1. Зарегистрируйте Dex как Service Provider (SP) в вашем IdP со следующими параметрами:
   - **ACS URL (Assertion Consumer Service)**: `https://dex.<modules.publicDomainTemplate>/callback`
   - **Entity ID**: `https://dex.<modules.publicDomainTemplate>/callback`
   - **Формат NameID**: `persistent` или `emailAddress`
   - **SLO URL** (опционально): `https://dex.<modules.publicDomainTemplate>/saml/slo/<имя-провайдера>`

1. Настройте маппинг атрибутов в IdP для отправки атрибутов `email`, `name` (имя пользователя) и `groups` в SAML assertion.

1. Экспортируйте сертификат подписи IdP и укажите его в поле `ca` или `caData` ресурса DexProvider.

{% alert level="info" %}
SAML не поддерживает refresh tokens нативно. Dex кеширует identity пользователя из первичного SAML assertion и возвращает её при последующих запросах refresh. Время жизни сессии контролируется настройками `expiry.refreshTokens` в конфигурации модуля `user-authn`.
{% endalert %}

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

## Локальная аутентификация

Локальная аутентификация обеспечивает проверку и управление доступом пользователей с возможностью настройки парольной политики, поддержкой двухфакторной аутентификации (2FA) и управлением группами.  
Реализация соответствует требованиям безопасности ФСТЭК и рекомендациям OWASP, обеспечивая надёжную защиту доступа к кластеру и приложениям без необходимости интеграции с внешними системами аутентификации.

### Создание пользователя

Придумайте пароль и укажите его хеш-сумму, закодированную в base64, в поле `password`. Email-адрес должен быть в нижнем регистре.

Для вычисления хеш-суммы пароля воспользуйтесь командой:

```shell
echo -n '3xAmpl3Pa$$wo#d' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
```

{% alert level="info" %}
Если команда `htpasswd` недоступна, установите соответствующий пакет:

* `apache2-utils` — для дистрибутивов, основанных на Debian;
* `httpd-tools` — для дистрибутивов, основанных на CentOS;
* `apache2-htpasswd` — для ALT Linux.
{% endalert %}

Также можно воспользоваться [онлайн-сервисом](https://bcrypt-generator.com/).

Обратите внимание, что в приведенном примере указан [`ttl`](cr.html#user-v1-spec-ttl).

{% raw %}

```yaml
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@yourcompany.com
  # echo -n '3xAmpl3Pa$$wo#d' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  password: 'JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H'
  ttl: 24h
```

{% endraw %}

### Добавление пользователя в группу

Пользователи могут быть объединены в группы для управления правами доступа. Пример манифеста ресурса Group для группы:

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

Здесь `members` — список пользователей, которые входят в группу.

### Парольная политика

Настройки парольной политики позволяют контролировать сложность пароля, ротацию и блокировку пользователей:

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    passwordPolicy:
      complexityLevel: Fair
      passwordHistoryLimit: 10
      lockout:
        lockDuration: 15m
        maxAttempts: 3
      rotation:
        interval: "30d"
```

{% endraw %}

Описание полей:

* `complexityLevel` — уровень сложности пароля;
* `passwordHistoryLimit` — число предыдущих паролей, которые хранит система, чтобы предотвратить их повторное использование;
* `lockout` — настройки блокировки при превышении лимита неудачных попыток входа:
  * `lockout.maxAttempts` — лимит неудачных попыток;
  * `lockout.lockDuration` — длительность блокировки пользователя;
* `rotation` — настройки ротации паролей:
  * `rotation.interval` — период обязательной смены пароля.

### Двухфакторная аутентификация (2FA)

2FA позволяет повысить уровень безопасности, требуя ввести код из приложения-аутентификатора TOTP (например, Google Authenticator) при входе.

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    staticUsers2FA:
      enabled: true
      issuerName: "awesome-app"
```

{% endraw %}

Описание полей:

* `enabled` — включает или отключает 2FA для всех статических пользователей;
* `issuerName` — имя, которое будет отображаться в приложении-аутентификаторе при добавлении аккаунта.

{% alert level="info" %}
После включения 2FA каждый пользователь должен пройти процесс регистрации в приложении-аутентификаторе при первом входе.
{% endalert %}

### Выдача прав пользователю или группе

Для настройки прав доступа используются параметры кастомного ресурса [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule).
