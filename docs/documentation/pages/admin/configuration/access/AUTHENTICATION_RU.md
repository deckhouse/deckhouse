---
title: "Аутентификация"
permalink: ru/admin/configuration/access/authentication.html
lang: ru
---

## Введение

Аутентификация — это процесс проверки подлинности пользователя. В Deckhouse Kubernetes Platform (DKP) реализована сквозная аутентификация, которая позволяет выполнять проверку пользователя при доступе к любым интерфейсам DKP и ресурсам кластера. Пользователь кластера также может использовать DKP для включения аутентификации в своем приложении.

Аутентификация в DKP реализована с помощью федеративного OIDC-провайдера. Подробнее об устройстве аутентификации в DKP можно узнать в разделе [Архитектуры](#TODO - ссылка на архитектуру аутентификации).
  
В основе механизма аутентификации DKP находится федеративный OpenID Connect провайдер `Dex`. В зависимости от конфигурации DKP, при аутентификации может использоваться как [внутренняя база данных](#локальная-аутентификация) (локальная аутентификация), так и [внешние источники](#интеграция-с-внешними-провайдерами-аутентификации) (провайдеры) аутентификации. В случае локальной аутентификации данные пользователей и групп хранятся в специальных ресурсах (User и Group). При этом в ресурсе User сохраняется не пароль в открытом виде, а его зашифрованная хеш-сумма (bcrypt). Подключение внешнего провайдера аутентификации позволяет использовать учетные данные, например, LDAP, GitLab, GitHub и т.д, для доступа, а также  использовать единые учетные данные для аутентификации в нескольких кластерах DKP.

С точки зрения пользователя кластера или разработчика приложения, не имеет значения то, как администратор настроил аутентификацию в DKP — интерфейс аутентификации для пользователя и способы включения аутентификации для приложения будут одинаковые.

С помощью DKP можно:

- Выполнять аутентификацию с помощью локальных (статических) [пользователей и групп](#локальная-аутентификация), созданных в кластере;
- [Интегрироваться](#интеграция-с-внешними-провайдерами-аутентификации) с внешними системами аутентификации;
- Включить [аутентификацию в любом веб-приложении](#общая-схема-интеграции) кластера.
- Организовать [доступ с аутентификацией к Kubernetes API](#доступ-к-kubernetes-api-через-балансировщик-трафика) через балансировщик трафика.

## Доступ к Kubernetes API через балансировщик трафика

С DKP можно использовать аутентификацию при доступе к Kubernetes API. В этом случае, пользователь в web-интерфейсе `kubeconfig` DKP может сгенерировать конфигурацию для kubectl, для безопасного доступа к Kubernetes API через балансировщик трафика (Ingress-контроллер).

Чтобы настроить доступ, выполните следующие шаги:

1. Включите публикацию Kubernetes API. Отредактируйте настройки модуля `user-authn` (если ресурс ModuleConfig не существует — создайте его):

   ```console
   kubectl edit moduleconfig user-authn
   ```

   Добавьте в раздел `settings`:

   ```yaml
   publishAPI:
     enabled: true
   ```

1. Откройте веб-интерфейс kubeconfig. После включения параметра `publishAPI` в модуле `user-authn`, в DKP автоматически активируется веб-интерфейс генерации kubeconfig. Он доступен по URL:

   ```console
   https://kubeconfig.<publicDomainTemplate>
   ```

   Например, если `publicDomainTemplate`: `%s.kube.my`, то URL будет `https://kubeconfig.kube.my`.

1. Сгенерируйте конфигурацию kubectl. После авторизации в интерфейсе kubeconfig пользователь получит набор команд для настройки `kubectl`. Эти команды можно скопировать и вставить в консоль. Аутентификация будет производиться по OIDC-токену, выданному Dex. При поддержке провайдером функции продления сессии, конфигурация будет включать `refresh token`, что позволит продлевать доступ без повторной аутентификации.

1. Настройте несколько точек подключения к API. В конфигурации модуля `user-authn` можно задать несколько точек подключения (kube-apiserver) с описанием и CA-сертификатами для каждой. Это может понадобиться, если кластер доступен через разные сети — например, VPN или публичный IP:

   ```yaml
   settings:
     kubeconfigGenerator:
     - id: direct
       masterURI: https://159.89.5.247:6443
       description: "Direct access to kubernetes API"
   ```

### Как работает защита доступа к Kubernetes API

В Deckhouse Kubernetes Platform вы можете безопасно опубликовать Kubernetes API наружу с помощью Ingress-контроллера, сохранив контроль над доступом. Публикация API и настройка аутентификации осуществляется через модуль `user-authn`. Вы можете настроить:

- Список доверенных IP-адресов или сетей, которым разрешён доступ;
- Список групп пользователей, которые имеют право аутентификации;
- Ingress-контроллер, через который будет осуществляться доступ.

Для настройки:

1. Включите публикацию API.
1. Настройте ограничения доступа. В конфигурации модуля можно указать:
   - Список сетевых адресов, которым разрешён доступ (`allowedSourceRanges`);
   - Список групп пользователей, которым разрешено подключение к Kubernetes API (`allowedUserGroups`);
   - Выбор Ingress-контроллера, через который будет работать публикация (`ingressClass`).
1. Используйте веб-интерфейс kubeconfig. Пользователи смогут получить безопасный доступ к API через kubeconfig, сгенерированный в веб-интерфейсе (`https://kubeconfig.<publicDomainTemplate>`).  Этот kubeconfig будет содержать OIDC-токен и настройки подключения через Ingress.

Что будет настроено автоматически при включении публикации API:

- Deckhouse сам настроит необходимые аргументы для kube-apiserver;
- Будет сгенерирован сертификат CA и добавлен в kubeconfig;
- Настроится вход через Dex с поддержкой OIDC.

## Интеграция с внешними провайдерами аутентификации

Подключение внешнего провайдера аутентификации позволяет использовать единые учетные данные для аутентификации в нескольких кластерах DKP. DKP позволяет подключить более одного провайдера аутентификации одновременно.

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

     > **Важно**. При указании Redirect URI подставьте значение `publicDomainTemplate` без `%s`. Например, если указано `publicDomainTemplate: '%s.sandbox1.deckhouse-docs.flant.com'`, то фактический URI будет `https://dex.sandbox20.deckhouse-docs.flant.com/callback`.
     >
     > Адрес Dex (URI) можно узнать командой:
     >
     > ```console
     > kubectl -n d8-user-authn get ingress dex -o jsonpath="{.spec.rules[*].host}"
     > ```

1. Создайте ресурс DexProvider с учётом специфики выбранного провайдера.
1. Включите модуль user-authn (если он выключен).

   Включить модуль user-authn можно как через веб-интерфейс администратора, так и через CLI. Далее приведен пример работы через CLI (требуется `kubectl` настроенный на работу с кластером).

   Проверить статус модуля:
  
   ```shell
   kubectl get module user-authn
   ```

   Пример вывода:  

   ```console
   kubectl get module user-authn
   NAME         WEIGHT   SOURCE     PHASE   ENABLED   READY
   user-authn   150      Embedded   Ready   True      True
   ```

   Включить модуль через CLI:

   ```shell
   kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable user-authn
   ```

1. Настройте модуль.

   - Откройте настройки модуля `user-authn` (создайте ресурс moduleConfig `user-authn`, если его нет):

     ```shell
     kubectl edit mc user-authn
     ```

   - Укажите необходимые параметры модуля в секции `spec.settings`. Подробнее о возможных настройках модуля `user-authn` можно узнать в разделе [справки модуля](#TODO).

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

### Интеграция по OIDC (OpenID Connect)

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

## Локальная аутентификация

Помимо внешних провайдеров аутентификации, DKP позволяет использовать локальную аутентификацию.

Локальная аутентификация подразумевает создание в кластере объектов User и Group для статических пользователей и групп:

- В объекте User хранится информация о пользователе, включая email и хеш пароля (пароль в явном виде не сохраняется).
- В объекте Group задаётся список пользователей, объединённых в группу.

1. Создание статического пользователя.

   Для создания статического пользователя создайте ресурс User.

   Пример создания ресурса (обратите внимание, что в приведенном примере указан [ttl](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/user-authn/cr.html#user-v1-spec-ttl)):

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

   Придумайте пароль и укажите его хэш-сумму в поле `password`. Пароль хранится в зашифрованном виде (bcrypt).
   Хэш-сумму можно сгенерировать с помощью команды:

   ```console
   echo "$password" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
   ```

1. Добавление пользователя в группу.

   Чтобы объединять статических пользователей в группы, создайте ресурс Group.

   Пример создания ресурса:

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

   `Members` — список пользователей, которые входят в группу (указывается `kind`: User и имя пользователя).

   После создания группы и добавления в неё пользователей, необходимо настроить [авторизацию](authorization.html).
