---
title: "Настройка аутентификации и авторизации"
permalink: ru/admin/configuration/delivery/argocd/authentication/
description: "Настройка аутентификации и авторизации Argo CD в Deckhouse Kubernetes Platform."
lang: ru
relatedLinks:
  - title: "Официальный сайт проекта Argo CD"
    url: "https://argo-cd.readthedocs.io"
  - title: "Официальный сайт проекта Argo CD Operator"
    url: "https://argocd-operator.readthedocs.io"
---

Argo CD поддерживает локальную аутентификацию, а также интегрирован с подсистемой идентификации и доступа Deckhouse Kubernetes Platform.
В документации DKP можно подробнее узнать о настройке [аутентификации](../../../access/authentication/) и [авторизации](../../../access/authorization/).

{% alert level="warning" %}
Если в объекте ArgoCD не заданы дополнительные настройки, по умолчанию активна локальная учётная запись `admin` с ролью `admin`.
{% endalert %}

Об управлении полномочиями пользователей и групп можно узнать в разделе [«Настройка ролевой модели доступа»](../rbac/).

## Локальная аутентификация

При создании объекта ArgoCD автоматически создаётся пользователь `admin` с ролью `admin`. Пароль пользователя `admin` генерируется автоматически. Чтобы получить его, выполните команду:

```bash
d8 k -n argocd get secret argocd-cluster -o jsonpath='{.data.admin\.password}' | base64 -d
```

### Создание дополнительных локальных пользователей

При создании локального пользователя можно определить, будут ли у него права на доступ к веб-интерфейсу Argo CD (атрибут `login`) и/или к API Argo CD (атрибут `apiKey`).

Чтобы создать локальных пользователей, задайте параметр [`spec.localUsers`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-localusers) ArgoCD, например:

```bash
d8 k create -f - <<EOF
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  localUsers:
    # Пользователь "deploy" с правами на вход в веб-интерфейс и генерацию токенов.
    - name: deploy
      apiKey: true
      login: true
      enabled: true
    # Пользователь "ci-bot" только для автоматизации, без доступа к веб-интерфейсу.
    - name: ci-bot
      apiKey: true
      login: false
      enabled: true
EOF
```

После создания пользователя и применения манифеста ArgoCD дополнительно сгенерируйте пароль для доступа к веб-интерфейсу, если пользователю выданы соответствующие права.

Чтобы задать пароль через Secret, сначала вычислите его bcrypt-хеш, а затем закодируйте результат в Base64:

```bash
ARGOCD_USER=<имя пользователя>
ARGOCD_PASS=$(echo "<пароль пользователя>" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0)
d8 k -n argocd patch secret argocd-secret -p "{\"data\":{\"accounts.$ARGOCD_USER.password\":\"$ARGOCD_PASS\"}}"
```

{% alert level="info" %}
Пароль локального пользователя можно задать или изменить с помощью CLI-утилиты `argocd`:

```bash
argocd login <fqdn Argo CD ingress> --username admin --password <admin-пароль>
argocd account update-password \
  --account <аккаунт> \
  --current-password <admin-пароль> \
  --new-password <желаемый-пароль>
```

{% endalert %}

### Создание пользовательского токена

Для работы с API у пользователя должны быть соответствующие права (параметр [`apiKey`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-localusers-apikey), пример в разделе выше). Чтобы выпустить токен, используйте веб-интерфейс Argo CD или CLI-утилиту `argocd`.

Чтобы выпустить токен через веб-интерфейс, перейдите в раздел «Settings» → «Accounts», выберите нужного пользователя и нажмите «Generate New» в разделе «Tokens».

Чтобы выпустить токен с помощью CLI-утилиты `argocd`, выполните команды (укажите необходимые значения):

```bash
argocd login <ARGOCD_DOMAIN>:443 --username admin
argocd account generate-token --account <аккаунт>
```

{% alert level="info" %}
Если у пользователя нет прав на доступ к веб-интерфейсу (`login`), токен для него может выпустить другой пользователь с правами администратора.
{% endalert %}

### Отключение локальной аутентификации

Чтобы отключить локальную аутентификацию, отключите пользователя `admin`, задав параметру [`spec.disableAdmin`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-disableadmin) ArgoCD значение `true`. Также удалите всех дополнительных локальных пользователей, если они были созданы ранее (подробнее — в разделе [«Создание дополнительных локальных пользователей»](#создание-дополнительных-локальных-пользователей)).

После отключения локальной аутентификации проверьте правила доступа в разделе [«Настройка ролевой модели доступа»](../rbac/).

## Аутентификация с помощью SSO

Перед настройкой объекта ArgoCD создайте OAuth2-клиент, создав объект DexClient, необходимый для интеграции с Deckhouse Kubernetes Platform:

```bash
d8 k create -f -<<EOF
apiVersion: deckhouse.io/v1
kind: DexClient
metadata:
  name: argocd
  namespace: argocd
spec:
  redirectURIs:
    - https://<ARGOCD_DOMAIN>/api/dex/callback
    - https://<ARGOCD_DOMAIN>/api/dex/callback-reserve
EOF
```

`<ARGOCD_DOMAIN>` — это полное доменное имя (Fully Qualified Domain Name, FQDN), заданное в секции [`.spec.server.host`](/modules/operator-argo/cr.html#argocd-v1beta1-spec-server-host) ArgoCD.

Дождитесь, пока Deckhouse Kubernetes Platform создаст Secret с секретным ключом для клиента:

```shell
d8 k -n argocd get secret/dex-client-argocd
```

Настройте объект ArgoCD на использование SSO в Deckhouse Kubernetes Platform:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  sso:
    dex:
      config: |
        connectors:
          - type: oidc
            id: deckhouse
            name: deckhouse
            config:
              issuer: "https://dex.<CLUSTER_DOMAIN>/"
              clientID: "dex-client-argocd@argocd"
              clientSecret: "$dex-client-argocd:clientSecret"
              insecureEnableGroups: true
              scopes:
                - profile
                - email
                - openid
                - groups
    provider: dex
  server:
    host: <ARGOCD_DOMAIN>
    ingress:
      enabled: true
      ingressClassName: <INGRESS_CLASS_NAME>
      tls:
        - hosts:
            - <ARGOCD_DOMAIN>
          secretName: argocd-ingress-tls
    insecure: true
```

Перезапустите сервер Argo CD:

```shell
d8 k -n argocd rollout restart deploy/argocd-server
```

{% alert level="warning" %}
Если не перезапустить сервер Argo CD, попытка входа завершится ошибкой, а в журнале сервера Argo CD появится сообщение об ошибке ([issue argoproj/argo-cd#13526](https://github.com/argoproj/argo-cd/issues/13526)).

{% offtopic title="Пример сообщения об ошибке..." %}

<!-- markdownlint-disable MD013 -->

```text
time="2024-10-16T14:12:59Z" level=warning msg="Failed to verify token: failed to verify token: token verification failed for all audiences: error for aud \"argo-cd\": Failed to query provider \"https://argocd.<ARGOCD_DOMAIN>/api/dex\": Get \"https://argocd.<ARGOCD_DOMAIN>/api/dex/.well-known/openid-configuration\": tls: failed to verify certificate: x509: certificate is valid for ingress.local, not argocd.<ARGOCD_DOMAIN>, error for aud \"argo-cd-cli\": Failed to query provider \"https://argocd.<ARGOCD_DOMAIN>/api/dex\": Get \"https://argocd.<ARGOCD_DOMAIN>/api/dex/.well-known/openid-configuration\": tls: failed to verify certificate: x509: certificate is valid for ingress.local, not argocd.<ARGOCD_DOMAIN>"
```

<!-- markdownlint-enable MD013 -->

{% endofftopic %}
{% endalert %}

### Использование самоподписного сертификата

Предварительно получите самоподписанный сертификат, используемый подсистемой идентификации и доступа Deckhouse Kubernetes Platform:

```bash
d8 k -n d8-user-authn get secret ingress-tls -o jsonpath='{.data.tls\.crt}' | base64 -d
```

Затем добавьте полученный сертификат в конфигурацию OIDC-коннектора, указав его в секции `rootCAs` ArgoCD:

```yaml
apiVersion: argoproj.io/v1beta1
kind: ArgoCD
metadata:
  name: argocd
  namespace: argocd
spec:
  sso:
    dex:
      config: |
        connectors:
          - type: oidc
            id: deckhouse
            name: deckhouse
            config:
              issuer: "https://dex.<cluster-domain>/"
              rootCAs:
                - |
                  -----BEGIN CERTIFICATE-----
                  <Самоподписанный сертификат, полученный на предыдущем шаге>
                  -----END CERTIFICATE-----
              clientID: "dex-client-argocd@argocd"
              clientSecret: "$dex-client-argocd:clientSecret"
              insecureEnableGroups: true
              scopes:
                - profile
                - email
                - openid
                - groups
    provider: dex
  server:
    host: <ARGOCD_DOMAIN>
    ingress:
      enabled: true
      tls:
        - hosts:
            - <ARGOCD_DOMAIN>
          secretName: argocd-ingress-tls
    insecure: true
```

### Создание пользовательского токена

Argo CD не позволяет выпускать бессрочные токены для пользователей, аутентифицированных с помощью SSO. При этом такие пользователи могут использовать CLI-утилиту `argocd`, указав при аутентификации флаг `--sso`:

```bash
argocd login <ARGOCD_DOMAIN>:443 --sso
```

При выполнении этой команды на рабочей станции администратора будет запущен веб-браузер с формой аутентификации в Deckhouse Kubernetes Platform.

{% alert level="info" %}
Чтобы настроить бессрочный токен, создайте локального пользователя Argo CD и укажите для него атрибут `apiKey`. В этом случае у пользователя будут права доступа только к Argo CD API. Подробнее см. в разделе [«Создание дополнительных локальных пользователей»](../authentication/#создание-дополнительных-локальных-пользователей).
{% endalert %}
