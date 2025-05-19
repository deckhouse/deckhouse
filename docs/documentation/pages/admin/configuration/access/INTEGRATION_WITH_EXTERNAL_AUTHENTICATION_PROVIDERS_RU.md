---
title: "Интеграция с внешними провайдерами аутентификации"
permalink: ru/admin/access/external-authentication-providers.html
lang: ru
---

Подключение внешнего провайдера аутентификации позволяет использовать единые учетные данные для входа в несколько кластеров и одновременно работать с несколькими провайдерами.

DKP поддерживает подключение следующих внешних провайдеров и протоколов аутентификации:
- [LDAP (например, Active Directory)](#интеграция-по-ldap);
- [OIDC (например, Okta, Keycloak, Gluu, Blitz Identity Provider)](#интеграция-по-oidc-openid-connect);
- [GitHub](#интеграция-с-github);
- [GitLab](#интеграция-с-gitlab);
- [Bitbucket Cloud](#интеграция-с-bitbucket-cloud);
- [Atlassian Crowd](#интеграция-с-atlassian-crowd).

{% alert level="info" %}
Политика безопасности паролей (требования к сложности, срок действия, история, двухфакторная аутентификация и т.д.) полностью контролируется внешним провайдером аутентификации. Deckhouse не управляет паролями и не вмешивается в реализацию этих политик на стороне провайдера.
{% endalert %}

### Общая схема интеграции

1. Создайте OAuth-приложение у провайдера аутентификации:
    - укажите Redirect URI вида `https://dex.<publicDomainTemplate>/callback`;
    - получите `clientID` и `clientSecret`.

      {% alert level="warning" %}
      Важно. При указании Redirect URI подставьте значение `publicDomainTemplate` без `%s`. Например, если указано `publicDomainTemplate: '%s.sandbox1.deckhouse-docs.flant.com'`, то фактический URI будет `https://dex.sandbox20.deckhouse-docs.flant.com/callback`.

      Для того, чтобы узнать адрес Dex (URI), выполните команду:
     
      ```console
      kubectl -n d8-user-authn get ingress dex -o jsonpath="{.spec.rules[*].host}"
      ```

      {% endalert %}

1. Создайте ресурс DexProvider с учётом специфики выбранного провайдера.
1. Включите модуль user-authn (если он выключен).

   Сделать это можно как через веб-интерфейс администратора, так и через CLI. Далее приведен пример работы через CLI (требуется `kubectl` настроенный на работу с кластером).

   Проверьте статус модуля:

   ```shell
   kubectl get module user-authn
   ```

   Пример вывода:

   ```console
   kubectl get module user-authn
   NAME         WEIGHT   SOURCE     PHASE   ENABLED   READY
   user-authn   150      Embedded   Ready   True      True
   ```

   Включите модуль через CLI:

   ```shell
   kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable user-authn
   ```

1. Настройте модуль user-authn.

   - Откройте настройки модуля `user-authn` (создайте ресурс moduleConfig `user-authn`, если его нет):

     ```shell
     kubectl edit mc user-authn
     ```

   - Укажите необходимые параметры модуля в секции `spec.settings`. Подробнее о настройках модуля `user-authn` можно узнать в разделе [справки модуля](#TODO).

     Пример конфигурации user-authn:

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

Полученные в ходе выполнения инструкции `clientID` и `clientSecret` укажите в ресурсе DexProvider.

{% alert level="info" %}
При регистрации приложения в любом OIDC-провайдере необходимо указать адрес перенаправления (Redirect URI). Для интеграции с DexProvider используйте следующий формат: `https://dex.<publicDomainTemplate>/callback`, где `publicDomainTemplate` — шаблон DNS-имен вашего кластера, определенный в модуле `global`.
{% endalert %}

{% alert level="info" %}
Для обеспечения корректного отзыва токена при выходе из приложения и требования повторной авторизации установите параметр `prompt` в значение `login`. Это гарантирует, что пользователю будет предложено повторно ввести учетные данные при повторной аутентификации.
{% endalert %}

Чтобы настроить детальный (гранулированный) доступ пользователей к приложениям:

- добавьте параметр `allowedUserGroups` в ModuleConfig нужного приложения;
- добавьте соответствующие группы пользователю, используя те же наименования групп как на стороне провайдера, так и на стороне Deckhouse.

#### Keycloak

После выбора `realm` для настройки, добавления пользователя в [Users](https://www.keycloak.org/docs/latest/server_admin/index.html#assembly-managing-users_server_administration_guide) и создания клиента в разделе [Clients](https://www.keycloak.org/docs/latest/server_admin/index.html#proc-creating-oidc-client_server_administration_guide) с включенной [аутентификацией](https://www.keycloak.org/docs/latest/server_admin/index.html#capability-config), которая необходима для генерации `clientSecret`, выполните следующие шаги:

- Создайте в разделе [Client scopes](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes) `scope` с именем `groups`, и назначьте ему предопределенный маппинг `groups` («Client scopes» → «Client scope details» → «Mappers» → «Add predefined mappers»).
- В созданном ранее клиенте добавьте данный `scope` [во вкладке Client scopes](https://www.keycloak.org/docs/latest/server_admin/#_client_scopes_linking) («Clients → «Client details» → «Client Scopes» → «Add client scope»).
- В полях «Valid redirect URIs», «Valid post logout redirect URIs» и «Web origins» [конфигурации клиента](https://www.keycloak.org/docs/latest/server_admin/#general-settings) укажите `https://dex.<publicDomainTemplate>/*`, где `publicDomainTemplate` – это [указанный](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) шаблон DNS-имен кластера в модуле `global`.

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
    getUserInfo: true
    scopes:
      - openid
      - profile
      - email
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

### Интеграция по LDAP

Для настройки аутентификации создайте в LDAP учетную запись с правами только на чтение (service account). Эта учетная запись будет использоваться для выполнения поисковых запросов в каталоге LDAP.

В ресурсе DexProvider укажите следующие параметры:​

- `bindDN`: Полный DN (Distinguished Name) созданного service account. Например: `cn=readonly,dc=example,dc=org`.
- `bindPW`: Пароль для указанного `bindDN`.

{% alert level="info" %}
Если ваш LDAP-сервер позволяет анонимный доступ для выполнения поисковых запросов, параметры bindDN и bindPW можно опустить. Однако, рекомендуется использовать аутентифицированный доступ для повышения безопасности..

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

- перейдите в «Settings» → «Developer settings» → «OAuth App» → «New OAuth App», и в качестве «Authorization callback URL» укажите адрес `https://dex.<publicDomainTemplate>/callback`;
- полученные `Client ID` и `Client Secret` укажите в ресурсе DexProvider.

Если организация GitHub находится под управлением клиента:

- перейдите в «Settings» → «Applications» → «Authorized OAuth Apps» → `<name of created OAuth App>` и нажмите «Send Request» для подтверждения;
- попросите клиента подтвердить запрос, который придет к нему на email.

Пример настройки провайдера для интеграции с GitHub:

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

### Интеграция с GitLab

В GitLab проекта необходимо создать новое приложение.

Для этого выполните следующие шаги:

- self-hosted: перейдите в «Admin Area» → «Applications» → «New application» и в качестве «Redirect URI (Callback url)» укажите адрес `https://dex.<publicDomainTemplate>/callback`, а также выберите «scopes»: `read_user`, `openid`;
- cloud gitlab.com: под главной учетной записью проекта перейдите в «User Settings» → «Applications» → «Add new application» и в качестве «Redirect URI (Callback url)» укажите адрес `https://dex.<publicDomainTemplate>/callback`, а также выберите «scopes»: `read_user`, `openid`;
- полученные `Application ID` и секрет укажите в ресурсе DexProvider.

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

- перейдите в «Applications» → «Add application»;
- полученные «Application Name» и «Password» укажите в ресурсе DexProvider;
- при указании групп в ресурсе DexProvider убедитесь, что их названия приведены к нижнему регистру (lowercase). Это необходимо для корректного сопоставления групп между Crowd и Deckhouse.

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

- перейдите в «Personal settings» → «Access management» → «OAuth consumers» → «Add consumer» и в качестве «Callback URL» укажите адрес `https://dex.<publicDomainTemplate>/callback`;
- разрешите доступ: «Account: Read» → позволяет получать основную информацию о пользователе (например, email, имя пользователя), «Workspace membership → Read»: позволяет получать информацию о членстве пользователя в рабочих пространствах.
- полученные `Key` и секрет укажите в ресурсе DexProvider.

Пример настройки провайдера для интеграции с Bitbucket:

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
