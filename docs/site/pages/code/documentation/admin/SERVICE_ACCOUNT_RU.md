---
title: "Сервисный аккаунт"
menuTitle: Сервисный аккаунт
force_searchable: true
description: Сервисный аккаунт
permalink: ru/code/documentation/admin/service-account.html
lang: ru
weight: 50
---

Сервисный аккаунт — это учетная запись, предназначенная не для людей, а для использования в автоматизированных скриптах. Такие аккаунты применяются в пайплайнах и интеграциях. Аутентификация через веб-интерфейс от имени сервисного аккаунта невозможна, как и его имперсонация.

## Создание сервисного аккаунта

### Открыть rails console

Получить доступ к консоли можно через [toolbox](https://deckhouse.ru/products/kubernetes-platform/modules/code/stable/maintenance.html#toolbox)

### Создать аккаунт

Нужно указать имя, юзернейм, емейл, является ли пользователем администратором

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

Выбрать пользователя от имени которого будет создан сервисный аккаунт. И создать пользователя.

```ruby
user = User.find_by_username('root')
Users::CreateService.new(user, user_args).execute
```

## Токен доступа

Для получения токена можно воспользоваться [АПИ](https://docs.gitlab.com/api/personal_access_tokens/)
