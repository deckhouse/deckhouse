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

Для создания статического пользователя создайте ресурс [User](/modules/user-authn/cr.html#user).

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
Если команда `htpasswd` не найдена установите пакет `apache2-utils` для Debian-основанных дистрибутивов и `httpd-utils` для CentOS-основанных дистрибутивов.
Если команда `htpasswd` недоступна, установите соответствующий пакет:

* `apache2-utils` — для дистрибутивов, основанных на Debian;
* `httpd-tools` — для дистрибутивов, основанных на CentOS;
* `apache2-htpasswd` — для ALT Linux.
{% endalert %}

## Добавление пользователя в группу

{% alert level="warning" %}
Запрещено использовать пользователей и группы с префиксом `system:`.  
Аутентификация таких пользователей или участников этих групп будет отклонена, а в логах `kube-apiserver` появится соответствующее предупреждение.
{% endalert %}

Чтобы объединять статических пользователей в группы, создайте [ресурс Group](/modules/user-authn/cr.html#group).

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

После создания группы и добавления в неё пользователей, необходимо настроить [авторизацию](../../access/authorization/).

## Настройка парольной политики

Парольная политика позволяет контролировать сложность пароля, ротацию и блокировку пользователей.

Для настройки парольной политики используйте поле [`passwordPolicy`](/modules/user-authn/configuration.html#parameters-passwordpolicy) в конфигурации модуля `user-authn`:

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

Описание полей:

- `complexityLevel` — уровень сложности пароля;
- `passwordHistoryLimit` — число предыдущих паролей, которые хранит система, чтобы предотвратить их повторное использование;
- `lockout` — настройки блокировки при превышении лимита неудачных попыток входа:
  - `lockout.maxAttempts` — лимит неудачных попыток;
  - `lockout.lockDuration` — длительность блокировки пользователя;
- `rotation` — настройки ротации паролей:
  - `rotation.interval` — период обязательной смены пароля.

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
