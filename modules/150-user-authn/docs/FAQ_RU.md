---
title: "Модуль user-authn: FAQ"
---

## Как защитить мое приложение?

Чтобы включить аутентификацию через Dex для приложения, выполните следующие шаги:

1. Создайте [DexAuthenticator](cr.html#dexauthenticator).

   При создании [DexAuthenticator](cr.html#dexauthenticator) в кластере создаётся экземпляр [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy), подключённый к Dex. В указанном неймспейсе создаются Deployment, Service и Secret, а также Ingress, если задан `applicationIngressClassName`, и HTTPRoute, если задан `gatewayAPI`.

   Пример ресурса [DexAuthenticator](/modules/user-authn/cr.html#dexauthenticator):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: DexAuthenticator
   metadata:
     # Префикс имени подов Dex authenticator.
     # Например, если префикс имени `app-name`, то поды Dex authenticator будут вида `app-name-dex-authenticator-7f698684c8-c5cjg`.
     name: app-name
     # Неймспейс, в котором будет развернут Dex authenticator.
     namespace: app-ns
   spec:
     # Домен вашего приложения. Запросы на него будут перенаправляться для прохождения аутентификации в Dex.
     applicationDomain: "app-name.kube.my-domain.com"
     # Отправлять ли заголовок `Authorization: Bearer` приложению. Полезно в связке с auth_request в NGINX.
     # При значении sendAuthorizationHeader: true добавьте заголовок Authorization в аннотацию nginx.ingress.kubernetes.io/auth-response-headers Ingress приложения или в аннотацию alb.network.deckhouse.io/auth-response-headers ресурса HTTPRoute.
     sendAuthorizationHeader: false
     # Имя секрета с SSL-сертификатом.
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

   Чтобы добавить лейблы и аннотации подам `dex-authenticator`, например отключить внедрение sidecar Istio, используйте [`spec.podMetadata`](cr.html#dexauthenticator-v1-spec-podmetadata):

   ```yaml
   spec:
     podMetadata:
       annotations:
         sidecar.istio.io/inject: "false"
   ```

1. Подключите приложение к Dex.

   Для этого добавьте аннотации в объект, через который публикуется приложение. Набор аннотаций зависит от способа публикации приложения. Выберите подходящий вариант:

{% tabs connect-app %}
{% tab "Через Ingress-ресурс" %}

{% raw %}

Добавьте в Ingress-ресурс приложения следующие аннотации:

- `nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in`
- `nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email`
- `nginx.ingress.kubernetes.io/auth-url: https://<SERVICE_NAME>.<NS>.svc.{{ C_DOMAIN }}/dex-authenticator/auth`, где:
  - `SERVICE_NAME` — имя сервиса (Service) аутентификатора. Как правило, оно соответствует формату `<NAME>-dex-authenticator` (`<NAME>` — это `metadata.name` ресурса [DexAuthenticator](/modules/user-authn/cr.html#dexauthenticator));
  - `NS` — значение параметра `metadata.namespace` ресурса [DexAuthenticator](/modules/user-authn/cr.html#dexauthenticator);
  - `C_DOMAIN` — домен кластера (параметр [clusterDomain](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) ресурса [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration)).

{% endraw %}

{% alert level="info" %}

Если имя DexAuthenticator (`<NAME>`) слишком длинное, имя сервиса (Service) может быть сокращено. Чтобы найти корректное имя сервиса, воспользуйтесь следующей командой (укажите имя неймспейса и аутентификатора):

```shell
d8 k get service -n <NS> -l "deckhouse.io/dex-authenticator-for=<NAME>" -o jsonpath='{.items[0].metadata.name}'
```

{% endalert %}

Пример аннотаций Ingress-ресурса приложения для подключения к Dex:

```yaml
annotations:
  nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
  nginx.ingress.kubernetes.io/auth-url: https://app-name-dex-authenticator.app-ns.svc.cluster.local/dex-authenticator/auth
  nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
```

Для приложения, в котором указан только `applicationIngressClassName`, HTTPRoute не создаётся. Если публикация через Ingress отключена параметром [`global.modules.ingress.enabled`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-ingress-enabled), не создаётся и Ingress. Само приложение остаётся доступным через тот маршрут, которым вы его публикуете, а вот его эндпоинты `/dex-authenticator` — нет: редирект на вход попадает в само приложение, и войти невозможно. На остальные приложения это не влияет.

{% endtab %}
{% tab "Через ALBInstance или ClusterALBInstance" %}

Если приложение публикуется через ресурс ALBInstance или ClusterALBInstance (подробнее — в документации модуля [`alb`](/modules/alb/)), добавьте в HTTPRoute-ресурс приложения следующие аннотации:

- `alb.network.deckhouse.io/auth-signin: https://<домен-приложения>/dex-authenticator/sign_in` — в отличие от nginx, контроллер `alb` не поддерживает переменную `$host`, поэтому домен приложения нужно указать явно;
- `alb.network.deckhouse.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email`;
- `alb.network.deckhouse.io/auth-url: https://<SERVICE_NAME>.<NS>.svc.<C_DOMAIN>/dex-authenticator/auth`, где `SERVICE_NAME`, `NS` и `C_DOMAIN` определяются так же, как для Ingress-ресурса.

Пример аннотаций ресурса HTTPRoute для подключения приложения к Dex:

```yaml
annotations:
  alb.network.deckhouse.io/auth-signin: https://app-name.kube.my-domain.com/dex-authenticator/sign_in
  alb.network.deckhouse.io/auth-url: https://app-name-dex-authenticator.app-ns.svc.cluster.local/dex-authenticator/auth
  alb.network.deckhouse.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
```

Также укажите в ресурсе DexAuthenticator тот же ListenerSet, через который опубликован домен приложения:

```yaml
spec:
  gatewayAPI:
    applicationHTTPRouteListenerSetName: my-listenerset
```

Создаваемый для DexAuthenticator HTTPRoute подключается к этому же ListenerSet, поэтому ListenerSet должен принимать маршруты из неймспейса, в котором находится DexAuthenticator.

Для приложения, в котором указан только `gatewayAPI`, Ingress не создаётся. Если Gateway API отключён параметром [`global.modules.gatewayAPI.enabled`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-gatewayapi-enabled) либо в кластере нет API HTTPRoute, не создаётся и HTTPRoute — с тем же результатом: приложение остаётся доступным, его эндпоинты `/dex-authenticator` — нет, войти невозможно. На остальные приложения это не влияет.

{% endtab %}
{% endtabs %}

{% alert level="warning" %}
При включении `sendAuthorizationHeader: true` в Ingress (или в HTTPRoute, если используется модуль [`alb`](/modules/alb/)) укажите все необходимые заголовки в соответствующей аннотации, поскольку заголовок `Authorization` по умолчанию не передаётся.

Подробнее о том, что передаётся в заголовке `Authorization` и как указать его в аннотации, читайте в разделе [«Как передать приложению логин и группы пользователя»](#как-передать-приложению-логин-и-группы-пользователя).
{% endalert %}

{% alert level="warning" %}
Ingress приложения должен иметь настроенный TLS. DexAuthenticator не поддерживает Ingress-ресурсы, работающие только по HTTP.
{% endalert %}

### Настройка ограничений на основе CIDR

Чтобы разрешить аутентификацию только из определённых сетей, перечислите их в [`whitelistSourceRanges`](cr.html#dexauthenticator-v1-spec-whitelistsourceranges). Ограничение действует на эндпоинты `/dex-authenticator`. Чтобы ограничить доступ к самому приложению или пропускать часть сетей без аутентификации, используйте аннотации Ingress приложения:

* Если нужно ограничить доступ по IP и оставить прохождение аутентификации в Dex, добавьте аннотацию с указанием разрешенных CIDR через запятую:

  ```yaml
  nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1
  ```

* Чтобы разрешить доступ без аутентификации в Dex для пользователей из указанных сетей, а для остальных оставить обязательную аутентификацию, добавьте аннотацию:

  ```yaml
  nginx.ingress.kubernetes.io/satisfy: "any"
  ```

## Как передать приложению логин и группы пользователя?

По умолчанию DexAuthenticator передаёт приложению только два заголовка: `X-Auth-Request-User` (значение основано на непрозрачном claim'е `sub`) и `X-Auth-Request-Email`. Заголовок с группами пользователя не передаётся. Он неограниченно растёт при большом количестве групп, поэтому в DP отключён, и включить его нельзя.

Чтобы приложение получило полную информацию о пользователе, включая группы, включите параметр [`sendAuthorizationHeader`](cr.html#dexauthenticator-v1-spec-sendauthorizationheader):

```yaml
apiVersion: deckhouse.io/v1
kind: DexAuthenticator
metadata:
  name: app-name
  namespace: app-ns
spec:
  applicationDomain: "app-name.kube.my-domain.com"
  applicationIngressClassName: "nginx"
  applicationIngressCertificateSecretName: "ingress-tls"
  sendAuthorizationHeader: true
```

В этом случае приложению передаётся заголовок `Authorization: Bearer <id_token>`, где `<id_token>` — подписанный Dex JWT (подробнее о содержимом токена и его обработке в приложении — в разделе [«Содержимое токена JWT и особенности его обработки»](#содержимое-токена-jwt-и-особенности-его-обработки)).

Независимо от того, каким способом публикуется приложение, этот заголовок не передается в приложение автоматически. Его нужно явно указать в списке передаваемых с помощью аннотации ресурса, через который публикуется приложение. Выберите подходящий вариант, в зависимости от того, каким способом публикуется приложение:

{% tabs publications %}
{% tab "Через Ingress-ресурс" %}

Если приложение публикуется через Ingress-ресурс (подробнее — в документации модуля [`ingress-nginx`](/modules/ingress-nginx/)), при включении [`sendAuthorizationHeader: true`](cr.html#dexauthenticator-v1-spec-sendauthorizationheader) необходимо:

- указать заголовки, которые нужно передавать в приложение, в аннотации `nginx.ingress.kubernetes.io/auth-response-headers`;
- увеличить размер буфера с помощью аннотации `nginx.ingress.kubernetes.io/proxy-buffer-size`, поскольку JWT с большим числом групп не помещается в буфер по умолчанию.

Пример указания заголовков и размера буфера с помощью соответствующих аннотаций:

{% raw %}

```yaml
annotations:
  nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
  nginx.ingress.kubernetes.io/auth-url: https://app-name-dex-authenticator.app-ns.svc.cluster.local/dex-authenticator/auth
  nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email,Authorization
  nginx.ingress.kubernetes.io/proxy-buffer-size: 32k
```

{% endraw %}

Если заголовок `Authorization` не указан в `auth-response-headers`, приложение его не получит. Если не увеличить `proxy-buffer-size`, запросы будут завершаться ошибкой 500, а в логах контроллера Ingress появится сообщение `upstream sent too big header while reading response header from upstream`.

{% endtab %}
{% tab "Через ALBInstance или ClusterALBInstance" %}

Если приложение публикуется через ресурс ALBInstance или ClusterALBInstance (подробнее — в документации модуля [`alb`](/modules/alb/)), при включении [`sendAuthorizationHeader: true`](cr.html#dexauthenticator-v1-spec-sendauthorizationheader) укажите заголовки, которые нужно передавать в приложение, в аннотации `alb.network.deckhouse.io/auth-response-headers` ресурса HTTPRoute:

```yaml
annotations:
  alb.network.deckhouse.io/auth-signin: https://app-name.kube.my-domain.com/dex-authenticator/sign_in
  alb.network.deckhouse.io/auth-url: https://app-name-dex-authenticator.app-ns.svc.cluster.local/dex-authenticator/auth
  alb.network.deckhouse.io/auth-response-headers: Authorization
```

В аннотации `alb.network.deckhouse.io/auth-response-headers` достаточно указать только `Authorization`, так как базовый набор заголовков передаётся по умолчанию.

Также в ресурсе DexAuthenticator укажите тот же ListenerSet, через который опубликован домен приложения:

```yaml
spec:
  gatewayAPI:
    applicationHTTPRouteListenerSetName: my-listenerset
```

{% endtab %}
{% endtabs %}

### Содержимое токена JWT и особенности его обработки

Пример полезной нагрузки JWT для статического пользователя (ресурсы [User](cr.html#user) и [Group](cr.html#group)):

```json
{
  "iss": "https://dex.kube.my-domain.com/",
  "sub": "Cg1qb2huLmRvZUBleGFtcGxlEgVsb2NhbA",
  "aud": "app-name-app-ns-dex-authenticator",
  "exp": 1757000600,
  "iat": 1757000000,
  "email": "john.doe@example.com",
  "email_verified": true,
  "name": "john-doe",
  "preferred_username": "",
  "groups": ["everyone", "developers"]
}
```

При использовании данных из токена в приложении обращайте внимание на следующее:

- Для идентификации пользователя используйте поле `email`. Поле `sub` непрозрачно (не является предсказуемым и не может быть использовано как осмысленный идентификатор пользователя вне контекста конкретной системы), а `preferred_username` для статических пользователей пуст (внешние провайдеры аутентификации могут его заполнять).
- Поле `name` содержит имя объекта (из поля `metadata.name` объекта [User](cr.html#user)), а не отображаемое имя пользователя.
- Поле `aud` содержит идентификатор клиента аутентификатора (`<name>-<namespace>-dex-authenticator`), а не идентификатор OIDC-клиента вашего приложения. Приложение, проверяющее `aud` по собственному `client_id`, отклонит такой токен.
- Подпись проверяйте по JWKS-эндпоинту `https://<DEX_DOMAIN>/keys`, где `<DEX_DOMAIN>` — значение [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate), в котором `%s` заменено на `dex`.
- Время жизни токена определяется параметром [`idTokenTTL`](configuration.html#parameters-idtokenttl) (по умолчанию 10 минут). DexAuthenticator обновляет токен самостоятельно, поэтому приложение всегда получает актуальный токен.

Если приложение поддерживает OIDC самостоятельно, вместо DexAuthenticator используйте ресурс [DexClient](cr.html#dexclient): приложение само запросит необходимый объём полномочий и получит `refresh_token` в дополнение к `id_token`.

## Как работает аутентификация с помощью DexAuthenticator

![Как работает аутентификация с помощью DexAuthenticator](images/dex_login.svg)

DexAuthenticator устанавливает cookie с атрибутом `Secure`, поэтому они передаются только по HTTPS.

1. Dex в большинстве случаев перенаправляет пользователя на страницу входа провайдера и ожидает, что пользователь будет перенаправлен на его `/callback` URL. Однако такие провайдеры, как LDAP или Atlassian Crowd, не поддерживают этот вариант. Вместо этого пользователь должен ввести логин и пароль в форму входа в Dex, и Dex сам проверит учётные данные, выполнив запрос к API провайдера.

1. DexAuthenticator начинает вход с Proof Key for Code Exchange (PKCE) по методу `S256` и получает токены и ключи подписи у Service `dex` в неймспейсе `d8-user-authn`. Сертификат Dex он проверяет по центру сертификации (CA) внутреннего сертификата Dex. Этот CA `user-authn` добавляет в Secret каждого DexAuthenticator.

1. DexAuthenticator устанавливает cookie с полным токеном обновления (вместо выдачи тикета, как для ID-токена), потому что Redis не сохраняет данные на диск.
   Если по тикету в Redis не найден ID-токен, пользователь сможет запросить новый ID-токен, предоставив токен обновления из cookie.

1. DexAuthenticator выставляет HTTP-заголовок `Authorization`, равный значению ID-токена из Redis. Это необязательно для сервисов вроде [`upmeter`](/modules/upmeter/), так как права доступа к `upmeter` менее детальные.
   Для [Kubernetes Dashboard](/modules/dashboard/) это критичная функциональность: Dashboard передаёт ID-токен дальше для доступа к API Kubernetes.

### Обработка истёкшей сессии в одностраничном приложении

Без сессии запрос к приложению перенаправляется на страницу входа из аннотации `auth-signin` (`/dex-authenticator/sign_in`). DexAuthenticator отправляет пользователя в Dex, только когда браузер переходит на эту страницу. Переходом считается запрос с заголовком `Sec-Fetch-Mode: navigate`. Если браузер не передаёт `Sec-Fetch-Mode`, переходом считается запрос без заголовков `Accept: application/json` и `X-Requested-With: XMLHttpRequest`. Запрос скрипта (`fetch`, XHR) получает код ответа `401` с телом `{}` вместо перенаправления в Dex, которое браузер заблокировал бы как запрос к другому домену.

Чтобы отправить пользователя на вход, перезагрузите страницу, когда запрос скрипта получает `401`:

```javascript
const response = await fetch("/api/items");
if (response.status === 401) {
  window.location.reload();
}
```

### Завершение сессии DexAuthenticator

DexAuthenticator завершает сессии пользователя в течение нескольких секунд после операции `Lock`, `ResetPassword`, `Reset2FA` или `Logout` и после удаления пользователя. Запрос с завершённой сессией получает тот же ответ, что и запрос без сессии.

DexAuthenticator получает отзывы от сервиса `revocation-feed` в неймспейсе `d8-user-authn`. Если NetworkPolicy ограничивает исходящий трафик в неймспейсе DexAuthenticator, разрешите трафик к подам с лейблом `app: user-authn-controller` в неймспейсе `d8-user-authn` на порт `8443`. Без отзывов DexAuthenticator проверяет в Dex каждую сессию старше 30 секунд, а через 15 минут срабатывает алерт `D8UserAuthnRevocationFeedNotConsumed`.

Когда пользователь выходит из приложения, DexAuthenticator отзывает refresh-токен сессии в Dex. Копия cookie сессии тоже перестаёт работать.

## Как сгенерировать kubeconfig для доступа к Kubernetes API?

Для генерации kubeconfig воспользуйтесь разделом [«Как сгенерировать kubeconfig для доступа к Kubernetes API?»](/modules/control-plane-manager/faq.html#как-сгенерировать-kubeconfig-для-доступа-к-kubernetes-api) документации модуля `control-plane-manager`.

### Настройка kube-apiserver

С помощью модуля [`control-plane-manager`](/modules/control-plane-manager/) DP передаёт `kube-apiserver` конфигурацию AuthenticationConfiguration с JWT-аутентификатором для Dex: издатель `https://<DEX_DOMAIN>/` (где `<DEX_DOMAIN>` — значение `publicDomainTemplate`, в котором `%s` заменено на `dex`), аудитория `kubernetes`, имя пользователя из claim `email`, группы из claim `groups`. Если CA задан через [`controlPlaneConfigurator.dexCAMode`](configuration.html#parameters-controlplaneconfigurator-dexcamode), он встраивается в конфигурацию. Имена пользователей и групп с префиксом `system:` отклоняются.

### Как работает подключение к Kubernetes API с помощью сгенерированного kubeconfig

![Схема взаимодействия при подключении к Kubernetes API с помощью сгенерированного kubeconfig](images/kubeconfig_dex.svg)

1. `kube-apiserver` получает конфигурацию Dex (издатель и JWKS-эндпоинт). В Kubernetes 1.30 и новее он получает её через сервис Dex внутри кластера.

1. Файл kubeconfig, сгенерированный в веб-интерфейсе Deckhouse, содержит ID-токен и refresh-токен.

1. После получения запроса с ID-токеном `kube-apiserver` проверяет, что токен подписан провайдером, настроенным на первом шаге, с помощью ключей, полученных с JWKS-эндпоинта. Затем сравнивает значения claim `iss` и `aud` из токена со значениями из конфигурации.

## Как сменить секрет OAuth2-клиента kubernetes?

Секрет привилегированного OAuth2-клиента `kubernetes` хранится в Secret `kubernetes-dex-client-app-secret` в неймспейсе `d8-user-authn`. То же значение используют OAuth2-клиенты `kubeconfig-generator`, `kubeconfig-generator-<index>`, `kubeconfig-publish-api` и `kubeconfig-<slug>`, а также basic-auth-proxy, если для LDAP-провайдера включена базовая аутентификация.

Удаление Secret не приводит к смене секрета: пока значение сохраняется во внутренних параметрах модуля, хук восстановит Secret с прежним значением.

Чтобы сменить секрет, выполните следующие действия:

1. Если неймспейс `d8-user-authn` управляется с помощью GitOps-инструмента, исключите Secret `kubernetes-dex-client-app-secret` из синхронизации. Иначе GitOps-инструмент восстановит прежнее значение.

1. Очистите поле `secret`:

   ```shell
   d8 k -n d8-user-authn patch secret kubernetes-dex-client-app-secret --type merge -p '{"data":{"secret":""}}'
   ```

1. Перезапустите DP, чтобы хук зарегистрировал пустое поле и сгенерировал новый секрет:

   ```shell
   d8 k -n d8-system rollout restart deployment/deckhouse
   ```

1. Убедитесь, что значение секрета изменилось:

   ```shell
   d8 k -n d8-user-authn get secret kubernetes-dex-client-app-secret -o jsonpath='{.data.secret}'
   ```

   Если значение не изменилось, повторите шаги 2 и 3. Модуль мог восстановить прежнее значение до перезапуска DP.

После смены секрета конфигурация использующих его компонентов в кластере обновится автоматически, а их поды будут перезапущены.

Файлы kubeconfig, сгенерированные в веб-интерфейсе Deckhouse до смены секрета, содержат прежний клиентский секрет и больше не обновляют токены. Сгенерируйте их заново. Уже выданные ID-токены продолжат действовать до истечения их срока действия, определяемого [параметром `settings.idTokenTTL`](configuration.html#parameters-idtokenttl) (по умолчанию — 10 минут).

## Как включить SSO по Kerberos (SPNEGO) для LDAP?

Если на стороне клиента настроено доменное SSO (браузер доверяет домену Dex), Dex может принимать Kerberos‑билеты по заголовку `Authorization: Negotiate` и выполнять аутентификацию без отображения формы ввода логина/пароля.

Включение SSO по Kerberos (SPNEGO) для LDAP:

1. Создайте в AD/KDC SPN `HTTP/<DEX_FQDN>` для сервисной учётной записи и сгенерируйте `keytab`.
1. В кластере создайте секрет в неймспейсе `d8-user-authn` с ключом `krb5.keytab`.
1. В ресурсе DexProvider (тип LDAP) включите блок [`spec.ldap.kerberos`](/modules/user-authn/cr.html#dexprovider-v1-spec-ldap-kerberos) и настройте в нём параметры:
   - `enabled: true`;
   - `keytabSecretName: <имя секрета>`;
   - опционально: `expectedRealm`, `usernameFromPrincipal`, `fallbackToPassword`.

Dex автоматически смонтирует `keytab` и начнёт принимать SPNEGO. `krb5.conf` на сервере не обязателен — билеты проверяются по keytab.

## Как настроить базовую аутентификацию для доступа к Kubernetes API через LDAP?

Используйте настройки модуля `control-plane-manager`:

1. Включите параметр [`apiserver.publishAPI`](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi) в конфигурации модуля `control-plane-manager`.
1. Создайте [DexProvider](cr.html#dexprovider) типа `LDAP` и установите параметр [`enableBasicAuth: true`](cr.html#dexprovider-v1-spec-ldap-enablebasicauth).
1. Выдайте группам из LDAP права с помощью RoleBinding или ClusterRoleBinding [гранулярной ролевой модели](/modules/user-authz/#гранулярная-ролевая-модель).
1. Передайте пользователям kubeconfig с настроенными параметрами базовой аутентификации (логин и пароль LDAP).

{% alert level="warning" %}
В кластере может быть только один провайдер аутентификации со включённым параметром [`enableBasicAuth`](cr.html#dexprovider-v1-spec-ldap-enablebasicauth).
{% endalert %}

Подробный пример описан в разделе [Примеры конфигурации](/modules/user-authn/usage.html#настройка-базовой-аутентификации).

## Как Dex защищен от подбора логина и пароля?

Dex ограничивает частоту неудачных попыток входа с одного IP-адреса на форме входа, эндпоинте токенов, странице смены пароля и шагах 2FA. Ограничение настраивается параметром [`rateLimit`](configuration.html#parameters-ratelimit) и по умолчанию выключено.

Кроме того, Dex блокирует учётную запись после неудачных попыток входа. Блокировка настраивается параметром [`passwordPolicy.lockout`](configuration.html#parameters-passwordpolicy-lockout). Она действует для локальных пользователей, а для пользователей LDAP и Crowd — если их типы указаны в [`applyToConnectors`](configuration.html#parameters-passwordpolicy-lockout-applytoconnectors).

## Как узнать, почему пользователю отказано во входе?

Dex записывает каждый вход в журнал в формате JSON. Отказ во входе записывается с уровнем `WARN` и полем `reason`:

* `invalid_credentials` — неверный логин или пароль либо страница смены пароля открыта вне входа, который на неё привёл;
* `invalid_totp` — неверный или уже использованный код 2FA;
* `not_in_allowed_groups` — пользователь не входит в группы, которых требуют настройки провайдера;
* `not_allowed` — пользователя нет в списке разрешённых (поле `rule` указывает список);
* `account_locked` — пользователь заблокирован;
* `rate_limited` — с IP-адреса клиента пришло слишком много неудачных попыток (вместо провайдера, клиента и пользователя запись содержит запрошенный путь в поле `path`);
* `upstream_refused` — провайдер отказал в продлении сессии, потому что пользователя в нём больше нет или он потерял доступ.
* `revoked` — refresh-токен, код авторизации или сессия браузера выданы до операции `Lock`, `ResetPassword`, `Reset2FA` или `Logout` либо до удаления пользователя.

В записи также есть `connector_id`, `client_id` и известные атрибуты пользователя, например `username` и `email`. При отказе по группам в записи также есть `user_groups` и `allowed_groups`. Сбой Dex или провайдера записывается с уровнем `ERROR`.

Чтобы найти отказы в журнале Dex, выполните:

```shell
d8 k -n d8-user-authn logs -l app=dex -c dex --tail=-1 | grep '"reason"'
```

## UserOperation в статусе Failed — что делать?

Проверьте поле `status.message` объекта UserOperation, чтобы узнать описание ошибки:

```shell
d8 k get useroperation <имя> -o jsonpath='{.status.message}'
```

Устраните причину (например, неверный хеш пароля или несуществующий пользователь), затем создайте новый UserOperation. UserOperation выполняется один раз, и изменения его спецификации после создания игнорируются.

## Как разблокировать пользователя?

Используйте команду:

```shell
d8 iam user unlock <USERNAME>
```

Команда разблокирует локального пользователя. Чтобы разблокировать пользователя внешнего провайдера, создайте UserOperation с `type: Unlock` и [`spec.target`](cr.html#useroperation-v1-spec-target), как описано [в разделе «Операции над внешними пользователями»](usage.html#операции-над-внешними-пользователями). UserOperation со `spec.user` также разблокирует локального пользователя. Операция `Lock` уже завершила сессии пользователя, и `Unlock` их не восстанавливает, поэтому пользователь входит заново. Секрет 2FA пользователя удаляет только `Reset2FA`.

## Пользователь заблокирован автоматически — почему?

Количество неудачных попыток входа превысило [`passwordPolicy.lockout.maxAttempts`](configuration.html#parameters-passwordpolicy-lockout-maxattempts). Пользователь блокируется на время, указанное в [`passwordPolicy.lockout.lockDuration`](configuration.html#parameters-passwordpolicy-lockout-lockduration), после чего разблокируется автоматически. Локальные пользователи блокируются всегда. Пользователи LDAP и Crowd блокируются, только если их тип указан в [`passwordPolicy.lockout.applyToConnectors`](configuration.html#parameters-passwordpolicy-lockout-applytoconnectors). Dex хранит блокировку сам и не создаёт UserOperation. Администратор может также разблокировать пользователя вручную командой `d8 iam user unlock <USERNAME>` (локальные пользователи) или создав UserOperation с `type: Unlock` (со `spec.user` для локальных пользователей, со `spec.target` для пользователей LDAP и Crowd).

Заблокирован ли пользователь и до какого времени, показывают столбцы `Locked` и `LockedUntil` в выводе `d8 k get useraccounts`. Модуль ведёт объект [UserAccount](cr.html#useraccount) для каждого локального пользователя и для каждого пользователя, который входил через внешний провайдер.

## Можно ли удалить объект UserIdentity?

Нет. Dex хранит секрет 2FA, блокировку и время последнего входа учётной записи в объекте UserIdentity в неймспейсе `d8-user-authn`. В аннотации `user-authn.deckhouse.io/generation` объект также хранит поколение учётной записи, по которому модуль отзывает её токены. Модуль отклоняет запрос на удаление UserIdentity. Чтобы удалить локального пользователя, удалите User. Чтобы сбросить 2FA или блокировку, используйте UserOperation с типом `Reset2FA` или `Unlock`.

## Можно ли отменить операцию UserOperation?

Нет. UserOperation выполняется один раз. Чтобы отменить эффект операции, создайте обратную операцию, например UserOperation с `type: Unlock` после операции `Lock`.
