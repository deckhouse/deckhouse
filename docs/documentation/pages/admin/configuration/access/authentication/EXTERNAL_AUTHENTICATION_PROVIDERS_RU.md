---
title: "Интеграция с внешними провайдерами аутентификации"
permalink: ru/admin/configuration/access/authentication/external-authentication-providers.html
description: "Интеграция платформы Deckhouse Kubernetes Platform с внешними провайдерами аутентификации включая LDAP, OIDC, GitHub, GitLab, Atlassian Crowd и Bitbucket. Пошаговое руководство по настройке."
lang: ru
---

Подключение внешнего провайдера аутентификации позволяет использовать единые учетные данные для входа в несколько кластеров и одновременно работать с несколькими провайдерами.

DKP поддерживает подключение следующих внешних провайдеров и протоколов аутентификации:

- [LDAP (например, Active Directory)](#интеграция-по-ldap);
- [OIDC (например, Okta, Keycloak, Gluu, Blitz Identity Provider)](#интеграция-по-oidc-openid-connect);
- [GitHub](#интеграция-с-github);
- [GitLab](#интеграция-с-gitlab);
- [Atlassian Crowd](#интеграция-с-atlassiancrowd);
- [Bitbucket Cloud](#интеграция-с-bitbucketcloud).

{% alert level="info" %}
Политика безопасности паролей (требования к сложности, срок действия, история, двухфакторная аутентификация и т.д.) полностью контролируется внешним провайдером аутентификации. Deckhouse не управляет паролями и не вмешивается в реализацию этих политик на стороне провайдера.
{% endalert %}

## Общая схема интеграции

{% alert level="info" %}
[Параметр `allowedGroups`](/modules/user-authn/cr.html#dexprovider-v1-spec-oidc-allowedgroups) в ресурсе DexProvider позволяет ограничить вход только пользователям, входящим в указанные группы.
Если список `allowedGroups` задан, пользователь обязан состоять хотя бы в одной из этих групп — иначе аутентификация будет считаться неуспешной.
Если параметр не указан, фильтрация по группам не применяется.
{% endalert %}

1. Создайте OAuth-приложение у провайдера аутентификации:
    - укажите Redirect URI вида `https://dex.<publicDomainTemplate>/callback`;
    - получите `clientID` и `clientSecret`.

    > **Важно**. При указании Redirect URI подставьте значение `publicDomainTemplate` без `%s`. Например, если указано `publicDomainTemplate: '%s.sandbox1.deckhouse-docs.flant.com'`, то фактический URI будет `https://dex.sandbox20.deckhouse-docs.flant.com/callback`.
    >
    > Для того, чтобы узнать адрес Dex (URI), выполните команду:
    >
    > ```console
    > d8 k -n d8-user-authn get ingress dex -o jsonpath="{.spec.rules[*].host}"
    > ```

1. Создайте [ресурс DexProvider](/modules/user-authn/cr.html#dexprovider) с учётом специфики выбранного провайдера.
1. Включите [модуль `user-authn`](/modules/user-authn/) (если он выключен).

   Сделать это можно как через веб-интерфейс администратора, так и через CLI. Далее приведен пример работы через [Deckhouse CLI](/products/kubernetes-platform/documentation/v1/cli/d8/) (требуется настроенный на работу с кластером контекст kubectl).

   Проверьте статус модуля:

   ```shell
   d8 k get module user-authn
   ```

   Пример вывода:

   ```console
   NAME         STAGE   SOURCE     PHASE       ENABLED   READY
   user-authn           Embedded   Available   True      True
   ```

   Включите модуль через CLI:

   ```shell
   d8 platform module enable user-authn
   ```

1. Настройте [модуль `user-authn`](/modules/user-authn/).

   - Откройте настройки модуля `user-authn` (создайте ресурс ModuleConfig `user-authn`, если его нет):

     ```shell
     d8 k edit mc user-authn
     ```

   - Укажите необходимые параметры модуля в секции `spec.settings`. Подробнее о настройках модуля `user-authn` можно узнать в разделе [справки модуля](/modules/user-authn/).

     Пример конфигурации `user-authn`:

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

### Интеграция по OIDC (OpenID Connect)

Аутентификация через OIDC-провайдера требует регистрации клиента (или создания приложения). Сделайте это по документации вашего провайдера (например, [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_OIDC.htm), [Keycloak](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide), [Gluu](https://gluu.org/docs/gluu-server/4.4/admin-guide/openid-connect/#manual-client-registration) или [Blitz](https://docs.identityblitz.ru/latest/integration-guide/oidc-app-enrollment.html)).

Полученные в ходе выполнения инструкции `clientID` и `clientSecret` укажите в [ресурсе DexProvider](/modules/user-authn/cr.html#dexprovider).

{% alert level="info" %}
При регистрации приложения в любом OIDC-провайдере необходимо указать адрес перенаправления (Redirect URI). Для интеграции с DexProvider используйте следующий формат: `https://dex.<publicDomainTemplate>/callback`, где [`publicDomainTemplate`](../../../../reference/api/global.html#parameters-modules-publicdomaintemplate) — шаблон DNS-имен вашего кластера, определенный в модуле `global`.
{% endalert %}

{% alert level="info" %}
Для обеспечения корректного отзыва токена при выходе из приложения и требования повторной авторизации установите параметр `prompt` в значение `login`. Это гарантирует, что пользователю будет предложено повторно ввести учетные данные при повторной аутентификации.
{% endalert %}

Чтобы настроить детальный (гранулированный) доступ пользователей к приложениям:

- добавьте параметр `allowedUserGroups` в ModuleConfig нужного приложения;
- добавьте соответствующие группы пользователю, используя те же наименования групп как на стороне провайдера, так и на стороне Deckhouse.

#### Keycloak

В процессе настройки Keycloak выберите подходящий `realm`, добавьте пользователя в [Users](https://www.keycloak.org/docs/latest/server_admin/index.html#assembly-managing-users_server_administration_guide) и создайте клиент в разделе [Clients](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide) с включённой [аутентификацией](https://www.keycloak.org/docs/latest/server_admin/index.html#capability-config), необходимой для генерации `clientSecret`. Затем выполните следующие шаги:

1. Создайте в разделе [Client scopes](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes) `scope` с именем `groups`, и назначьте ему предопределенный маппинг `Group Membership` («Client scopes» → «Client scope details» → «Mappers» → «Configure a new mapper»). В поле «Name» и «Token Claim Name» впишите `groups`, в параметре «Full group path» задайте `off`.
1. В созданном ранее клиенте добавьте данный `scope` [во вкладке Client scopes](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes_linking) («Clients → «Client details» → «Client Scopes» → «Add client scope»).
1. В полях «Valid redirect URIs», «Valid post logout redirect URIs» и «Web origins» [конфигурации клиента](https://www.keycloak.org/docs/latest/server_admin/#general-settings) укажите `https://dex.<publicDomainTemplate>/*`, где `publicDomainTemplate` – это [указанный](../../../../reference/api/global.html#parameters-modules-publicdomaintemplate) шаблон DNS-имен кластера в модуле `global`.

Пример настройки провайдера для интеграции с Keycloak:

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
  Это необходимо для корректной обработки значения `true` в поле [`insecureSkipEmailVerified`](/modules/user-authn/cr.html#dexprovider-v1-spec-oidc-insecureskipemailverified) и правильной выдачи прав пользователям с неподтвержденным email.

* Если отредактировать или удалить маппинг `Email verified` невозможно, создайте отдельный Client Scope с именем `email_dkp` (или любым другим) и добавьте в него два маппинга:
  * `email`: «Client Scopes» → `email_dkp` → «Add mapper» → «From predefined mappers» → `email`;
  * `email verified`: «Client Scopes» → `email_dkp` → «Add mapper» → «By configuration» → «Hardcoded claim». Укажите следующие поля:
    * «Name»: `email verified`;
    * «Token Claim Name»: `emailVerified`;
    * «Claim value»: `true`;
    * «Claim JSON Type»: `boolean`.
  
  После этого в клиенте, зарегистрированном для кластера DKP, в разделе «Clients» для `Client scopes` замените значение `email` на `email_dkp`.

  В [ресурсе DexProvider](/modules/user-authn/cr.html#dexprovider) укажите параметр `insecureSkipEmailVerified: true` и в поле `.spec.oidc.scopes` замените название Client Scope на `email_dkp`, следуя примеру:

  ```yaml
  scopes:
   - openid
   - profile
   - email_dkp
   - groups
  ```

#### Blitz Identity Provider

Пример настройки провайдера для интеграции с Blitz Identity Provider:

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
      groups: your_claim # Claim для получения групп пользователя, группы пользователя настраиваются на стороне провайдера Blitz Identity Provider.
    clientID: clientID
    clientSecret: clientSecret
    getUserInfo: true
    insecureSkipEmailVerified: true # Установить true, если нет необходимости в проверке email пользователя.
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

#### Okta

Пример настройки провайдера для интеграции с Okta:

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

После включения интеграции с Okta можно  использовать группы пользователей из Okta, для управления правами. Например, можно задать список групп, пользователи из которых получат доступ к [Grafana](../../../../user/web/grafana.html).

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

### Интеграция по LDAP

Для настройки аутентификации создайте в LDAP учетную запись с правами только на чтение (service account). Эта учетная запись будет использоваться для выполнения поисковых запросов в каталоге LDAP.

В [ресурсе DexProvider](/modules/user-authn/cr.html#dexprovider) укажите следующие параметры:​

- `bindDN`: Полный DN (Distinguished Name) созданного service account. Например: `cn=readonly,dc=example,dc=org`.
- `bindPW`: Пароль для указанного `bindDN`.

{% alert level="info" %}
Если ваш LDAP-сервер позволяет анонимный доступ для выполнения поисковых запросов, параметры `bindDN` и `bindPW` можно опустить. Однако, рекомендуется использовать аутентифицированный доступ для повышения безопасности.

В параметре `bindPW` укажите пароль в открытом виде (plain text). Dex не поддерживает передачу хэшированных паролей в этом параметре.
{% endalert %}

Пример настройки провайдера для интеграции с Active Directory:

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

### Интеграция с GitHub

В организации GitHub необходимо создать новое приложение.

Для этого выполните следующие шаги:

1. Перейдите в «Settings» → «Developer settings» → «OAuth App» → «New OAuth App», и в качестве «Authorization callback URL» укажите адрес `https://dex.<publicDomainTemplate>/callback`.
1. Полученные `Client ID` и `Client Secret` укажите в [ресурсе DexProvider](/modules/user-authn/cr.html#dexprovider).

Если организация GitHub находится под управлением клиента:

1. Перейдите в «Settings» → «Applications» → «Authorized OAuth Apps» → `<имя созданного OAuth-приложения>` и нажмите «Send Request» для подтверждения.
1. Попросите клиента подтвердить запрос, который придет к нему на email.

Пример настройки провайдера для интеграции с GitHub:

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

### Интеграция с GitLab

В GitLab проекта необходимо создать новое приложение.

Для этого выполните следующие шаги:

1. Self-hosted-версия GitLab: перейдите в «Admin Area» → «Applications» → «New application» и в качестве «Redirect URI (Callback url)» укажите адрес `https://dex.<publicDomainTemplate>/callback`, а также выберите «scopes»: `read_user`, `openid`.
1. GitLab Cloud (gitlab.com): под главной учетной записью проекта перейдите в «User Settings» → «Applications» → «Add new application» и в качестве «Redirect URI (Callback url)» укажите адрес `https://dex.<publicDomainTemplate>/callback`, а также выберите «scopes»: `read_user`, `openid`.
1. Полученные `Application ID` и секрет укажите в [ресурсе DexProvider](/modules/user-authn/cr.html#dexprovider).

{% alert level="info" %}
Для GitLab версии 16 и выше включите опцию «Trusted» при создании приложения. Эта опция доступна при создании приложений в «Admin Area» → «Applications». Установка приложения как доверенного позволяет пропустить шаг авторизации для пользователей, что может быть полезно в контролируемых средах.
{% endalert %}

Пример настройки провайдера для интеграции с GitLab:

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

### Интеграция с Atlassian Crowd

В соответствующем проекте Atlassian Crowd необходимо создать новое Generic-приложение.

Для этого выполните следующие шаги:

1. Перейдите в «Applications» → «Add application».
1. Полученные «Application Name» и «Password» укажите в [ресурсе DexProvider](/modules/user-authn/cr.html#dexprovider).
1. При указании групп в ресурсе DexProvider убедитесь, что их названия приведены к нижнему регистру (lowercase). Это необходимо для корректного сопоставления групп между Crowd и Deckhouse.

Пример настройки провайдера для интеграции с Atlassian Crowd:

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

### Интеграция с Bitbucket Cloud

Для настройки аутентификации необходимо в меню команды Bitbucket создать нового OAuth пользователя (consumer).

Для этого выполните следующие шаги:

1. Перейдите в «Personal settings» → «Access management» → «OAuth consumers» → «Add consumer» и в качестве «Callback URL» укажите адрес `https://dex.<publicDomainTemplate>/callback`.
1. Разрешите доступ: «Account: Read» → позволяет получать основную информацию о пользователе (например, email, имя пользователя), «Workspace membership → Read»: позволяет получать информацию о членстве пользователя в рабочих пространствах.
1. Полученные `Key` и секрет укажите в [ресурсе DexProvider](/modules/user-authn/cr.html#dexprovider).

Пример настройки провайдера для интеграции с Bitbucket:

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
