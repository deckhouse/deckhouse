---
title: "Сервисный аккаунт"
menuTitle: Сервисный аккаунт
force_searchable: true
description: Сервисный аккаунт
permalink: ru/code/documentation/admin/service-account.html
lang: ru
weight: 50
---

Сервисный аккаунт — это учетная запись, предназначенная для использования в автоматизированных скриптах. Такие аккаунты применяются в CI/CD-пайплайнах и интеграциях. Сервисный аккаунт нельзя использовать для аутентификации в веб-интерфейсе, а также для выполнения действий от его имени через имперсонацию.

## Создание сервисного аккаунта

### Rails-консоль

Для создания сервисного аккаунта используется Rails-консоль из набора служебных инструментов [Toolbox](/modules/code/stable/maintenance.html#toolbox).
Откройте консоль, выполнив следующую команду:

```shell
gitlab-rails console -e production
```

### Создание аккаунта

1. Используя Rails-консоль, подготовьте параметры с описанием создаваемого аккаунта.
   Заполните поля `name`, `username`, `email` и `admin` и задайте остальные параметры на основе следующего примера:

   ```ruby
   user_args = {
   name: 'kaiten_sa',
   username: 'kaiten_sa',
   email: 'kaiten_sa@flant.com',
   admin: false,
   user_type: :service_account,
   organization_id: Organizations::Organization.default_organization.id,
   password_automatically_set: true,
   force_random_password: true,
   skip_confirmation: true
   }
   ```

1. Выберите пользователя, от имени которого будет создан сервисный аккаунт, после чего выполните создание пользователя:

   ```ruby
   user = User.find_by_username('root')
   Users::CreateService.new(user, user_args).execute
   ```

## Генерация токена доступа

Чтобы сгенерировать токен доступа, используйте [Personal access tokens API](https://docs.gitlab.com/api/personal_access_tokens/) от GitLab.
