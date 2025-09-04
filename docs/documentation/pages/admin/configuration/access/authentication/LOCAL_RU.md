---
title: "Локальная аутентификация"
permalink: ru/admin/configuration/access/authentication/local.html
lang: ru
---

Помимо внешних провайдеров аутентификации, DKP позволяет использовать локальную аутентификацию.

Локальная аутентификация подразумевает создание в кластере объектов User и Group для статических пользователей и групп:

- В объекте User хранится информация о пользователе, включая email и хеш пароля (пароль в явном виде не сохраняется).
- В объекте Group задаётся список пользователей, объединённых в группу.

## Создание статического пользователя

Для создания статического пользователя создайте ресурс User.

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

Придумайте пароль и укажите его хэш-сумму в поле `password`. Пароль хранится в зашифрованном виде (bcrypt).
Хэш-сумму можно сгенерировать с помощью команды:

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

Чтобы объединять статических пользователей в группы, создайте ресурс Group.

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

`Members` — список пользователей, которые входят в группу (указывается `kind`: User и имя пользователя).

После создания группы и добавления в неё пользователей, необходимо настроить [авторизацию](../../access/authorization/).
