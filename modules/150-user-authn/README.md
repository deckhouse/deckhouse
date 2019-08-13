Модуль user-authn
=======

Данный модуль устанавливает [dex](https://github.com/dexidp/dex) в кластер для возможности использования внешней аутентификации как в самом кластере (API) так и во всех веб-интерфейсах (Grafana, Dashboard и др.). 

Модуль состоит из нескольких компонентов:
- [dex](https://github.com/dexidp/dex) — федеративный OpenID Connect провайдер, который подключается к одному или нескольким внешним провайдерам (например, он поддерживает SAML, Gitlab и Github);
- kubeconfig-generator (на самом деле [dex-k8s-authenticator](https://github.com/mintel/dex-k8s-authenticator)) — веб-приложение, которое после авторизации в dex генерирует команды для настройки локального kubectl;
- dex-authenticator (на самом деле [oauth2-proxy](https://github.com/pusher/oauth2_proxy)) — приложение, которое принимает запросы от nginx ingress (auth_request) и производит их аутентификацию в dex.

**Важно!** Так как использование OpenID Connect по протоколу HTTP является слишком значительной угрозой безопасности (что подтверждается, например, тем что kubernetes api-сервер не поддерживает работу с OIDC по HTTP), данный модуль можно установить только при включеном HTTPS (`https.mode` выставить в отличное от `Disabled` значение или на уровне кластера, или в самом модуле).

**Важно!** После включения данного модуля аутентификация во всех веб-интерфейсах перестанет использовать HTTP Basic Auth и переключится на dex (который, в свою очередь, будет использовать настроенные вами внешние провайдеры). 
Для настройки kubectl необходимо перейти по адресу: `https://kubeconfig.<modules.publicDomainTemplate>/`, авторизоваться в настроенном внешнем провайдере и скопировать shell команды к себе в консоль.

**Важно!** Для работы аутентификации в dashboard и kubectl необходимо [сконфигурировать API-сервер](#настройка-kube-apiserver).

Интеграция с Okta
-----------------

Для корректной интеграции с Okta потребуется на каждый кластер заводить "приложение" в Okta. Мы планируем это автоматизировать и ПОКА ЧТО просим не использовать нашу Okta для аутентификации.

Вместо этого заведите пользователя `admin@flant.ru` следующим образом:

```

  userAuthn: |
    users:
      admin@flant.com: # <- тут достаточно пустой строки, пароль будет сгенерирован автоматически
```


Конфигурация
------------


### Параметры

* `providers` — настройки провайдеров аутентификации:
  * `id` — уникальный идентификатор провайдера аутентификации;
  * `name` — имя провайдера, которое будет отображено на странице выбора провайдера для аутентификации (если настроен всего один – эта страница не будет показана);
  * `type` — тип внешнего провайдера, в данный момент поддерживается 3 типа: `Github`, `Gitlab`, `SAML`;
  * `github` – параметры провайдера Github (можно указывать, только если `type: Github`; как [настроить Github](docs/github.md)):
    * `clientID` — ID организации на Github;
    * `clientSecret` — secret организации на Github;
    * `orgs` — массив названий организаций в Github;
    * `teams` — если в организации настроены команды, то по-умолчанию (не указав данный параметр) будет получен список всех команд у пользователя или вы можете указать список команд, которые нобходимо получать;
    * `teamNameField` — данная опция отвечает за формат команд, которые будут получены из github. Может быть одним из трех вариантов: `name` (default), `slug`, `both`.
      * Если в организации `acme` есть группа `Site Reliability Engineers`, то в случае:
        * `name` будет получена группа с именем `['acme:Site Reliability Engineers']`;
        * `slug` будет получена группа с именем `['acme:site-reliability-engineers']`;
        * `both` будут получены группы с именами `['acme:Site Reliability Engineers', 'acme:site-reliability-engineers']`.
    * `useLoginAsID` — данная опция позволяет вместо использования внутренного github id, использовать имя пользователя. 
  * `gitlab` – параметры провайдера Gitlab (можно указывать, только если `type: Gitlab`; как [настроить Gitlab](docs/gitlab.md)):
    * `clientID` — ID приложения созданного в Gitlab;
    * `clientSecret` — secret приложения созданного в Gitlab;
    * `baseURL` — адрес Gitlab'а (например: `https://fox.flant.com`);
    * `groups` — если в данном параметре передать список групп, то из gitlab'а будут получены только эти группы, а если не передать, то мы получим список всех групп пользователя в gitlab.
  * `saml` – параметры провайдера SAML (можно указывать, только если `type: SAML`):
    * `ca` — сертификат выданный SAML провайдером для проверки подлинности ответов от SAML провайдера;
    * `ssoURL` — адрес SAML аутентификации с ID и Secret'ом в URL (пример: `https://flant.okta.com/app/flant_test_dex_app_1/SECRET_TOKEN/sso/saml`);
    * `usernameAttr` — имя атрибута из которого будет получен username пользователя;
      * По-умолчанию используется атрибут `name`.
    * `emailAttr` — имя атрибута из которого будет получен email пользователя;
      * По-умолчанию используется атрибут `email`.
    * `groupsAttr` — имя атрибута из которого будет получены группы пользователя;
      * По-умолчанию используется атрибут `groups`.
    * `insecureSkipSignatureValidation` — при включении данной опции не происходит проверка подлинности ответа от провайдера с помощью `ca`;
      * По-умолчанию `false`.
    * `entityIssuer` — данная настройка отвечает за то, как мы идентифицируем себя для провайдера;
      * Если указан параметр `entityIssuer`, то он должен быть указан в провайдере? провайдер будет проверять, что dex прислал ему в запросе авторизации.
    * `ssoIssuer` — данная настройка отвечает за то, как провайдер себя идентифицирует для нас;
      * Если указан параметр `ssoIssuer`, то dex будет проверять его в ответе от провайдера.
    * `groupsDelim` — если SAML провайдер возвращает список групп юзеров одной строкой, то в данном параметре необходимо указать символ, который будет разделять список групп пользователя;
      * К примеру: `","`.
    * `nameIDPolicyFormat` — данный параметр отвечает за формат идентификатора, который будет отдавать провайдерв. Возможные значения: `EmailAddress, Unspecified, x509SubjectName, Persistent, Transistent`.
* `publishApi` — если выставить данный параметр в `true`, то в кластере будет создан ingress в default namespace, который выставляет Kubernetes API наружу, через ingress (в `kubeconfigGenerator` добавлять ничего не нужно, все будет настроено автоматически).
* `users` — данный параметр позволяет завести постоянных пользователей для логина (массив таких пользователей). В качестве ключа указывается email-адрес пользователя, а в качестве значения данного ключа - пароль. Если значеинем пароля будет пустая строка `""`, то пароль будет сгенерирован автоматически;
* `kubeconfigGenerator` — массив, в котором указываются дополнительные возможные способы доступа к API. Это может быть полезно, в случае если вы не хотите предоставить доступ к API-кластера через ingress, а хотите предоставить доступ другими способами (например, с бастион-хоста или через OpenVPN).
  * `id` — имя способа доступа к API-серверу (без пробелов, маленькими буквами);
  * `masterUri` — адрес API-сервера;
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
* `accessTokenTTL` — данный параметр отвечает за время жизни access токена (указывается с суффиксом s, m, h или d);
  * По-умолчанию — 10 минут.
* `refreshTokenTTL` — данный параметр отвечает за время жизни refresh токена (указывается с суффиксом s, m, h или d), он используется для обновления `accessToken` для доступа к приложениям.
  * По-умолчанию — 6 месяцев (4320h).
  * **Важно!** Не все типы провайдеров поддерживают механизм Refresh Token. Например, в SAML механизм не поддерживается (на уровне самого протокола SAML), так что время жизни аутентификации (SAML ответа) установленное провайдером (в нашей Okta это 2 часа) используется Dex'ом как максимальное время, в течение которого он сам (по своей воле, не обращаясь к провайдеру) может рефрешить токены. Это делает использование kubectl практически невозможным.
* `highAvailability` — ручное управление [режимом отказоустойчивости](/FEATURES.md#отказоустойчивость).
* `nodeSelector` — как в Kubernetes в `spec.nodeSelector` у pod'ов.
    * Если ничего не указано — будет использоваться значение `{"node-role.flant.com/vsphere-csi-driver":""}` или `{"node-role.flant.com/system":""}` (если в кластере есть такие узлы) или ничего не будет указано.
    * Можно указать `false`, чтобы не добавлять никакой nodeSelector.
* `tolerations` — как в Kubernetes в `spec.tolerations` у pod'ов.
    * Если ничего не указано — будет настроено значение `[{"key":"dedicated.flant.com","operator":"Equal","value":"vsphere-csi-driver"},{"key":"dedicated.flant.com","operator":"Equal","value":"system"}]`.
    * Можно указать `false`, чтобы не добавлять никакие toleration'ы.
* `ingressClass` — класс ingress контроллера, который используется для dex и kubeconfig-generator.
  * Опциональный параметр, по-умолчанию используется глобальное значение `modules.ingressClass`.
* `https` — выбираем, какой типа сертификата использовать для dex и kubeconfig-generator.
  * `mode` - режим работы HTTPS:
    * `Disabled` — при данном значении модуль автоматически отключается.
    * `CertManager` — dex и kubeconfig-generator будут работать по https и заказывать сертификат с помощью clusterissuer заданном в параметре `certManager.clusterIssuerName`;
    * `CustomCertificate` — dex и kubeconfig-generator будут работать по https используя сертификат из namespace `antiopa`;
    * `UriOnly` — dex и kubeconfig-generator. будет работать по http (подразумевая, что перед ними стоит внешний https балансер, который терминирует https) и все ссылки в `user-authn` будут генерироваться с https схемой.
  * `certManager`
    * `clusterIssuerName` — указываем, какой ClusterIssuer использовать для dex и kubeconfig-generator (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
  * `customCertificate`
    * `secretName` - указываем имя secret'а в namespace `antiopa`, который будет использоваться для dex и kubeconfig-generator (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets)).

### Примеt конфигурации

```yaml
  userAuthn: |
    providers:
    - id: github
      name: Github Asidorov
      type: Github
      github:
        clientID: 7d70961e35f46d220784b8
        clientSecret: db22a757102403199cza4d568404f67548b6f20a3
        orgs:
        - name: asidorovj-test
          teams:
          - bro
    - id: gitlab-fox
      name: Flant Gitlab
      type: Gitlab
      gitlab:
        baseURL: https://fox.flant.com
        clientID: 480dc4611c987b4997s605821ea3e79957be2a15cdz1664149643014a7c619c6379
        clientSecret: 22d134403a3a446fsee57a4a4d6262ba33fb1511375665fa76028d3039c307c9aca
    - id: okta
      name: Okta
      type: SAML
      saml:
        ca: |
          -----BEGIN CERTIFICATE-----
          ...
          -----END CERTIFICATE-----
        ssoURL: https://flant.okta.com/app/flant_dextest_1/ex1kmljf1v09zEq6gxsHU0x7/sso/saml
    kubeconfigGenerator:
      - id: direct
        masterUri: https://159.89.5.247:6443
        description: "Direct access to kubernetes API"
    publishApi: true
```

### Настройка kube-apiserver

#### Настройка kube-apiserver у bare-metal кластеров

Для этого необходимо отредактировать манифест `/etc/kubernetes/manifests/kube-apiserver.yaml` и добавить аргументы на всех master нодах:
```yaml
    - --oidc-client-id=kubernetes
    - --oidc-groups-claim=groups
    - --oidc-issuer-url=https://dex.<modules.publicDomainTemplate>/
    - --oidc-username-claim=email
```

И перезапустить kubelet:
```shell
systemctl restart kubelet
```

#### Настройка kube-apiserver у kops кластеров

Для этого необходимо отредактировать специфицкацию кластера:
```shell
kops edit cluster --name=kubernetes-cluster
```

И добавить параметр:
```yaml
  kubeAPIServer:
    oidcClientID: kubernetes
    oidcGroupsClaim: groups
    oidcIssuerURL: https://dex.<modules.publicDomainTemplate>/
    oidcUsernameClaim: email
```

После чего обновить кластер:
```shell
kops update cluster --name=kubernetes-cluster
kops update cluster --name=kubernetes-cluster --yes
kops rolling-update cluster --name=kubernetes-cluster
kops rolling-update cluster --name=kubernetes-cluster --yes
```

#### Настройка kube-apiserver у aks-engine кластеров

Для этого необходимо отредактировать описание кластера (`apimodel.json` файл, который был создан при генерации конфигурации кластера) и добавить в параметр `apiServerConfig` такие параметры:
```yaml
        "apiServerConfig": {
          ...
          "--oidc-client-id": "kubernetes",
          "--oidc-groups-claim": "groups",
          "--oidc-issuer-url": "https://dex.<modules.publicDomainTemplate>/",
          "--oidc-username-claim": "email"
        }
```

И обновить кластер:
```shell
aks-engine upgrade
  --subscription-id xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
  --api-model _output/kube-test-1-12-flant/apimodel.json \
  --location westeurope \
  --resource-group k-dev \
  --upgrade-version 1.15.0 \
  --client-id xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx \
  --client-secret xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

