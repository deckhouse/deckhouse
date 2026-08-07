---
title: "Локальная аутентификация"
permalink: ru/admin/configuration/access/authentication/local.html
description: "Настройка локальной аутентификации для платформы Deckhouse Kubernetes Platform с парольными политиками, поддержкой 2FA и управлением группами. Реализация безопасности, соответствующая требованиям OWASP."
lang: ru
---

Помимо внешних провайдеров аутентификации, DKP позволяет использовать локальную аутентификацию.

Локальная аутентификация обеспечивает проверку и управление доступом пользователей с возможностью настройки парольной политики, поддержкой двухфакторной аутентификации (2FA) и управлением группами.
Реализация соответствует требованиям безопасности ФСТЭК и рекомендациям OWASP, обеспечивая надёжную защиту доступа к кластеру и приложениям без необходимости интеграции с внешними системами аутентификации.

Локальная аутентификация подразумевает создание в кластере объектов User и Group для статических пользователей и групп:

- В [объекте User](/modules/user-authn/cr.html#user) хранится информация о пользователе, включая email и хеш пароля (пароль в явном виде не сохраняется).
- В [объекте Group](/modules/user-authn/cr.html#group) задаётся список пользователей, объединённых в группу.

## Создание статического пользователя

Рекомендуемый способ создания локального пользователя — команда [`d8 iam user create`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-user-create). Она поддерживает интерактивный ввод пароля, автоматическую генерацию пароля, добавление в группы и TTL для временных пользователей:

```shell
# Интерактивный ввод пароля (по умолчанию если stdin — терминал)
d8 iam user create anton --email anton@abc.com

# Автогенерация пароля (показывается один раз)
d8 iam user create anton --email anton@abc.com --generate-password

# Пароль из stdin (для CI/CD пайплайнов)
echo "s3cret" | d8 iam user create anton --email anton@abc.com --password-stdin

# Создать пользователя и добавить в группы (с автосозданием групп)
d8 iam user create anton --email anton@abc.com --generate-password --member-of admins --create-groups

# Создать временного пользователя с TTL
d8 iam user create anton --email anton@abc.com --generate-password --ttl 24h
```

В качестве альтернативы можно создать ресурс [User](/modules/user-authn/cr.html#user) вручную.

Пример создания ресурса (обратите внимание, что в приведенном примере указан [ttl](/modules/user-authn/cr.html#user-v1-spec-ttl)):

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

Придумайте пароль и укажите его хеш-сумму в поле `password`. Пароль хранится в зашифрованном виде (bcrypt).
Хеш-сумму можно сгенерировать с помощью команды:

```shell
echo "$password" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
```

{% alert level="info" %}
Если команда `htpasswd` недоступна, установите соответствующий пакет в зависимости от дистрибутива:

* `apache2-utils` — для дистрибутивов, основанных на Debian;
* `httpd-tools` — для дистрибутивов, основанных на CentOS;
* `apache2-htpasswd` — для ALT Linux.
{% endalert %}

## Удаление пользователя

Для удаления локального пользователя используйте команду [`d8 iam user delete`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-user-delete). По умолчанию команда также удаляет пользователя из всех ресурсов [Group](/modules/user-authn/cr.html#group), в которых он состоит:

```shell
# Удалить пользователя (+ автоматически удалить из всех групп)
d8 iam user delete anton

# Удалить пользователя, оставив ссылки в группах
d8 iam user delete anton --keep-memberships
```

## Операции над локальным пользователем

Операции сброса пароля, сброса 2FA и блокировки пользователя выполняются через ресурс [UserOperation](/modules/user-authn/cr.html#useroperation). В поле `initiatorType` указывается, кто инициировал операцию: администратор (`admin`), система (`system`) или сам пользователь (`self`).

### Административные операции

Для административных действий над локальными пользователями используйте команды [`d8 iam user`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam). Они создают ресурс UserOperation с `initiatorType: admin`, дожидаются выполнения операции и выводят результат.

При выполнении операций `ResetPassword`, `Reset2FA` и `Lock` удаляются объекты Dex OfflineSessions и RefreshToken, принадлежащие пользователю. Это завершает активные offline-сессии пользователя и требует повторной аутентификации.

Примеры использования команд [`d8 iam user`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam):

- интерактивный сброс пароля:

  ```shell
  d8 iam user reset-password admin
  ```

- сброс пароля с чтением нового из stdin:

  ```shell
  echo "N3wPa$$wo#d" | d8 iam user reset-password admin --password-stdin
  ```

- сброс пароля с автоматической генерацией нового:

  ```shell
  d8 iam user reset-password admin --generate-password
  ```

- сброс пароля в захешированном виде (если пароль захеширован, передайте bcrypt-хеш без кодирования в Base64):

  ```shell
  d8 iam user reset-password admin --password-hash '$2y$10$abcdef...'
  ```

- сброс 2FA:

  ```shell
  d8 iam user reset2fa admin
  ```

- блокировка пользователя на 30 минут:

  ```shell
  d8 iam user lock admin 30m
  ```

- разблокировка пользователя:

  ```shell
  d8 iam user unlock admin
  ```

По умолчанию команды ожидают завершения операции. Чтобы только создать UserOperation и вывести его имя, используйте флаг `--wait=false`.

### Сброс пароля пользователем

Локальный пользователь может самостоятельно сбросить свой пароль в интерфейсе аутентификации DKP. При этом создаётся ресурс UserOperation с типом `ResetPassword` и `initiatorType: self`.

Самостоятельный сброс пароля доступен только для локальных учётных записей (встроенный коннектор `Local`). Пользователи, которые входят через внешние провайдеры аутентификации, должны обращаться к администратору соответствующей системы.

При сбросе пароля:

- новый пароль должен соответствовать [парольной политике](#настройка-парольной-политики);
- завершаются активные сессии пользователя — требуется повторная аутентификация.

Подробнее о сценариях смены и сброса пароля с точки зрения пользователя — в разделе [Настройка аутентификации для приложений](../../../../user/access/authentication.html#смена-и-сброс-пароля-локального-пользователя).

### Ручное создание UserOperation

Когда CLI `d8 iam user` недоступен (например, в CI/CD, GitOps или скриптах автоматизации), ресурс [UserOperation](/modules/user-authn/cr.html#useroperation) можно создать напрямую. Пример — сброс пароля локального пользователя (в `newPasswordHash` указывается bcrypt-хеш без кодирования в Base64; хук кодирует его автоматически):

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: reset-password-admin
spec:
  user: admin
  type: ResetPassword
  initiatorType: admin
  resetPassword:
    newPasswordHash: "$2y$10$..."
```

Для пользователей, аутентифицируемых через внешние провайдеры (LDAP, Atlassian Crowd), вместо `spec.user` используется `spec.target`. Для внешних пользователей поддерживаются только операции `Lock` и `Unlock`. Пример — блокировка внешнего пользователя на 30 минут:

```yaml
apiVersion: deckhouse.io/v1
kind: UserOperation
metadata:
  name: lock-external-user
spec:
  target:
    connectorID: my-ldap
    email: jane.doe@example.org
  type: Lock
  initiatorType: admin
  lock:
    for: "30m"
```

UserOperation — **одноразовый** и **неизменяемый** объект: после создания хук обрабатывает его и записывает результат в `status.phase` (`Succeeded` или `Failed`). Завершённые операции **автоматически удаляются** через 24 часа.

{% alert level="warning" %}
Операции `ResetPassword`, `Reset2FA` и `Lock` завершают все активные сессии пользователя (удаляют объекты Dex OfflineSessions и RefreshToken). Пользователь будет вынужден пройти повторную аутентификацию.
{% endalert %}

## Добавление пользователя в группу

{% alert level="warning" %}
Запрещено использовать пользователей и группы с префиксом `system:`.  
Аутентификация таких пользователей или участников этих групп будет отклонена, а в логах `kube-apiserver` появится соответствующее предупреждение.
{% endalert %}

Рекомендуемый способ управления группами — команда [`d8 iam group`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-group):

```shell
# Создать группу
d8 iam group create admins

# Добавить пользователя в группу
d8 iam group add-member admins user anton

# Удалить участника из группы
d8 iam group remove-member admins user anton

# Удалить группу
d8 iam group delete admins
```

В качестве альтернативы можно создать [ресурс Group](/modules/user-authn/cr.html#group) вручную.

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

Здесь `members` — список пользователей, которые входят в группу.

После создания группы и добавления в неё пользователей необходимо настроить [авторизацию](../authorization/).

## Настройка парольной политики

Парольная политика позволяет контролировать сложность пароля, ротацию и блокировку пользователей.

Для настройки парольной политики используйте поле [`passwordPolicy`](/modules/user-authn/configuration.html#parameters-passwordpolicy) в конфигурации модуля `user-authn`.

Примеры политик:

{% tabs Примеры парольных политик%}
{% tab "Без пользовательских правил сложности" %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    passwordPolicy:
      complexityLevel: Fair
      passwordHistoryLimit: 10
      lockout:
        lockDuration: 15m
        maxAttempts: 3
      rotation:
        interval: "30d"
```

{% endtab %}
{% tab "С пользовательскими правилами сложности" %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    passwordPolicy:
      complexityLevel: Custom
      custom:
        minLength: 10
        specialCharacters: true
        numbers: false
        capitalized: true
        repeatedChars: false
      passwordHistoryLimit: 10
```

{% endtab %}
{% endtabs %}

## Настройка двухфакторной аутентификации (2FA)

2FA позволяет повысить уровень безопасности, требуя ввести код из приложения-аутентификатора TOTP (например, Google Authenticator) при входе.

Для настройки 2FA используйте поле [`staticUsers2FA`](/modules/user-authn/configuration.html#parameters-staticusers2fa) в конфигурации модуля `user-authn`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    staticUsers2FA:
      enabled: true
      issuerName: "awesome-app"
```

Описание полей:

- `enabled` — включает или отключает 2FA для всех статических пользователей;
- `issuerName` — имя, которое будет отображаться в приложении-аутентификаторе при добавлении аккаунта.

{% alert level="info" %}
После включения 2FA каждый пользователь должен пройти процесс регистрации в приложении-аутентификаторе при первом входе.
{% endalert %}
