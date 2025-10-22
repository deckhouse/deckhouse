---
title: "Service account"
menuTitle: Service account
force_searchable: true
description: Service account
permalink: en/code/documentation/admin/service-account.html
lang: en
weight: 50
---

A service account is a user account intended for use in automated scripts. These accounts are typically used in CI/CD pipelines and integrations. A service account cannot be used to authenticate via the web interface or to perform actions through impersonation.

## Creating a service account

### Rails console

To create a service account, use the Rails console provided in the [Toolbox](/modules/code/stable/maintenance.html#toolbox) utility set.
Open the console by running the following command:

```shell
gitlab-rails console -e production
```

### Creating an account

1. In the Rails console, prepare the parameters defining the account to be created.
   Fill in the `name`, `username`, `email`, and `admin` fields,
   and define the rest of the parameters as shown in the example below:

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

1. Select the user on whose behalf the service account will be created and execute the account creation:

   ```ruby
   user = User.find_by_username('root')
   Users::CreateService.new(user, user_args).execute
   ```

## Generating an access token

To generate an access token, use GitLab's [Personal access tokens API](https://docs.gitlab.com/api/personal_access_tokens/).
