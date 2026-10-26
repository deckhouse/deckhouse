---
title: "Локальная аутентификация"
permalink: ru/admin/configuration/access/authentication/local.html
description: "Настройка локальной аутентификации для Deckhouse Platform с парольными политиками, поддержкой 2FA и управлением группами."
lang: ru
---

Помимо внешних провайдеров аутентификации, DP позволяет использовать локальную аутентификацию.

Локальная аутентификация обеспечивает проверку и управление доступом пользователей с возможностью настройки парольной политики, поддержкой двухфакторной аутентификации (2FA) и управлением группами.
В параметрах модуля можно настроить сложность паролей, историю, ротацию, блокировку после неудачных попыток входа и ограничение частоты неудачных попыток с одного IP-адреса. Внешняя система аутентификации не требуется.

Локальная аутентификация подразумевает создание в кластере объектов User и Group для статических пользователей и групп:

- В [объекте User](/modules/user-authn/cr.html#user) хранится информация о пользователе, включая email и хеш пароля (пароль в явном виде не сохраняется).
- В [объекте Group](/modules/user-authn/cr.html#group) задаётся список пользователей, объединённых в группу.

## Создание статического пользователя

Рекомендуемый способ создания локального пользователя — команда [`d8 iam user create`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-user-create). Она поддерживает интерактивный ввод пароля, автоматическую генерацию пароля, добавление в группы и TTL для временных пользователей.

Примеры:

Интерактивный ввод пароля (по умолчанию если stdin — терминал):

```shell
d8 iam user create anton --email anton@abc.com
```

Автогенерация пароля (сгенерированный пароль показывается в выводе команды один раз):

```shell
d8 iam user create anton --email anton@abc.com --generate-password
```

Пароль из stdin (для CI/CD пайплайнов):

```shell
echo "s3cret" | d8 iam user create anton --email anton@abc.com --password-stdin
```

Создать пользователя и добавить в группы (с автосозданием групп):

```shell
d8 iam user create anton --email anton@abc.com --generate-password --member-of admins --create-groups
```

Создать временного пользователя с TTL:

```shell
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

Придумайте пароль и укажите в поле `password` его bcrypt-хеш в исходном виде или закодированном в Base64.
Чтобы получить bcrypt-хеш в Base64, выполните команду:

```shell
echo -n '<PASSWORD>' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
```

{% alert level="info" %}
Если команда `htpasswd` недоступна, установите соответствующий пакет в зависимости от дистрибутива:

* `apache2-utils` — для дистрибутивов, основанных на Debian;
* `httpd-tools` — для дистрибутивов, основанных на CentOS;
* `apache2-htpasswd` — для ALT Linux.
{% endalert %}

{% alert level="warning" %}
Нельзя создать User, у которого `spec.email` совпадает с субъектом типа `User` существующего ClusterAuthorizationRule или AuthorizationRule: новый пользователь незаметно получил бы права этого правила. Если совпадение намеренное, добавьте на User аннотацию `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"`. Подробнее — в разделе [«Как применяются правила»](../authorization/rbac-current.html#применение-правил).
{% endalert %}

## Удаление пользователя

Для удаления локального пользователя используйте команду [`d8 iam user delete`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-user-delete). По умолчанию команда также удаляет пользователя из всех ресурсов [Group](/modules/user-authn/cr.html#group), в которых он состоит.

Примеры:

Удалить пользователя (плюс автоматически удалить из всех групп):

```shell
d8 iam user delete anton
```

Удалить пользователя, оставив ссылки в группах:

```shell
d8 iam user delete anton --keep-memberships
```

## Операции над локальным пользователем

Операции сброса пароля, принудительной смены пароля, сброса 2FA, блокировки, разблокировки и выхода пользователя выполняются через ресурс [UserOperation](/modules/user-authn/cr.html#useroperation). В поле `initiatorType` указывается, кто инициировал операцию: администратор (`admin`), система (`system`) или сам пользователь (`self`).

### Административные операции

Для административных действий над локальными пользователями используйте команды [`d8 iam user`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam). Они создают ресурс UserOperation с `initiatorType: admin`, дожидаются выполнения операции и выводят результат.

{% alert level="info" %}
Удалить, пересоздать или выполнить над пользователем операцию `ResetPassword`, `Reset2FA`, `Lock` или `Unlock` можно, только если у вас уже есть права, покрывающие роли, которые пользователь получает через email или членство в группах, либо вам явно разрешено назначать эти роли. Значение поля `initiatorType` на эту проверку не влияет: `initiatorType: self` её не обходит. Подробнее — в [документации модуля `user-authn`](/modules/user-authn/usage.html#административные-операции). То же правило действует для объектов DexProvider, см. [«Ограничение идентичностей провайдера»](external-authentication-providers.html#ограничение-идентичностей-провайдера).
{% endalert %}

Операции `ResetPassword`, `Reset2FA`, `Lock` и `Logout` сразу завершают сессии пользователя, и пользователю нужно пройти повторную аутентификацию. Что именно завершается и как быстро, описано [в разделе «Жизненный цикл и побочные эффекты UserOperation»](/modules/user-authn/usage.html#жизненный-цикл-и-побочные-эффекты-useroperation). Секрет 2FA пользователя удаляет только `Reset2FA`.

После сброса пароля администратором операцией `ResetPassword` пользователь один раз входит с этим паролем. Затем Dex просит пользователя задать новый пароль, соответствующий парольной политике.

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

  Команда `d8 iam user lock` блокирует локального пользователя и принимает значение длительности в секундах, минутах или часах, например `30s`, `10m` или `1h`. Чтобы заблокировать пользователя на несколько дней (`lock.for: 7d`) или бессрочно (`lock.for: permanent`), создайте UserOperation, как описано [в разделе «Ручное создание UserOperation»](#ручное-создание-useroperation).

- разблокировка пользователя:

  ```shell
  d8 iam user unlock admin
  ```

По умолчанию команды ожидают завершения операции. Чтобы только создать UserOperation и вывести его имя, используйте флаг `--wait=false`.

### Операции, которые выполняет сам пользователь

В веб-интерфейсе Deckhouse пользователь может выйти на всех устройствах, а локальный пользователь — сменить пароль. Веб-интерфейс создаёт UserOperation с `initiatorType: self` для пользователя, который вошёл в систему:

- При выходе на всех устройствах создаётся операция `Logout`. Она завершает все сессии пользователя, включая файлы kubeconfig.
- При смене пароля создаётся операция `RequirePasswordChange`. При следующем входе Dex запрашивает текущий пароль и новый, соответствующий [парольной политике](#настройка-парольной-политики). Операция не завершает сессии.

Смена пароля доступна только для локальных учётных записей (встроенный коннектор `Local`). Пользователи, которые входят через внешние провайдеры аутентификации, меняют пароль в соответствующей системе.

Подробнее о сценариях смены и сброса пароля с точки зрения пользователя — в разделе [Настройка аутентификации для приложений](../../../../user/access/authentication.html#смена-и-сброс-пароля-локального-пользователя).

### Ручное создание UserOperation

Когда выполнение команды `d8 iam user` в CLI недоступно (например, в CI/CD, GitOps или скриптах автоматизации), ресурс [UserOperation](/modules/user-authn/cr.html#useroperation) можно создать напрямую. Пример — сброс пароля локального пользователя (в `newPasswordHash` указывается bcrypt-хеш без кодирования в Base64; модуль `user-authn` кодирует его автоматически):

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

Для пользователей внешних провайдеров любого типа (LDAP, Atlassian Crowd, OIDC, SAML, GitHub, GitLab, Bitbucket Cloud) вместо `spec.user` используется [`spec.target`](/modules/user-authn/cr.html#useroperation-v1-spec-target). Для внешних пользователей поддерживаются операции `Lock`, `Unlock` и `Logout`. Пример — блокировка внешнего пользователя на 30 минут:

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

Блокировка действует только на вход через провайдер, указанный в `connectorID`. Локальный пользователь или пользователь другого провайдера с тем же email не блокируется. Операция завершается ошибкой, если пользователь ещё ни разу не входил через этот провайдер.

UserOperation — **одноразовый** объект: после создания модуль обрабатывает его и записывает результат в `status.phase` (`Succeeded` или `Failed`). UserOperation выполняется один раз, и изменения его спецификации после создания игнорируются. Завершённые операции **автоматически удаляются** через 24 часа.

{% alert level="warning" %}
Операции `ResetPassword`, `Reset2FA`, `Lock` и `Logout` сразу завершают сессии пользователя, и пользователю нужно пройти повторную аутентификацию. Что именно завершается и как быстро, описано [в разделе «Жизненный цикл и побочные эффекты UserOperation»](/modules/user-authn/usage.html#жизненный-цикл-и-побочные-эффекты-useroperation). Операции `ResetPassword`, `Lock` и `Logout` сохраняют секрет 2FA пользователя.
{% endalert %}

### Просмотр состояния учётной записи

Для каждого локального пользователя и каждого пользователя, входившего через внешний провайдер, модуль `user-authn` ведёт объект [UserAccount](/modules/user-authn/cr.html#useraccount). В нём видно, заблокирована ли учётная запись, до какого времени и сколько неудачных попыток входа учтено. Не создавайте и не изменяйте объекты UserAccount.

Чтобы посмотреть учётные записи, выполните:

```shell
d8 k get useraccounts
```

## Добавление пользователя в группу

{% alert level="warning" %}
Запрещено использовать пользователей и группы с префиксом `system:`.  
Аутентификация таких пользователей или участников этих групп будет отклонена, а в логах `kube-apiserver` появится соответствующее предупреждение.
{% endalert %}

{% alert level="warning" %}
Нельзя создать Group, у которой `spec.name` совпадает с субъектом типа `Group` существующего ClusterAuthorizationRule или AuthorizationRule: участники новой группы незаметно получили бы права этого правила. Если совпадение намеренное, добавьте на Group аннотацию `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"`. Подробнее — в разделе [«Как применяются правила»](../authorization/rbac-current.html#применение-правил).
{% endalert %}

Рекомендуемый способ управления группами — команда [`d8 iam group`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-group). Примеры:

Создать группу:

```shell
d8 iam group create admins
```

Добавить пользователя в группу:

```shell
d8 iam group add-member admins user anton
```

Удалить пользователя из группы:

```shell
d8 iam group remove-member admins user anton
```

Удалить группу:

```shell
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

Парольная политика позволяет контролировать сложность пароля, ротацию и автоматическую блокировку после нескольких неуспешных попыток входа.

Администратор может заблокировать или разблокировать пользователя командами [`d8 iam user lock` / `unlock`](../../../../cli/d8/reference/#d8-iam) (или через UserOperation) без включения `passwordPolicy.lockout`. Секция `lockout` включает только автоматическую блокировку после заданного числа неудачных попыток.

Dex хранит сведения об автоматической блокировке самостоятельно и не создаёт для неё UserOperation. Локальные пользователи блокируются всегда. Пользователи LDAP и Atlassian Crowd блокируются, только если тип их провайдера указан в [`passwordPolicy.lockout.applyToConnectors`](/modules/user-authn/configuration.html#parameters-passwordpolicy-lockout-applytoconnectors). Чтобы ограничить число неудачных попыток входа с одного IP-адреса, используйте параметр [`rateLimit`](/modules/user-authn/configuration.html#parameters-ratelimit).

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
  version: 3
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
  version: 3
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
  version: 3
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
