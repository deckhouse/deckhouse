---
title: Метод аутентификации Kubernetes
permalink: ru/stronghold/documentation/user/auth/kubernetes.html
lang: ru
description: |-
  The Kubernetes auth method allows automated authentication of Kubernetes
  Service Accounts.
---

Метод `kubernetes` auth можно использовать для аутентификации в Stronghold с помощью токена учетной записи сервиса Kubernetes. Этот метод аутентификации позволяет легко использовать Stronghold в Kubernetes Pod-ах.

Вы также можете использовать токен учетной записи сервиса Kubernetes для [входа в систему через JWT-аутентификацию][k8s-jwt-auth].
См. раздел [Как работать с короткоживущими токенами Kubernetes](#short-lived-tokens) где описано, почему вы можете захотеть использовать JWT auth вместо Kubernetes auth.

## Аутентификация

### Через CLI

По умолчанию используется путь `/kubernetes_local`. Если этот метод аутентификации был включен по другому пути, укажите `-path=/my-path` в CLI.

```shell-session
d8 stronghold write auth/kubernetes/login role=demo jwt=...
```

### Через API

По умолчанию используется эндпоинт `auth/kubernetes_local/login`. Если метод авторизации был включен по другому пути, используйте это значение вместо `kubernetes_local`.

```shell-session
$ curl \
    --request POST \
    --data '{"jwt": "<your service account jwt>", "role": "demo"}' \
    https://stronghold.example.com/v1/auth/kubernetes/login
```

Ответ будет содержать токен по в поле `auth.client_token`:

```json
{
  "auth": {
    "client_token": "38fe9691-e623-7238-f618-c94d4e7bc674",
    "accessor": "78e87a38-84ed-2692-538f-ca8b9f400ab3",
    "policies": ["default"],
    "metadata": {
      "role": "demo",
      "service_account_name": "myapp",
      "service_account_namespace": "default",
      "service_account_secret_name": "myapp-token-pd21c",
      "service_account_uid": "aa9aa8ff-98d0-11e7-9bb7-0800276d99bf"
    },
    "lease_duration": 2764800,
    "renewable": true
  }
}
```

## Конфигурация

Методы аутентификации должны быть настроены заранее, прежде чем пользователи или машины смогут пройти аутентификацию. Эти шаги обычно выполняются оператором или инструментом управления конфигурацией.

В Stronghold по умолчанию включен метод Kubernetes по пути `kubernetes_local`, позволяющий аутентифицировать приложения, запущенные в том же кластере, где запущен Stronghold.
Вы можете добавить в Stronghold другой кластер kubernetes.

1. Включите метод аутентификации Kubernetes:

```bash
d8 stronghold auth enable kubernetes
```

1. Используйте эндпоинт `/config`, чтобы настроить Stronghold на взаимодействие с новым кластером Kubernetes. Используйте `d8 k cluster-info` для получения адреса хоста Kubernetes и TCP-порта.

```bash
d8 stronghold write auth/kubernetes/config \
   token_reviewer_jwt="<your reviewer service account JWT>" \
   kubernetes_host=https://192.168.99.100:<your TCP port or blank for 443> \
   kubernetes_ca_cert=@ca.crt
```

{% alert level="warning" %}
Шаблон, используемый Stronghold для аутентификации подов, зависит от обмена JWT-токеном по сети. Учитывая модель безопасности Stronghold, это допустимо, поскольку Stronghold является частью доверенной вычислительной системы. В целом, приложения Kubernetes не должны передавать этот JWT другим приложениям, поскольку он позволяет выполнять вызовы API от имени подов, что может привести к непреднамеренному предоставлению доступа третьим лицам.
{% endalert %}

1. Создайте именованную роль:

```text
d8 stronghold write auth/kubernetes/role/demo \
   bound_service_account_names=myapp \
   bound_service_account_namespaces=default \
   policies=default \
   ttl=1h
```

  Эта роль авторизует учетную запись службы `myapp` в неймспейсе
  `default` и назначает ей политику по умолчанию.

## Kubernetes 1.21

Начиная с версии [Kubernetes 1.21](https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.21.md#api-change-2), функция Kubernetes `BoundServiceAccountTokenVolume` по умолчанию включена. Начиная с этой версии, JWT-токен, добавляемый в контейнеры по умолчанию:

* Имеет срок действия и привязан к сроку жизни пода и учетной записи сервиса.
* Значение параметра JWT `«iss»` зависит от конфигурации кластера.

Изменения в сроке жизни токена важны при настройке опции `token_reviewer_jwt`. Если используется недолговечный токен, Kubernetes отзовет его, как только под или учетная запись сервиса будут удалены, или если пройдет время истечения срока действия, и Stronghold больше не сможет использовать API `TokenReview`. Подробности работы см. в разделе [Как работать с недолговечными токенами Kubernetes](#short-lived-tokens).

По этой причине Kubernetes auth по умолчанию не проверяет эмитента (`iss`). API Kubernetes выполняет ту же проверку при просмотре токенов, поэтому проверку эмитента на стороне Stronghold повторно делать не нужно.

### Как работать с недолговечными токенами kubernetes {#short-lived-tokens}

Существует несколько различных способов настроить аутентификацию для подов Kubernetes,
когда смонтированные по умолчанию токены подов недолговечны, каждый из которых имеет свои преимущества.
Эта таблица содержит краткое описание вариантов, каждый из которых более подробно рассматривается ниже.

| Вариант | Все токены короткоживущие | Можно досрочно отозвать токены | Другие варианты |
|--------------------------------------|----------------------------|-------------------------|-----------------------------------------------------------|
| Использовать локальный токен в качестве JWT для валидации | Да | Да | Требуется развертывание Stronghold на кластере Kubernetes | Да | Да.
| Использовать клиентский JWT в качестве JWT для валидации | Да | Да | Эксплуатационные расходы |
Использовать долгоживущий токен в качестве JWT для валидации | Нет | Да | | |
| Использовать JWT-аутентификацию вместо этого | Да | Нет | Да. |

{% alert %}
По умолчанию Kubernetes продлевает срок службы токенов сервис аккаунтов до года, чтобы помочь сгладить переход к короткоживущим токенам.
Если вы хотите отключить эту функцию, задайте [--service-account-extend-token-expiration=false](https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/#options) для `kube-apiserver` или определите собственную конфигурацию тома `serviceAccountToken`.
Подробный пример можно найти [в документации](jwt/oidc-providers/kubernetes.html#specifying-ttl-and-audience).
{% endalert %}

#### Использование токена Stronghold в качестве рецензента JWT

При запуске Stronghold в поде Kubernetes используется сервис аккаунт, позволяющий производить проверку токенов приложений, запущенных в том же кластере Kubernetes, что и Stronghold

#### Используйте JWT клиента в качестве рецензента JWT

При настройке Kubernetes auth вы можете опустить `token_reviewer_jwt`, и Stronghold
будет использовать JWT клиента Stronghold в качестве своего собственного токена при взаимодействии с
API Kubernetes `TokenReview`. Вам также необходимо установить значение `disable_local_ca_jwt=true`.

Это означает, что Stronghold не хранит никаких JWT и позволяет использовать короткоживущие токены
везде, но добавляет некоторые операционные накладные расходы на поддержание ролей кластера
привязки кластерной роли к набору учетных записей служб, которые вы хотите иметь возможность аутентифицировать в
Stronghold. Каждому клиенту Stronghold потребуется кластерная роль `system:auth-delegator:

```bash
d8 k create clusterrolebinding myapp-client-auth-delegator \
  --clusterrole=system:auth-delegator \
  --group=group1 \
  --serviceaccount=default:svcaccount1 \
  ...
```

#### Использование долгоживущих токенов

Вы можете создать долгоживущий токен, используя инструкции [здесь][k8s-create-secret]
и использовать его в качестве `token_reviewer_jwt`. В этом примере для службы `myapp`
потребуется кластерная роль `system:auth-delegator:

```bash
d8 k apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: myapp-k8s-auth-secret
  annotations:
    kubernetes.io/service-account.name: myapp
type: kubernetes.io/service-account-token
EOF
```

Использование этого способа позволяет упростить настройку, но не позволит воспользоваться преимуществами улучшенной безопасности короткоживущих токенов.

[k8s-create-secret]: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#manually-create-a-service-account-api-token

#### Использование JWT-аутентификации

Kubernetes auth использует API `TokenReview` от Kubernetes. Однако
JWT-токены, генерируемые Kubernetes, также могут быть проверены с помощью Kubernetes в качестве OIDC
провайдера. В документации по методу JWT auth есть [инструкции][k8s-jwt-auth]
по настройке JWT-auth с использованием Kubernetes в качестве OIDC-провайдера.

[k8s-jwt-auth]: jwt/oidc-providers/kubernetes.html

Это решение позволяет использовать короткоживущие токены для всех клиентов и убирает
необходимость в рецензируемом JWT. Однако клиентские токены не могут быть отозваны до того, как
истечения их TTL, поэтому рекомендуется держать TTL коротким с учетом этого
ограничение.

## Конфигурирование kubernetes

Этот метод авторизации обращается к `Kubernetes TokenReview API``, для
проверки того, что предоставленный JWT все еще действителен.

Учетные записи служб, используемые в этом методе аутентификации, должны иметь доступ к
TokenReview API. Учетной записи службы должны быть предоставлены разрешения на доступ к этому API.
Следующий пример ClusterRoleBinding может быть использован для предоставления этих разрешений:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: role-tokenreview-binding
  namespace: default
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
  - kind: ServiceAccount
    name: myapp-auth
    namespace: default
```
