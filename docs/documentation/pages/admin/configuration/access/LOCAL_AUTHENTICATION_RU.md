---
title: "Локальная аутентификация"
permalink: ru/admin/access/local-authentication.html
lang: ru
---

Помимо внешних провайдеров аутентификации, DKP позволяет использовать локальную аутентификацию.

Локальная аутентификация подразумевает создание в кластере объектов User и Group для статических пользователей и групп:

- В объекте User хранится информация о пользователе, включая email и хеш пароля (пароль в явном виде не сохраняется).
- В объекте Group задаётся список пользователей, объединённых в группу.

### Создание статического пользователя

   Для создания статического пользователя создайте ресурс User.

   Пример создания ресурса (обратите внимание, что в приведенном примере указан [ttl](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/user-authn/cr.html#user-v1-spec-ttl)):

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

   ```console
   echo "$password" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
   ```

### Добавление пользователя в группу

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

   После создания группы и добавления в неё пользователей, необходимо настроить [авторизацию](../access/authorization-overview.html).
