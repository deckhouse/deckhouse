---
title: "Модуль user-authn" 
search: kube config generator
---

Модуль user-authn
=================

Данный модуль устанавливает [dex](https://github.com/dexidp/dex) в кластер для возможности использования внешней аутентификации как в самом кластере (API) так и во всех веб-интерфейсах (Grafana, Dashboard и др.). 

Модуль состоит из нескольких компонентов:
- [dex](https://github.com/dexidp/dex) — федеративный OpenID Connect провайдер, который подключается к одному или нескольким внешним провайдерам (например, он поддерживает SAML, Gitlab и Github);
- kubeconfig-generator (на самом деле [dex-k8s-authenticator](https://github.com/mintel/dex-k8s-authenticator)) — веб-приложение, которое после авторизации в dex генерирует команды для настройки локального kubectl;
- dex-authenticator (на самом деле [oauth2-proxy](https://github.com/pusher/oauth2_proxy)) — приложение, которое принимает запросы от nginx ingress (auth_request) и производит их аутентификацию в dex.

**Важно!** Так как использование OpenID Connect по протоколу HTTP является слишком значительной угрозой безопасности (что подтверждается, например, тем что kubernetes api-сервер не поддерживает работу с OIDC по HTTP), данный модуль можно установить только при включенном HTTPS (`https.mode` выставить в отличное от `Disabled` значение или на уровне кластера, или в самом модуле).

**Важно!** При включении данного модуля аутентификация во всех веб-интерфейсах перестанет использовать HTTP Basic Auth и переключится на dex (который, в свою очередь, будет использовать настроенные вами внешние провайдеры). 
Для настройки kubectl необходимо перейти по адресу: `https://kubeconfig.<modules.publicDomainTemplate>/`, авторизоваться в настроенном внешнем провайдере и скопировать shell команды к себе в консоль.

**Важно!** Для работы аутентификации в dashboard и kubectl требуется [донастройка API-сервера](#настройка-kube-apiserver). Для автоматизации этого процесса реализован модуль [control-plane-configurator](/modules/160-control-plane-configurator), который включён по-умолчанию.

Конфигурация
------------

### Включение модуля

Модуль по-умолчанию **выключен**. Для включения добавьте в CM `deckhouse`:

```yaml
data:
  userAuthnEnabled: "true"
```

### Параметры

* `providers` — настройки провайдеров аутентификации:
  * `id` — уникальный идентификатор провайдера аутентификации;
  * `name` — имя провайдера, которое будет отображено на странице выбора провайдера для аутентификации (если настроен всего один – эта страница не будет показана);
  * `type` — тип внешнего провайдера, в данный момент поддерживается 6 типов: `Github`, `Gitlab`, `BitbucketCloud`, `Crowd`, `OIDC`, `LDAP`;
  * `github` – параметры провайдера Github (можно указывать только если `type: Github`; как [настроить Github](docs/github.md)):
    * `clientID` — ID организации на Github;
    * `clientSecret` — secret организации на Github;
    * `orgs` — массив названий организаций в Github;
        * `name` — название организации;
        * `teams` — массив команд, допустимых для приема из Github'а, токен пользователя будет содержать объединенное множество команд из Github'а и команд из этого списка (если множество окажется пустым, авторизация будет считаться не успешной), если параметр не указан, токен пользователя будет содержать все команды из Github'а;
    * `teamNameField` — данная опция отвечает за формат команд, которые будут получены из github. Может быть одним из трех вариантов: `name` (default), `slug`, `both`.
      * Если в организации `acme` есть группа `Site Reliability Engineers`, то в случае:
        * `name` будет получена группа с именем `['acme:Site Reliability Engineers']`;
        * `slug` будет получена группа с именем `['acme:site-reliability-engineers']`;
        * `both` будут получены группы с именами `['acme:Site Reliability Engineers', 'acme:site-reliability-engineers']`.
    * `useLoginAsID` — данная опция позволяет вместо использования внутреннего github id, использовать имя пользователя. 
  * `gitlab` – параметры провайдера Gitlab (можно указывать только если `type: Gitlab`; как [настроить Gitlab](docs/gitlab.md)):
    * `clientID` — ID приложения созданного в Gitlab;
    * `clientSecret` — secret приложения созданного в Gitlab;
    * `baseURL` — адрес Gitlab'а (например: `https://fox.flant.com`);
    * `groups` — массив групп, допустимых для приема из Gitlab'а, токен пользователя будет содержать объединенное множество групп из Gitlab'а и групп из этого списка (если множество окажется пустым, авторизация будет считаться не успешной), если параметр не указан, токен пользователя будет содержать все группы из Gitlab'а;
        * Массив групп Gitlab содержит пути групп (path), а не их имена.
  * `bitbucketCloud` – параметры провайдера Bitbucket Cloud (можно указывать только если `type: BitbucketCloud`); 
    * `clientID` — ID приложения созданного в Bitbucket Cloud;
    * `clientSecret` — secret приложения созданного в Bitbucket Cloud;
    * `teams` — массив комманд, допустимых для приема из Bitbucket Cloud'а, токен пользователя будет содержать объединенное множество комманд из Bitbucket Cloud'а и комманд из этого списка (если множество окажется пустым, авторизация будет считаться не успешной), если параметр не указан, токен пользователя будет содержать все комманды из  Bitbucket Cloud'а;
        * Токен будет содержать команды пользователя в claim'е `groups`, как и у других провайдеров.
    * `includeTeamGroups` — при включении данной опции в список команд будут включены все группы команды, в которых состоит пользователь.
        * По-умолчанию `false`.
        * Пример групп пользователя с включенной опцией:
          ```yaml
          groups=["my_team", "my_team/administrators", "my_team/members"]
          ```
  * `crowd` – параметры провайдера Crowd (можно указывать только если `type: Crowd`; как [настроить Crowd](docs/crowd.md)):
    * `baseURL` – адрес Crowd'а (например: `https://crowd.example.com/crowd`);
    * `clientID` – ID приложения созданного в Crowd;
    * `clientSecret` – пароль приложения созданного в Crowd;
    * `groups` – массив групп, допустимых для приема из Crowd'а, токен пользователя будет содержать объединенное множество групп из Crowd'а и групп из этого списка (если множество окажется пустым, авторизация будет считаться не успешной), если параметр не указан, токен пользователя будет содержать все группы из Crowd'а;
    * `enableBasicAuth` - включает возможность basic авторизации для kubernetes api server, в качестве credentials для basic авторизации указываются логин и пароль пользователя из приложения, созданного в Crowd (возможно включить при указании только одного провайдера с типом Crowd), работает ТОЛЬКО при включенном `publishAPI`, полученные от Crowd данные авторизации и групп сохраняются в кэш на 10 секунд; 
  * `ldap` – параметры провайдера LDAP (можно указывать только если `type: LDAP`):
    * `host` — адрес (и опционально порт) для LDAP-сервера;
    * `ca` — CA, используемый для валидации TLS;
    * `insecureSkipVerify` — при включении данной опции не происходит проверка подлинности ответа от провайдера с помощью `ca`;
      * По-умолчанию `false`.
    * `bindDN` — путь до сервис-аккаунта приложения в LDAP (пример: `uid=seviceaccount,cn=users,dc=example,dc=com`);
    * `bindPW` — пароль для сервис-аккаунта приложения в LDAP;
    * `startTLS` — использовать ли [STARTTLS](https://www.digitalocean.com/community/tutorials/how-to-encrypt-openldap-connections-using-starttls) для шифрования;
      * По-умолчанию `false`.
    * `userSearch` — настройки фильтра пользователей, которые помогают сначала отфильтровать директории, в которых будет производится поиск пользователей, а затем найти пользователя по полям (его имени, адресу электронной почты или отображаемому имени), [подробнее о процессе фильтрации можно прочитать в документации](https://github.com/dexidp/dex/blob/3b7292a08fd2c61900f5e6c67f3aa2ee81827dea/Documentation/connectors/ldap.md#example-mapping-a-schema-to-a-search-config):
      * `baseDN` — откуда будет начат поиск пользователей (пример: `cn=users,dc=example,dc=com`)
      * `filter` — опциональное поле, которое позволяет добавить фильтр для директории с пользователями (пример: `(objectClass=person)`);
      * `username` — имя атрибута из которого будет получен username пользователя (пример: `uid`);
      * `idAttr` — имя атрибута из которого будет получен идентификатор пользователя (пример: `uid`);
      * `emailAttr` — имя атрибута из которого будет получен email пользователя (пример: `mail`, указывать обязательно);
      * `nameAttr` — атрибут отображаемого имени пользователя (пример: `name`);
    * `groupSearch` — настройки фильтра для поиска групп для указанного пользователя, [подробнее о процессе фильтрации можно прочитать в документации](https://github.com/dexidp/dex/blob/3b7292a08fd2c61900f5e6c67f3aa2ee81827dea/Documentation/connectors/ldap.md#example-mapping-a-schema-to-a-search-config):
      * `baseDN` — откуда будет начат поиск групп (пример: `cn=groups,dc=freeipa,dc=example,dc=com`);
      * `filter` — опциональное поле, которое позволяет добавить фильтр для директории с группами (пример: `(objectClass=group)`);
      * `nameAttr` — имя атрибута, в котором хранится уникальное имя группы (пример: `name`);
      * `userAttr` — имя атрибута, в котором хранится имя пользователя (пример: `uid`); 
      * `groupAttr` — имя атрибута, в котором хранятся имена пользователей, состоящих в группе (пример: `member`);
  * `oidc` — параметры провайдера OIDC (можно указывать только если `type: OIDC`):
    * `issuer` — адрес провайдера (пример: `https://accounts.google.com`);
    * `clientID` – ID приложения, созданного в OIDC провайдере;
    * `clientSecret` – пароль приложения, созданного в OIDC провайдере;
    * `basicAuthUnsupported` — включение этого параметра означает, что dex для общения с провайдером будет использовать POST запросы вместо добавления токена в Basic Authorization header (в большинстве случаев dex сам определяет, какой запрос ему нужно сделать, но иногда включение этого параметра может помочь);
      * По-умолчанию `false`.
    * `insecureSkipEmailVerified` — при включении данной опции dex перестает обращать внимание на информацию о том, подтвержден e-mail пользователя или нет (как именно подтверждается e-mail решает сам провайдер, в ответе от провайдера приходит лишь информация, подтвержден e-mail или нет);
      * По-умолчанию `false`.
    * `getUserInfo` — запрашивать ли дополнительные данные об успешно подключенном пользователе, подробнее о механизме можно прочитать [здесь](https://openid.net/specs/openid-connect-core-1_0.html#UserInfo));
      * По-умолчанию `false`.
    * `scopes` — список [полей](https://github.com/dexidp/dex/blob/master/Documentation/custom-scopes-claims-clients.md) для включения в ответ при запросе токена;
      * По-умолчанию `["profile", "email"]`
    * `userIDKey` — [Claim](https://openid.net/specs/openid-connect-core-1_0.html#Claims), который будет использован для получения ID пользователя;
      * По-умолчанию `sub`.
    * `userNameKey` — [Claim](https://openid.net/specs/openid-connect-core-1_0.html#Claims), который будет использован для получения имени пользователя;
      * По-умолчанию `name`.  
* `publishAPI` — настройки публикации API-сервера, чeрез ingress:
  * `enable` — если выставить данный параметр в `true`, то в кластере будет создан ingress в namespace d8-user-authn, который выставляет Kubernetes API наружу.
    * По-умолчанию: `false`.
  * `ingressClass` — ingress-класс, который будет использован для публикации API kubernetes через ingress.
  * `whitelistSourceRanges` — массив CIDR, которым разрешено подключение к API.
    * Если параметр не указан, подключение к API не ограничивается по IP.
  * `https` — режим работы https для ingress'а API-сервера:
    * `mode` — режим выдачи сертификатов для данного ingress ресурса. Возможные значения `SelfSigned` и `Global`. В случае использования режима `SelfSigned` для ingress ресурса будет выпущен самоподписанный сертификат. В случае использования `Global` будут применены политики из глобальной настройки `global.modules.https.mode`. Т.е. если в глобальной настройке стоит режим `CertManager` с clusterissuer `letsencrypt`, то для ingress ресурса будет заказан сертификат Lets Encrypt.
      * По-умолчанию: `SelfSigned`
    * `global` — дополнительный параметр для режима `Global`;
      * `kubeconfigGeneratorMasterCA` — если у вас перед ingress'ом есть внешний балансер, который терминирует HTTPS трафик, то тут необходимо вставить CA от сертификата на балансировщике, что бы kubectl мог достучаться до API-сервера; 
          * В качестве CA можно указать сам сертификат с внешнего балансера, если по какой-то причине вы не можете получить подписавший его CA. Но нужно помнить, что после обновления сертификата на балансере сгенерированные ранее kubeconfig'и перестанут работать.
* `kubeconfigGenerator` — массив, в котором указываются дополнительные возможные способы доступа к API. Это может быть полезно, в случае если вы не хотите предоставить доступ к API-кластера через ingress, а хотите предоставить доступ другими способами (например, с бастион-хоста или через OpenVPN).
  * `id` — имя способа доступа к API-серверу (без пробелов, маленькими буквами);
  * `masterURI` — адрес API-сервера;
    * Если вы планируете использовать TCP прокси, то для адреса TCP-прокси должен быть сконфигурирован сертификат на стороне API-сервера. Например, в случае, если у вас API-сервера'а слушают на трех разных адресах (`192.168.0.10`, `192.168.0.11` и `192.168.0.12`), а ходить к API-серверу клиент будет, через TCP-балансер (пусть будет `192.168.0.15`), то вам необходимо перегенерировать сертификаты для API-серверов:
      * отредактировать `kubeadm-config`: `kubectl -n kube-system edit configmap kubeadm-config` добавив в `.apiServer.certSANs` адрес `192.168.0.15`;
      * сохранить получившийся конфиг: `kubeadm config view > kubeadmconf.yaml`;
      * удалить старые сертификаты API-сервера: `mv /etc/kubernetes/pki/apiserver.* /tmp/`;
      * перевыпустить новые сертификаты: `kubeadm init phase certs apiserver --config=kubeadmconf.yaml`;
      * перезапустить контейнер с API-сервером: `docker ps -a | grep 'kube-apiserver' | grep -v pause| awk '{print $1}' | xargs docker restart`;
      * повторить данное действие для всех мастеров.
  * `masterCA` — CA для доступа к API.
    * Если данный параметр не указать, то будет автоматически использован Kubernetes CA.
    * При публикации через HTTP-прокси, который терминирует HTTPS трафик, рекомендуется использовать самоподписанный сертификат, который и указать в настоящем параметре.
  * `description` — описание способа доступа к API-серверу, которое показывается пользователю (в списке).
* `idTokenTTL` — данный параметр отвечает за время жизни id токена (указывается с суффиксом s, m или h);
  * По-умолчанию — 10 минут.
  * Пример: `1h`
* `highAvailability` — ручное управление [режимом отказоустойчивости](/FEATURES.md#отказоустойчивость).
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role.flant.com/vsphere-csi-driver":""}` или `{"node-role.flant.com/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет настроено значение `[{"key":"dedicated.flant.com","operator":"Equal","value":"vsphere-csi-driver"},{"key":"dedicated.flant.com","operator":"Equal","value":"system"}]`.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
* `ingressClass` — класс ingress контроллера, который используется для dex и kubeconfig-generator.
  * Опциональный параметр, по-умолчанию используется глобальное значение `modules.ingressClass`.
* `https` — выбираем, какой тип сертификата использовать для dex и kubeconfig-generator.
  * При использовании этого параметра полностью переопределяются глобальные настройки `global.modules.https`.
  * `mode` — режим работы HTTPS:
    * `Disabled` — при данном значении модуль автоматически отключается.
    * `CertManager` — dex и kubeconfig-generator будут работать по https и заказывать сертификат с помощью clusterissuer заданном в параметре `certManager.clusterIssuerName`;
    * `CustomCertificate` — dex и kubeconfig-generator будут работать по https используя сертификат из namespace `d8-system`;
    * `OnlyInURI` — dex и kubeconfig-generator будут работать по http (подразумевая, что перед ними стоит внешний https балансер, который терминирует https) и все ссылки в `user-authn` будут генерироваться с https схемой.
  * `certManager`
    * `clusterIssuerName` — указываем, какой ClusterIssuer использовать для dex и kubeconfig-generator (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
  * `customCertificate`
    * `secretName` — указываем имя secret'а в namespace `d8-system`, который будет использоваться для dex и kubeconfig-generator (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets)).
* `controlPlaneConfigurator` — настройки параметров для модуля автоматической настройки kube-apiserver [control-plane-configurator](/modules/160-control-plane-configurator).
  * `enabled` — использовать ли control-plane-configurator для настройки OIDC в kube-apiserver.
    * По-умолчанию `true`.
  * `dexCAMode` — как вычислить CA, который будет использован при настройке kube-apiserver.
    * Значения:
      * `FromIngressSecret` — извлечь CA или сам сертификат из секрета, который используется в ингрессе. Если вы используете самоподписанные сертификаты на ингрессах — это ваш вариант.
      * `Custom` — использовать CA указанный явно, в параметре `dexCustomCA` (см. ниже). Этот вариант уместен, например, если вы используете внешний https-балансер перед ингрессами и на этом балансировщике используется самоподписанный сертификат.
      * `DoNotNeed` — CA не требуется (например, при использовании публичного LE или других TLS-провайдеров).
    * По-умолчанию — `DoNotNeed`.
  * `dexCustomCA` — CA, которая будет использована в случае `dexCAMode` = `Custom`.
    * Формат — обычный текст, без base64.
    * Необязательный параметр.

### Пример конфигурации

```yaml
  userAuthnEnabled: "true"
  userAuthn: |
    providers:
    - id: github
      name: Github Company
      type: Github
      github:
        clientID: 7d70961e35f46d220784b8
        clientSecret: db22a757102403199cza4d568404f67548b6f20a3
        orgs:
        - name: devops-company
          teams:
          - devops
    - id: gitlab-fox
      name: Flant Gitlab
      type: Gitlab
      gitlab:
        baseURL: https://fox.flant.com
        clientID: 480dc4611c987b4997s605821ea3e79957be2a15cdz1664149643014a7c619c6379
        clientSecret: 22d134403a3a446fsee57a4a4d6262ba33fb1511375665fa76028d3039c307c9aca
    kubeconfigGenerator:
    - id: direct
      masterURI: https://159.89.5.247:6443
      description: "Direct access to kubernetes API"
    publishAPI:
      enable: true
```
### Настройка Dex-аутентификатора
Для автоматического деплоя [oauth2-proxy](https://github.com/pusher/oauth2_proxy) в namespace вашего приложения, и подключения его к dex, реализован CRD `DexAuthenticator`. 

При появлении объекта DexAuthenticator в неймспейсе будут созданы:
* Deployment с oauth2-proxy и redis
* Service, ведущий на Deployment с oauth2-proxy
* Ingress, который принимает запросы по адресу `https://<applicationDomain>/dex-authenticator` и отправляет их в сторону сервиса
* Secret'ы, необходимые для доступа к Dex

**Важно!** При перезапуске pod'а с oauth2-proxy при помощи refresh token'а будут получены и сохранены в память (redis) актуальные access token и id token.

#### Параметры:
* `applicationDomain` — внешний адрес вашего приложения, с которого пользовательский запрос будет перенаправлен для авторизации в Dex.
    * Формат — строка с адресом (пример: `my-app.kube.my-domain.com`, обязательно НЕ указывать HTTP схему.
* `sendAuthorizationHeader` — флаг, который отвечает за отправку конечному приложению header'а `Authorization: Bearer`.
     * Включать только если ваше приложение умеет этот header обрабатывать.
* `keepUsersLoggedInFor` — отвечает за то, как долго пользовательская сессия будет считаться активной, если пользователь бездействует (указывается с суффиксом s, m или h).
    * По-умолчанию — 7 дней (`168h`).
* `applicationIngressCertificateSecretName` — имя secret'а с TLS-сертификатом (от домена `applicationDomain`), который используется в Ingress объекте вашего приложения. Secret должен обязательно находится в том же неймспейсе, что и DexAuthenticator.
* `applicationIngressClassName` — имя Ingress класса, который будет использоваться в ingress-объекте (должно совпадать с именем ingress класса для `applicationDomain`).
* `allowedGroups` — группы, пользователям которых разрешено проходить аутентификацию. Дополнительно, опция помогает ограничить список групп до тех, которые несут для приложения полезную информацию (для примера у пользователя 50+ групп, но приложению grafana мы хотим передать только определенные 5). 
    * По умолчанию разрешены все группы.
* `whitelistSourceRanges` — список CIDR, которым разрешено проходить аутентификацию. 
    * Если параметр не указан, аутентификацию разрешено проходить без ограничения по IP-адресу.

#### Пример:
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

После появления `DexAuthenticator` в кластере, в указанном namespace'е появятся необходимые deployment, service, ingress, secret.
Чтобы подключить своё приложение к dex, достаточно будет добавить в ingress вашего приложения следующие аннотации:


##### Пример указания аннотаций для подключения `DexAuthenticator`, который мы описали выше:
```yaml
annotations:
  nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
  nginx.ingress.kubernetes.io/auth-url: https://my-cool-app-dex-authenticator.my-cool-namespace.svc.{{ домен вашего кластера, например | cluster.local }}/dex-authenticator/auth
  nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
```

##### Настройки ограничений на основе CIDR

В DexAuthenticator нет встроенной управления разрешением аутентификации на основе IP адреса пользователя. Вместо этого вы можете воспользоваться аннотациями для ingress:

* Если нужно ограничить доступ по IP и оставить прохождение аутентификации в Dex, добавьте аннотацию с указанием разрешенных CIDR через запятую:
```yaml
nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1`
```
* Если вы хотите, чтобы пользователи из указанных сетей были освобождены от прохождения аутентификации в Dex, а пользователи из остальных сетей были обязаны аутентифицироваться в Dex - добавьте следующую аннотацию:
```yaml
nginx.ingress.kubernetes.io/satisfy: "any"
```

### Настройка статических пользователей для Dex

Dex может работать без подключения провайдеров. Эта возможность реализована при помощи заведения статических пользователей (users).

#### Параметры:
* `email` — email-адрес пользователя;
    *  Поле case-insensitive;
* `groups` — массив групп, в которых состоит пользователь;
* `password` — хэшированный пароль пользователя;
    * Для получения хэшированного пароля можно воспользоваться командой `echo "$password" | htpasswd -inBC 10 "" | tr -d ':\n' | sed 's/$2y/$2a/'`

#### Пример:
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

### Настройка OAuth2 клиента в Dex для подключения приложения

Данный вариант настройки подходит приложением, которые имеют возможность использовать oauth2 аутентификацию самостоятельно без помощи oauth2-proxy.
Чтобы позволить подобным приложениям взаимодействовать с dex, вводится новый примитив - `DexClient`.

#### Параметры:
* `redirectURIs` — список адресов, на которые допустимо редиректить dex'у после успешного прохождения аутентификации.
* `trustedPeers` — id клиентов, которым позволена cross аутентификация. [Подробнее тут](https://developers.google.com/identity/protocols/CrossClientAuth).
* `allowedGroups` — список групп, участникам которых разрешено подключаться к этому клиенту;
    * По умолчанию разрешено всем группам.
#### Пример:
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

После выкладывания описанного выше ресурса в Dex'е будет зарегистрирован клиент с идентификатором (clientID) - `dex-client-myname:mynamespace`

Пароль для доступа к клиенту (clientSecret) будет сохранен в секрете:
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

### Настройка kube-apiserver

Для работы dashboard и kubeconfig-generator в вашем кластере необходимо настроить kube-apiserver. Для этого предусмотрен специальный модуль [control-plane-configurator](/modules/160-control-plane-configurator).

<details>
  <summary>Аргументы kube-apiserver, которые будут настроены</summary>

* --oidc-client-id=kubernetes
* --oidc-groups-claim=groups
* --oidc-issuer-url=https://dex.%addonsPublicDomainTemplate%/
* --oidc-username-claim=email

В случае использования самоподписанных сертификатов для dex, будет добавлен ещё один аргумент, а так же в под с apiserver будет смонтирован файл с CA:

* --oidc-ca-file=/etc/kubernetes/oidc-ca.crt

</details>
