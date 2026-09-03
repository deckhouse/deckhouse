---
title: "Модуль user-authn: FAQ"
---

## Как защитить мое приложение?

Чтобы включить аутентификацию через Dex для приложения, выполните следующие шаги:

1. Создайте ресурс [DexAuthenticator](/modules/user-authn/cr.html#dexauthenticator).

   При создании [DexAuthenticator](/modules/user-authn/cr.html#dexauthenticator) в кластере создаётся экземпляр [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy), подключённый к Dex. В указанном неймспейсе будут созданы объекты Deployment, Service, Ingress, Secret.

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

1. Подключите приложение к Dex.

   Для этого добавьте в ресурс, через который публикуется приложение аннотации. Набор аннотаций зависит от того, каким способом публикуется приложение. Выберите подходящий вариант:

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

   > **Важно:** Если имя DexAuthenticator (`<NAME>`) слишком длинное, имя сервиса (Service) может быть сокращено. Чтобы найти корректное имя сервиса, воспользуйтесь следующей командой (укажите имя неймспейса и аутентификатора):
   >
   > ```shell
   > d8 k get service -n <NS> -l "deckhouse.io/dex-authenticator-for=<NAME>" -o jsonpath='{.items[0].metadata.name}'
   > ```
   >

   Пример аннотаций Ingress-ресурса приложения для подключения к Dex:

   ```yaml
   annotations:
     nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
     nginx.ingress.kubernetes.io/auth-url: https://app-name-dex-authenticator.app-ns.svc.cluster.local/dex-authenticator/auth
     nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
   ```

{% endraw %}

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

{% endtab %}
{% endtabs %}

{% alert level="warning" %}
При включении `sendAuthorizationHeader: true` в Ingress (или в HTTPRoute, если используется модуль [`alb`](/modules/alb/)) укажите все необходимые заголовки в соответствующей аннотации, поскольку заголовок `Authorization` по умолчанию не передаётся:

Подробнее о том, что передаётся в заголовке `Authorization` и как указать его в аннотации, читайте в разделе [«Как передать приложению логин и группы пользователя»](#как-передать-приложению-логин-и-группы-пользователя).
{% endalert %}

{% alert level="warning" %}
Ingress приложения должен иметь настроенный TLS. DexAuthenticator не поддерживает Ingress-ресурсы, работающие только по HTTP.
{% endalert %}

### Настройка ограничений на основе CIDR

В DexAuthenticator нет встроенной системы управления разрешением аутентификации на основе IP-адреса пользователя. Вместо этого вы можете воспользоваться аннотациями для Ingress-ресурсов:

* Если нужно ограничить доступ по IP и оставить прохождение аутентификации в Dex, добавьте аннотацию с указанием разрешенных CIDR через запятую:

  ```yaml
  nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1
  ```

* Чтобы разрешить доступ без аутентификации в Dex для пользователей из указанных сетей, а для остальных оставить обязательную аутентификацию, добавьте аннотацию:

  ```yaml
  nginx.ingress.kubernetes.io/satisfy: "any"
  ```

## Как передать приложению логин и группы пользователя?

По умолчанию DexAuthenticator передаёт приложению только два заголовка: `X-Auth-Request-User` (значение основано на непрозрачном claim'е `sub`) и `X-Auth-Request-Email`. Заголовок с группами пользователя не передаётся. Он неограниченно растёт при большом количестве групп, поэтому в DKP отключён, и включить его нельзя.

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

Независимо от того, каким способом публикуется приложение, этот заголовок не передается в приложение автоматически. Его нужно явно указать в списке передаваемых с помощью аннотация ресурса, через который публикуется приложение. Выберите подходящий вариант, в зависимости от того, каким способом публикуется приложение:

{% tabs puplications %}
{% tab "Через Ingress-ресурс" %}

Если приложение публикуется через Ingress-ресурс (подробнее — в документации модуля [`ingress-nginx`](/modules/ingress-nginx/)), при включении [`sendAuthorizationHeader: true`]((cr.html#dexauthenticator-v1-spec-sendauthorizationheader)) необходимо:

- указать заголовки, которые нужно передавать в приложение, в аннотации `nginx.ingress.kubernetes.io/auth-response-headers`;
- а также увеличить размер буфера с помощью аннотации `nginx.ingress.kubernetes.io/proxy-buffer-size`, поскольку JWT с большим числом групп не помещается в буфер по умолчанию.

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

Если приложение публикуется через ресурс ALBInstance или ClusterALBInstance (подробнее — в документации модуля [`alb`](/modules/alb/)), при включении [`sendAuthorizationHeader: true`]((cr.html#dexauthenticator-v1-spec-sendauthorizationheader)) укажите заголовки, которые нужно передавать в приложение, в аннотации `alb.network.deckhouse.io/auth-response-headers` ресурса HTTPRoute:

```yaml
annotations:
  alb.network.deckhouse.io/auth-signin: https://app-name.kube.my-domain.com/dex-authenticator/sign_in
  alb.network.deckhouse.io/auth-url: https://app-name-dex-authenticator.app-ns.svc.cluster.local/dex-authenticator/auth
  alb.network.deckhouse.io/auth-response-headers: Authorization
```

В аннотации `alb.network.deckhouse.io/auth-response-headers`, достаточно указать только `Authorization`, так как она уже передаваемый по умолчанию базовый набор заголовков.

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

При разборе токена в приложении обращайте внимание на следующее:

- Для идентификации пользователя используйте поле `email`. Поле `sub` непрозрачно (не является предсказуемым и не может быть использовано как осмысленный идентификатор пользователя вне контекста конкретной системы), а `preferred_username` для статических пользователей пуст (внешние провайдеры аутентификации могут его заполнять).
- Поле `name` содержит имя объекта (из поля `metadata.name` объекта [User](cr.html#user)), а не отображаемое имя пользователя.
- Поле `aud` содержит идентификатор клиента аутентификатора (`<name>-<namespace>-dex-authenticator`), а не идентификатор OIDC-клиента вашего приложения. Приложение, проверяющее `aud` по собственному `client_id`, отклонит такой токен.
- Подпись проверяйте по JWKS `https://dex.<modules.publicDomainTemplate>/keys`.
- Время жизни токена определяется параметром [`idTokenTTL`](configuration.html#parameters-idtokenttl) (по умолчанию 10 минут). DexAuthenticator обновляет токен самостоятельно, приложение всегда получает актуальный.

Если приложение поддерживает OIDC самостоятельно, вместо DexAuthenticator используйте ресурс [DexClient](cr.html#dexclient): приложение само запросит необходимый объём полномочий и получит `refresh_token` в дополнение к `id_token`.

## Как работает аутентификация с помощью DexAuthenticator

![Как работает аутентификация с помощью DexAuthenticator](images/dex_login.svg)

{% alert level="warning" %}
DexAuthenticator работает только по HTTPS. Ingress-ресурсы, настроенные только на HTTP, не поддерживаются.

Аутентификационные cookie устанавливаются с атрибутом `Secure`, что означает их передачу только через зашифрованные HTTPS-соединения.

Убедитесь, что для Ingress вашего приложения настроен TLS, прежде чем интегрировать его с DexAuthenticator.
{% endalert %}

1. Dex в большинстве случаев перенаправляет пользователя на страницу входа провайдера и ожидает, что пользователь будет перенаправлен на его `/callback` URL. Однако такие провайдеры, как LDAP или Atlassian Crowd, не поддерживают этот вариант. Вместо этого пользователь должен ввести логин и пароль в форму входа в Dex, и Dex сам проверит учётные данные, выполнив запрос к API провайдера.

1. DexAuthenticator устанавливает cookie с полным токеном обновления (вместо выдачи тикета, как для ID-токена), потому что Redis не сохраняет данные на диск.
   Если по тикету в Redis не найден ID-токен, пользователь сможет запросить новый ID-токен, предоставив токен обновления из cookie.

1. DexAuthenticator выставляет HTTP-заголовок `Authorization`, равный значению ID-токена из Redis. Это необязательно для сервисов вроде [`upmeter`](/modules/upmeter/), так как права доступа к `upmeter` менее детальные.
   Для [Kubernetes Dashboard](/modules/dashboard/) это критичная функциональность: Dashboard передаёт ID-токен дальше для доступа к API Kubernetes.

## Как сгенерировать kubeconfig для доступа к Kubernetes API?

`kubeconfig` для удалённого доступа к кластеру через `kubectl` можно сгенерировать в [веб-интерфейсе `kubeconfigurator`](/products/kubernetes-platform/documentation/v1/user/web/kubeconfig.html).

Настройте параметр [`publishAPI`](/modules/user-authn/configuration.html#parameters-publishapi):

- Откройте настройки модуля `user-authn` (создайте ресурс ModuleConfig `user-authn`, если его нет):

  ```shell
  d8 k edit mc user-authn
  ```

- Добавьте следующую секцию в блок `settings` и сохраните изменения:

  ```yaml
  publishAPI:
    enabled: true
  ```

Имя `kubeconfig` зарезервировано для веб-интерфейса генерации kubeconfig. URL зависит от параметра [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) (например, при шаблоне вида `%s.kube.my` веб-интерфейс генерации kubeconfig будет доступен по адресу `kubeconfig.kube.my`, при `%s-kube.company.my` — по адресу `kubeconfig-kube.company.my`).  

### Настройка kube-apiserver

С помощью функций модуля [`control-plane-manager`](/modules/control-plane-manager/) DKP автоматически настраивает `kube-apiserver`, выставляя следующие флаги так, чтобы модули `dashboard` и `kubeconfig-generator` могли работать в кластере.

{% offtopic title="Аргументы kube-apiserver, которые будут настроены" %}

* `--oidc-client-id=kubernetes`
* `--oidc-groups-claim=groups`
* `--oidc-issuer-url=https://dex.%addonsPublicDomainTemplate%/`
* `--oidc-username-claim=email`

При использовании самоподписанных сертификатов для Dex добавляется ещё один аргумент, а в под `apiserver` монтируется файл CA:

* `--oidc-ca-file=/etc/kubernetes/oidc-ca.crt`
{% endofftopic %}

### Как работает подключение к Kubernetes API с помощью сгенерированного kubeconfig

![Схема взаимодействия при подключении к Kubernetes API с помощью сгенерированного kubeconfig](images/kubeconfig_dex.svg)

1. До начала работы `kube-apiserver` необходимо запросить конфигурационный эндпоинт OIDC провайдера (в нашем случае — Dex), чтобы получить issuer и настройки JWKS-эндпоинта.

1. Kubeconfig generator сохраняет ID-токен и Refresh-токен в файл `kubeconfig`.

1. После получения запроса с ID-токеном `kube-apiserver` проверяет, что токен подписан провайдером, настроенным на первом шаге, с помощью ключей, полученных с JWKS-эндпоинта. Затем сравнивает значения claim `iss` и `aud` из токена со значениями из конфигурации.

## Как сменить секрет OAuth2-клиента kubernetes?

Секрет привилегированного OAuth2-клиента `kubernetes` хранится в Secret `kubernetes-dex-client-app-secret` в неймспейсе `d8-user-authn`. То же значение используют OAuth2-клиенты `kubeconfig-generator`, `kubeconfig-publish-api` и `kubeconfig-<slug>`, а также компонент basic-auth-proxy, которому секрет передаётся с помощью параметра `--ldap-client-secret`.

Удаление Secret не приводит к смене секрета: пока значение сохраняется во внутренних параметрах модуля, хук восстановит Secret с прежним значением.

Чтобы сменить секрет, выполните следующие действия:

1. Если неймспейс `d8-user-authn` управляется с помощью GitOps-инструмента, исключите Secret `kubernetes-dex-client-app-secret` из синхронизации. Иначе GitOps-инструмент восстановит прежнее значение.

1. Очистите поле `secret`:

   ```shell
   d8 k -n d8-user-authn patch secret kubernetes-dex-client-app-secret --type merge -p '{"data":{"secret":""}}'
   ```

1. Перезапустите DKP, чтобы хук зарегистрировал пустое поле и сгенерировал новый секрет:

   ```shell
   d8 k -n d8-system rollout restart deployment/deckhouse
   ```

1. Убедитесь, что значение секрета изменилось:

   ```shell
   d8 k -n d8-user-authn get secret kubernetes-dex-client-app-secret -o jsonpath='{.data.secret}'
   ```

   Если значение не изменилось, повторите шаги 2 и 3. Модуль мог восстановить прежнее значение до перезапуска DKP.

После смены секрета конфигурация использующих его компонентов в кластере обновится автоматически, а их поды будут перезапущены.

Ранее загруженные файлы kubeconfig содержат прежний клиентский секрет и больше не смогут обновлять токены. Скачайте такие файлы заново. Уже выданные ID-токены продолжат действовать до истечения их срока действия, определяемого [параметром `settings.idTokenTTL`](configuration.html#parameters-idtokenttl) (по умолчанию — 10 минут).

## Как включить SSO по Kerberos (SPNEGO) для LDAP?

Если на стороне клиента настроено доменное SSO (браузер доверяет домену Dex), Dex может принимать Kerberos‑билеты по заголовку `Authorization: Negotiate` и выполнять аутентификацию без отображения формы ввода логина/пароля.

Включение SSO по Kerberos (SPNEGO) для LDAP:

1. В инфраструктуре клиента должен быть задан SPN `HTTP/<fqdn-dex>` для сервисного аккаунта и сгенерирован `keytab`.
1. В кластере создайте секрет в неймспейсе `d8-user-authn` с ключом `krb5.keytab`.
1. В ресурсе DexProvider (тип LDAP) включите блок [`spec.ldap.kerberos`](/modules/user-authn/cr.html#dexprovider-v1-spec-ldap-kerberos) и настройте в нём параметры:
   - `enabled: true`;
   - `keytabSecretName: <имя секрета>`;
   - опционально: `expectedRealm`, `usernameFromPrincipal`, `fallbackToPassword`.

Dex автоматически смонтирует `keytab` и начнёт принимать SPNEGO. `krb5.conf` на сервере не обязателен — билеты проверяются по keytab.

## Как настроить базовую аутентификацию для доступа к Kubernetes API через LDAP?

1. Включите параметр [`publishAPI`](/modules/user-authn/configuration.html#parameters-publishapi) в конфигурации модуля `user-authn`.
1. Создайте ресурс [DexProvider](/modules/user-authn/cr.html#dexprovider) типа `LDAP` и установите параметр [`enableBasicAuth: true`](/modules/user-authn/cr.html#dexprovider-v1-spec-oidc-enablebasicauth).
1. Настройте [RBAC](/modules/user-authz/cr.html#clusterauthorizationrule) для групп, получаемых из LDAP.
1. Передайте пользователям `kubeconfig` с настроенными параметрами базовой аутентификации (логин и пароль LDAP).

{% alert level="warning" %}
В кластере может быть только один провайдер аутентификации со включенным параметром [`enableBasicAuth`](/modules/user-authn/cr.html#dexprovider-v1-spec-oidc-enablebasicauth).
{% endalert %}

Подробный пример описан в разделе [Примеры конфигурации](/modules/user-authn/usage.html#настройка-базовой-аутентификации).

## Как Dex защищен от подбора логина и пароля?

Каждому пользователю разрешено не более 20 попыток входа. После исчерпания лимита одна дополнительная попытка добавляется каждые 6 секунд.

## UserOperation в статусе Failed — что делать?

Проверьте поле `status.message` ресурса UserOperation, чтобы узнать описание ошибки:

```shell
d8 k get useroperation <имя> -o jsonpath='{.status.message}'
```

Устраните причину (например, неверный хеш пароля или несуществующий пользователь), затем создайте новый UserOperation. UserOperation неизменяем — его спецификацию нельзя изменить после создания.

## Как разблокировать пользователя?

Используйте команду:

```shell
d8 iam user unlock <имя>
```

Либо создайте новый ресурс UserOperation с `type: Unlock`. Учтите, что операции `ResetPassword`, `Reset2FA` и `Lock` завершают все активные сессии пользователя.

## Пользователь заблокирован автоматически — почему?

Количество неудачных попыток входа превысило [`passwordPolicy.lockout.maxAttempts`](configuration.html#parameters-passwordpolicy-lockout-maxattempts). Пользователь блокируется на время, указанное в [`passwordPolicy.lockout.lockDuration`](configuration.html#parameters-passwordpolicy-lockout-lockduration), после чего разблокируется автоматически. Администратор может также разблокировать пользователя вручную командой `d8 iam user unlock <имя>` или создав UserOperation с `type: Unlock`.

## Можно ли отменить операцию UserOperation?

Нет. UserOperation — одноразовый неизменяемый объект. Чтобы отменить эффект операции, нужно создать обратную — например, создать UserOperation с `type: Unlock` после операции `Lock`.
