---
title: "Модуль documentation: примеры конфигурации"
description: "Пример конфигурации модуля documentation, аутентификация, хранение подготовленной документации версий модулей в своём хранилище S3 и режим отказоустойчивости."
---

## Пример конфигурации модуля

Модуль работает на системных узлах, аутентифицирует пользователей через внешний сервис аутентификации и пускает пользователей группы `admins`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: documentation
spec:
  version: 1
  enabled: true
  settings:
    nodeSelector:
      node-role/system: ""
    tolerations:
    - key: dedicated.deckhouse.io
      operator: Equal
      value: system
    auth:
      externalAuthentication:
        authURL: "https://<applicationDomain>/auth"
        authSignInURL: "https://<applicationDomain>/sign-in"
      allowedUserGroups:
      - admins
```

## Аутентификация

По умолчанию пользователей аутентифицирует модуль [user-authn](../user-authn/). Внешний сервис аутентификации задайте в параметре [auth.externalAuthentication](configuration.html#parameters-auth-externalauthentication).
Если модуль `user-authn` выключен и параметр `auth.externalAuthentication` не задан, модуль включает базовую аутентификацию с пользователем `admin` и сгенерированным паролем.

Чтобы посмотреть сгенерированный пароль, выполните команду:

```shell
d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values documentation -o json | jq '.internal.auth.password'
```

Чтобы сгенерировать новый пароль, удалите Secret:

```shell
d8 k -n d8-system delete secret/documentation-basic-auth
```

## Своё хранилище S3

С `moduleStorage.s3.type: External` модуль хранит подготовленную документацию версий модулей в вашем бакете S3. Создайте Secret в любом неймспейсе с такими ключами. Deckhouse Platform копирует его в Secret `documentation-s3-connection` в `d8-system`. После того как вы создадите или измените Secret, под документации перезапустится в течение 5 минут. Храните в Secret только ключи подключения: Deckhouse Platform копирует в `d8-system` все ключи Secret.

| Ключ | Обязательный | Значение |
|---|---|---|
| `S3_ENDPOINT` | да | Адрес хранилища, только `https://`. |
| `S3_BUCKET` | да | Имя бакета. |
| `S3_REGION` | да | Регион бакета, например `us-east-1`. |
| `AWS_ACCESS_KEY_ID` | да | Ключ доступа. |
| `AWS_SECRET_ACCESS_KEY` | да | Секретный ключ. |
| `S3_CA_CERT` | нет | Сертификат CA хранилища в формате PEM, если системные сертификаты ему не доверяют. |

Ключу нужны права читать, записывать, перечислять и удалять объекты под префиксом из [moduleStorage.s3.external.prefix](configuration.html#parameters-modulestorage-s3-external-prefix) (по умолчанию `documentation/`). Объекты вне префикса модуль не читает, не записывает и не удаляет, поэтому в S3-бакете могут лежать и другие данные.

Пример Secret и конфигурации модуля, которая его использует:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: documentation-s3
  namespace: documentation-storage
stringData:
  S3_ENDPOINT: https://s3.example.com
  S3_BUCKET: documentation
  S3_REGION: us-east-1
  AWS_ACCESS_KEY_ID: <ключ доступа>
  AWS_SECRET_ACCESS_KEY: <секретный ключ>
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: documentation
spec:
  version: 1
  settings:
    moduleStorage:
      s3:
        type: External
        external:
          secretRef:
            namespace: documentation-storage
            name: documentation-s3
```

Пока Secret не создан или хранилище не отвечает, версии хранятся только в поде. После перезапуска пода модуль готовит их заново. Метрика `docs_builder_ondemand_s3_ready` пода документации в это время равна 0. Ошибку в Secret (нет ключа, ключ назван иначе, адрес `http://`) модуль записывает в журнал контейнера `builder` пода документации. Если хранилище не готово в течение 15 минут, срабатывает алерт `D8DocumentationS3NotReady`.

## Режим отказоустойчивости

Параметр [highAvailability](configuration.html#parameters-highavailability) включает режим отказоустойчивости. Если параметр не задан, действует режим отказоустойчивости кластера. В этом режиме документация работает в двух репликах на разных узлах. Поэтому нужны два узла, на которых модуль может работать (по умолчанию — системные). Если такой узел один, вторая реплика остаётся в статусе `Pending`. PodDisruptionBudget блокирует освобождение узла (drain) с работающей репликой, пока вторая реплика не запустится на другом узле. Аутентификатор документации (DexAuthenticator), если он развёрнут, тоже работает в двух репликах.

Пользователь остаётся на одной реплике: её выбирает cookie `d8-documentation-affinity`. Через Ingress cookie живёт 24 часа, в том числе после того как пользователь закрыл браузер. Через Gateway API cookie живёт, пока пользователь не закроет браузер. Версии модулей, которые открывает пользователь, готовит и хранит эта реплика.

Включите хранение версий в S3 ([moduleStorage.s3.type](configuration.html#parameters-modulestorage-s3-type): `System` или `External`). Тогда версию, которую подготовила одна реплика, другая реплика открывает из S3 без повторной подготовки. Пользователь, который перешёл на другую реплику при перезапуске своей, не ждёт подготовки заново. Без S3 каждая реплика сама обращается в хранилище образов контейнеров. Она готовит версии для поиска и версии, которые открывают её пользователи. В этом случае документация расходует вдвое больше CPU и отправляет в хранилище вдвое больше запросов. Пользователь, который перешёл на другую реплику, ещё раз ждёт подготовки уже открытой версии.

С S3 одна реплика, выбранная через Lease, читает хранилище образов контейнеров и готовит версии для поиска. Другая реплика в течение минуты берёт эти версии из S3 и сама готовит только версии, которые открывают её пользователи. Реплика начинает принимать пользователей, когда получила версии из S3, но не позже чем через 5 минут после запуска.

После обновления Deckhouse Platform, которое меняет вид документации, версии из S3 открываются в прежнем виде. Модуль готовит их заново в фоне, по одной, пока никто не ждёт подготовки версии. Если подготовить версию в новом виде не удалось, она остаётся прежней.
