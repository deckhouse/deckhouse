---
title: Kubernetes
permalink: ru/stronghold/documentation/user/auth-methods/kubernetes.html
lang: ru
description: |-
  The Kubernetes auth method allows automated authentication of Kubernetes
  Service Accounts.
---

# Метод аутентификации Kubernetes

{% alert level="warning" %}

**Примечание**: Этот механизм может использовать внешние сертификаты X.509 в качестве части TLS или проверки подписи.
   Проверка подписей на сертификатах X.509, использующих SHA-1, устарела и больше не
   и больше не может использоваться без обходного пути. См.
   [deprecation FAQ](/docs/deprecation/faq#q-what-is-the-impact-of-removing-support-for-x-509-certificates-with-signatures-that-use-sha-1)
   для получения дополнительной информации.

{% endalert %}
Метод `kubernetes` auth можно использовать для аутентификации в Stronghold с помощью
Токен учетной записи сервиса Kubernetes. Этот метод аутентификации позволяет легко
внедрить токен Stronghold в Kubernetes Pod.

Вы также можете использовать токен учетной записи сервиса Kubernetes для [входа в систему через JWT-аутентификацию][k8s-jwt-auth].
См. раздел [Как работать с короткоживущими токенами Kubernetes][short-lived-tokens]
где кратко описано, почему вы можете захотеть использовать JWT auth вместо этого и как он сравнивается с
Kubernetes auth.

## Аутентификация

### Через CLI

По умолчанию используется путь `/kubernetes`. Если этот метод аутентификации был включен по
другой путь, укажите `-path=/my-path` в CLI.

```shell-session
$ d8 stronghold write auth/kubernetes/login role=demo jwt=...
```

### Через API

По умолчанию используется конечная точка `auth/kubernetes/login`. Если этот метод авторизации был включен
по другому пути, используйте это значение вместо `kubernetes`.

```shell-session
$ curl \
    --request POST \
    --data '{"jwt": "<your service account jwt>", "role": "demo"}' \
    http://127.0.0.1:8200/v1/auth/kubernetes/login
```

Ответ будет содержать токен по адресу `auth.client_token`:

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

Методы аутентификации должны быть настроены заранее, прежде чем пользователи или машины смогут
аутентификации. Эти шаги обычно выполняются оператором или инструментом управления конфигурацией
управления конфигурацией.

1. Включите метод аутентификации Kubernetes:

  ```bash
  d8 stronghold auth enable kubernetes
  ```

1. Используйте конечную точку `/config`, чтобы настроить Stronghold на взаимодействие с Kubernetes. Используйте
  `d8 kubectl cluster-info` для проверки адреса хоста Kubernetes и TCP-порта.
  Список доступных опций конфигурации см.
  [документация API](/api-docs/auth/kubernetes).

  ```bash
  d8 stronghold write auth/kubernetes/config \
      token_reviewer_jwt="<your reviewer service account JWT>" \
      kubernetes_host=https://192.168.99.100:<your TCP port or blank for 443> \
      kubernetes_ca_cert=@ca.crt
  ```

{% alert level="critical" %}
 **Примечание:** Шаблон, используемый Stronghold для аутентификации подов, зависит от возможности
  передачи JWT-токена по сети. Учитывая [модель безопасности
  Stronghold] (/docs/internals/security), это допустимо, поскольку Stronghold является
  частью базы доверенных вычислений. В целом, приложения Kubernetes должны
  **не** передавать этот JWT другим приложениям, поскольку он позволяет осуществлять вызовы API
  от имени подсистемы и может привести к непреднамеренному предоставлению доступа
  третьим лицам.

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

  Полный список параметров конфигурации можно найти в [API
  документацию](/api-docs/auth/kubernetes).

## Kubernetes 1.21

Начиная с версии [1.21][k8s-1.21-changelog], Kubernetes
опция `BoundServiceAccountTokenVolume` по умолчанию имеет значение enabled. Это изменяет
JWT-токен, установленный в контейнеры по умолчанию, двумя способами, важными для
Kubernetes auth:

* Он имеет срок действия и привязан к сроку жизни пода и учетной записи сервиса.
* Значение параметра JWT `«iss»` зависит от конфигурации кластера.

Изменение времени жизни токена важны при настройке
[`token_reviewer_jwt`](/api-docs/auth/kubernetes#token_reviewer_jwt).
Если используется токен с коротким сроком действия,
Kubernetes отзовет его, как только под или учетная запись сервиса будут удалены, или
если истечет срок действия, и Stronghold больше не сможет использовать
API `TokenReview`. См. раздел [Как работать с недолговечными токенами Kubernetes][short-lived-tokens]
ниже для получения подробной информации о работе с этим изменением.

В связи с этими изменениями, Kubernetes auth был обновлен, чтобы по умолчанию
не проверять эмитента. API Kubernetes выполняет ту же проверку при
при проверке токенов, поэтому включение проверки эмитента на стороне Stronghold является
дублированием. Без отключения проверки эмитента в Stronghold невозможно,
чтобы одна конфигурация Kubernetes auth работала для смонтированных по умолчанию
pod-токенов в Kubernetes 1.20 и 1.21.
См. раздел [Обнаружение службы учетной записи `issuer`](#discovering-the-service-account-issuer)
ниже для получения рекомендаций, если вы хотите включить проверку эмитента в Stronghold.

[k8s-1.21-changelog]: https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG/CHANGELOG-1.21.md#api-change-2
[short-lived-tokens]: #how-to-work-with-short-lived-kubernetes-tokens

### Как работать с недолговечными токенами kubernetes

Существует несколько различных способов настроить аутентификацию для подов Kubernetes,
когда смонтированные по умолчанию токены подов недолговечны, каждый из которых имеет свои преимущества.
Эта таблица содержит краткое описание вариантов, каждый из которых более подробно рассматривается ниже.

| Вариант | Все токены короткоживущие | Можно досрочно отозвать токены | Другие варианты |
|--------------------------------------|----------------------------|-------------------------|-----------------------------------------------------------|
| Использовать локальный токен в качестве JWT для валидации | Да | Да | Требуется развертывание Stronghold на кластере Kubernetes | Да | Да.
| Использовать клиентский JWT в качестве JWT для валидации | Да | Да | Эксплуатационные расходы |
Использовать долгоживущий токен в качестве JWT для валидации | Нет | Да | | |
| Использовать JWT-аутентификацию вместо этого | Да | Нет | Да. |

{% alert level="info" %}

**Примечание:** По умолчанию Kubernetes в настоящее время продлевает срок службы
вводимых токенов учетных записей служб до года, чтобы помочь сгладить переход к
короткоживущим токенам. Если вы хотите отключить эту функцию, задайте
[--service-account-extend-token-expiration=false][k8s-extended-tokens] для
`kube-apiserver` или укажите собственное монтирование тома `serviceAccountToken`. См.
[здесь](/docs/auth/jwt/oidc-providers/kubernetes#specifying-ttl-and-audience) для примера.

{% endalert %}

[k8s-extended-tokens]: https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/#options

#### Использование токена локальной учетной записи службы в качестве JWT рецензента

При запуске Stronghold в поде Kubernetes рекомендуется использовать локальный
токен учетной записи сервиса. Stronghold будет периодически перечитывать файл, чтобы поддерживать
кородкоживущие токены. Чтобы использовать локальный токен и сертификат центра сертификации, опустите
`token_reviewer_jwt` и `kubernetes_ca_cert` при настройке метода аутентификации.
Stronghold попытается загрузить их из `token` и `ca.crt` соответственно внутри
в папке монтирования по умолчанию `/var/run/secrets/kubernetes.io/serviceaccount/`.

```bash
d8 stronghold write auth/kubernetes/config \
    kubernetes_host=https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT
```

#### Используйте JWT клиента Stronghold в качестве JWT рецензента.

При настройке Kubernetes auth вы можете опустить `token_reviewer_jwt`, и Stronghold
будет использовать JWT клиента Stronghold в качестве своего собственного токена при взаимодействии с
API Kubernetes `TokenReview`. Если Stronghold работает в Kubernetes, вам также необходимо
установить значение `disable_local_ca_jwt=true`.

Это означает, что Stronghold не хранит никаких JWT и позволяет использовать короткоживущие токены
везде, но добавляет некоторые операционные накладные расходы на поддержание ролей кластера
привязки кластерной роли к набору учетных записей служб, которые вы хотите иметь возможность аутентифицировать в
Stronghold. Каждому клиенту Stronghold потребуется кластерная роль `system:auth-delegator:

```bash
kubectl create clusterrolebinding myapp-client-auth-delegator \
  --clusterrole=system:auth-delegator \
  --group=group1 \
  --serviceaccount=default:svcaccount1 \
  ...
```

#### Использование долгоживущих токенов.

Вы можете создать долгоживущий токен, используя инструкции [здесь][k8s-create-secret]
и использовать его в качестве `token_reviewer_jwt`. В этом примере для службы `myapp`
потребуется кластерная роль `system:auth-delegator:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: myapp-k8s-auth-secret
  annotations:
    kubernetes.io/service-account.name: myapp
type: kubernetes.io/service-account-token
EOF
```

Использование этого способа позволяет упростить настройку, но не
позволит воспользоваться преимуществами улучшенной безопасности короткоживущих токенов.

[k8s-create-secret]: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#manually-create-a-service-account-api-token

#### Использование JWT-аутентификации

Kubernetes auth использует API `TokenReview` от Kubernetes. Однако
JWT-токены, генерируемые Kubernetes, также могут быть проверены с помощью Kubernetes в качестве OIDC
провайдера. В документации по методу JWT auth есть [инструкции][k8s-jwt-auth]
по настройке JWT-auth с использованием Kubernetes в качестве OIDC-провайдера.

[k8s-jwt-auth]: /docs/auth/jwt/oidc-providers/kubernetes

Это решение позволяет использовать короткоживущие токены для всех клиентов и убирает
необходимость в рецензируемом JWT. Однако клиентские токены не могут быть отозваны до того, как
истечения их TTL, поэтому рекомендуется держать TTL коротким с учетом этого
ограничение.

## Конфигурирование kubernetes

Этот метод авторизации обращается к [Kubernetes TokenReview API][k8s-tokenreview], для
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
