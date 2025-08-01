---
title: "Service account"
menuTitle: Service account
force_searchable: true
description: Service account
permalink: en/code/documentation/admin/service-account.html
lang: en
weight: 50
---

A service account is a type of account used not by humans, but in automation scripts. It can be used in pipelines and integrations. It's not possible to authenticate via the web interface using a service account or to impersonate it.

## Create service account

### Open rails console

You can access the console using the [toolbox](https://deckhouse.ru/products/kubernetes-platform/modules/code/stable/maintenance.html#toolbox).

### Create account

Required fields are name, username, email, admin

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

Select the user on whose behalf the service account will be created, then create the service account.

```ruby
user = User.find_by_username('root')
Users::CreateService.new(user, user_args).execute
```

## Personall access token

For create personall access token you can use [API](https://docs.gitlab.com/api/personal_access_tokens/)
