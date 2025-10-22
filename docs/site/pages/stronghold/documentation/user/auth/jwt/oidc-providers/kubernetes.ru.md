---
title: "Kubernetes как провайдер OIDC"
permalink: ru/stronghold/documentation/user/auth/jwt/oidc-providers/kubernetes.html
lang: ru
description: OIDC provider configuration for Kubernetes
---

## Kubernetes

Kubernetes может выступать в качестве провайдера OIDC, чтобы Stronghold мог подтверждать токены сервисных учетных записей с помощью JWT/OIDC auth.

{% alert %}Механизм JWT-аутентификации **не** использует при аутентификации API Kubernetes `TokenReview` , а вместо этого используется криптография с открытым ключом для проверки содержимого JWT. Это означает, что токены, которые были отозваны Kubernetes, будут считаться действительными до истечения срока их действия. Чтобы снизить этот риск, используйте короткие TTL для токенов сервисных учетных записей или используйте [Kubernetes auth](../../kubernetes.html), который использует API `TokenReview`.
{% endalert %}

### Использование адреса автонастройки

При использовании автонастройки вам нужно указать только OIDC discovery URL. В случае, если OIDC URL использует кастомный сертификат, так же понадобится CA, которому можно доверять. Это режим наиболее простой в настройке, если ваш кластер Kubernetes соответствует требованиям.

Требования к кластеру Kubernetes:

* Включенная опция [`ServiceAccountIssuerDiscovery`][k8s-sa-issuer-discovery].
  * Доступна с версии 1.18, включена по умолчанию с версии 1.20.
* Значение URL в параметре  kube-apiserver-а `--service-account-issuer` должно содержать адрес, доступный из Stronghold. Для большинства managed сервисов Kubernetes этот адрес публичный.
* Должны использоваться короткоживущие токены для сервисных аккаунтов Kubernetes.
  * По умолчанию такое поведение включено для токенов, подключаемых в поды, начиная с Kubernetes 1.21.

Шаги по настройке:

Убедитесь, что URL адреса обнаружения OIDC не требует аутентификации, как описано [тут][k8s-sa-issuer-discovery]:

```bash
d8 k create clusterrolebinding oidc-reviewer  \
   --clusterrole=system:service-account-issuer-discovery \
   --group=system:unauthenticated
```

Определите адрес issuer URL для вашего кластера.

```bash
ISSUER="$(d8 k get --raw /.well-known/openid-configuration | jq -r '.issuer')"
```

Включите и настройте аутентификацию JWT в Stronghold.

```bash
d8 stronghold auth enable jwt
d8 stronghold write auth/jwt/config oidc_discovery_url="${ISSUER}"
```

Настройте необходимые роли, как описано [ниже](#создание-ролей-и-аутентификация).

[k8s-sa-issuer-discovery]: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-issuer-discovery

### Использование публичных ключей для проверки JWT

Этот метод может быть полезен, если API Kubernetes недоступен из Stronghold или если вы хотите, чтобы один эндпоинт JWT auth обслуживал несколько кластеров Kubernetes, используя цепочку публичных ключей.

Требования к кластеру Kubernetes:

* Включенная опция [`ServiceAccountIssuerDiscovery`][k8s-sa-issuer-discovery].
  * Доступна с версии 1.18, включена по умолчанию с версии 1.20.
  * Этого требование не обязательно, если вы имеете доступ файлу `/etc/kubernetes/pki/sa.pub` на master-узле кластера. В этом случае вы можете пропустить шаги по получению ключа и конвертации его в формат PEM, так как ключ уже находится в файле в нужном формате.
* Должны использоваться короткоживущие токены для сервисных аккаунтов Kubernetes.
  * По умолчанию такое поведение включено для токенов, подключаемых в поды, начиная с Kubernetes 1.21.

Шаги по настройке:

Получите открытый ключ подписи токенов сервис-аккаунтов из JWKS URI вашего кластера.

```bash
# jwks_uri доступен в /.well-known/openid-configuration
d8 k get --raw "$(d8 k get --raw /.well-known/openid-configuration | jq -r '.jwks_uri' | sed -r 's/.*\.[^/]+(.*)/\1/')"
```

Преобразуйте ключи из формата JWK в формат PEM. Вы можете сделать это с помощью консольной утилиты, или любого онлайн-сервиса, например [этого][jwk-to-pem].

Настройте эндпоинт JWT auth на использование полученных ключей.

```bash
d8 stronghold write auth/jwt/config \
   jwt_validation_pubkeys="-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9...
-----END PUBLIC KEY-----","-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9...
-----END PUBLIC KEY-----"
```

Настройте необходимые роли, как описано [ниже](#создание-ролей-и-аутентификация).

[jwk-to-pem]: https://8gwifi.org/jwkconvertfunctions.jsp

### Создание ролей и аутентификация

После того как эндпоинт JWT-auth настроен, вы можете настроить роль и пройти аутентификацию. Далее предполагается, что вы используете токен учетной записи сервиса, доступный по умолчанию во всех подах. Если вы хотите контролировать целевую группу (audience) или TTL, смотрите раздел [Указание TTL и целевой группы](#указание-ttl-и-целевых-групп-specifying-ttl-and-audience).

Выберите любое значение из набора стандартных целевых групп по умолчанию. В этих примерах в массиве `aud` есть только одна целевая группа, `https://kubernetes.default.svc.cluster.local`.

Чтобы найти целевую группу по умолчанию, вы можете создать новый токен (требуется `kubectl` v1.24.0+):

```shell-session
$ d8 k create token default | cut -f2 -d. | base64 --decode
{"aud":["https://kubernetes.default.svc.cluster.local"], ... "sub":"system:serviceaccount:default:default"}
```

Или прочитать токен из запущенного пода:

```shell-session
$ d8 k exec my-pod -- cat /var/run/secrets/kubernetes.io/serviceaccount/token | cut -f2 -d. | base64 --decode
{"aud":["https://kubernetes.default.svc.cluster.local"], ... "sub":"system:serviceaccount:default:default"}
```

Создайте роль для JWT-auth, которую сможет использовать сервис-аккаунт `default` в неймспейсе `default`.

```bash
d8 stronghold write auth/jwt/role/my-role \
   role_type="jwt" \
   bound_audiences="<AUDIENCE-FROM-PREVIOUS-STEP>" \
   user_claim="sub" \
   bound_subject="system:serviceaccount:default:default" \
   policies="default" \
   ttl="1h"
```

Теперь, с помощью этого токена поды, или клиенты, имеющие доступ к JWT сервис-аккаунта, смогут аутентифицироваться.

```bash
d8 stronghold write auth/jwt/login \
   role=my-role \
   jwt=@/var/run/secrets/kubernetes.io/serviceaccount/token
# OR equivalent to:
curl \
   --fail \
   --request POST \
   --data '{"jwt":"<JWT-TOKEN-HERE>","role":"my-role"}' \
   "${STRONGHOLD_ADDR}/v1/auth/jwt/login"
```

### Указание TTL и целевых групп {#specifying-ttl-and-audience}

Если вы хотите указать пользовательский TTL или целевую группу для токенов сервисных учетных записей, в следующем манифесте пода показано монтирование тома, которое переопределяет инжектируемый токен по умолчанию. Это особенно актуально, если вы не можете отключить флаг [--service-account-extend-token-expiration][k8s-extended-tokens] для `kube-apiserver` и хотите использовать короткие TTL.

При использовании полученного токена вам нужно будет установить `bound_audiences=stronghold` при создании ролей в JWT auth.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
spec:
  # automountServiceAccountToken является лишним в этом примере, поскольку используемый
  # используемый mountPath совпадает с путем по умолчанию. Это перекрытие предотвращает
  # создание токена по умолчанию. Однако вы можете использовать этот параметр, чтобы
  # обеспечить монтирование только одного токена, если вы выберете другой путь монтирования.
  automountServiceAccountToken: false
  containers:
    - name: nginx
      image: nginx
      volumeMounts:
      - name: custom-token
        mountPath: /var/run/secrets/kubernetes.io/serviceaccount
  volumes:
  - name: custom-token
    projected:
      defaultMode: 420
      sources:
      - serviceAccountToken:
          path: token
          expirationSeconds: 600 # Минимальный TTL 10 минут
          audience: stronghold   # Должен совпадать с параметром `bound_audiences` вашей роли
      # Остальные параметры добавлены для имитации обычного поведения при создании токена,
      # и создают объекты, которые создаются при включенном параметре automountServiceAccountToken
      - configMap:
          name: kube-root-ca.crt
          items:
          - key: ca.crt
            path: ca.crt
      - downwardAPI:
          items:
          - fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
            path: namespace
```

[k8s-extended-tokens]: https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/#options
