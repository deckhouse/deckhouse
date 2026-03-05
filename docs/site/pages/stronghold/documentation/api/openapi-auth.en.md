---
title: "API - Auth"
permalink: en/stronghold/documentation/api/openapi-auth.html
search: true
sitemap_include: false
description: API reference - AUTH
lang: en
---

{% raw %}

## auth


### GET /auth/token/accessors

**Operation ID:** `token-list-accessors`


List token accessors, which can then be
be used to iterate and discover their properties
or revoke them. Because this can be used to
cause a denial of service, this endpoint
requires 'sudo' capability in addition to
'list'.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### POST /auth/token/create

**Operation ID:** `token-create`


The token create path is used to create new tokens.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `display_name` | string | no | Name to associate with this token |
| `entity_alias` | string | no | Name of the entity alias to associate with this token |
| `explicit_max_ttl` | string | no | Explicit Max TTL of this token |
| `id` | string | no | Value for the token |
| `lease` | string | no | ⚠️ Deprecated. Use 'ttl' instead |
| `meta` | object | no | Arbitrary key=value metadata to associate with the token |
| `no_default_policy` | boolean | no | Do not include default policy for this token |
| `no_parent` | boolean | no | Create the token with no parent |
| `num_uses` | integer | no | Max number of uses for this token |
| `period` | string | no | Renew period |
| `policies` | array | no | List of policies for the token |
| `renewable` | boolean (default: True) | no | Allow token to be renewed past its initial TTL up to system/mount maximum TTL |
| `ttl` | string | no | Time to live for this token |
| `type` | string | no | Token type |




#### Responses


**200**: OK



### POST /auth/token/create-orphan

**Operation ID:** `token-create-orphan`


The token create path is used to create new orphan tokens.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `display_name` | string | no | Name to associate with this token |
| `entity_alias` | string | no | Name of the entity alias to associate with this token |
| `explicit_max_ttl` | string | no | Explicit Max TTL of this token |
| `id` | string | no | Value for the token |
| `lease` | string | no | ⚠️ Deprecated. Use 'ttl' instead |
| `meta` | object | no | Arbitrary key=value metadata to associate with the token |
| `no_default_policy` | boolean | no | Do not include default policy for this token |
| `no_parent` | boolean | no | Create the token with no parent |
| `num_uses` | integer | no | Max number of uses for this token |
| `period` | string | no | Renew period |
| `policies` | array | no | List of policies for the token |
| `renewable` | boolean (default: True) | no | Allow token to be renewed past its initial TTL up to system/mount maximum TTL |
| `ttl` | string | no | Time to live for this token |
| `type` | string | no | Token type |




#### Responses


**200**: OK



### POST /auth/token/create/{role_name}

**Operation ID:** `token-create-against-role`


This token create path is used to create new tokens adhering to the given role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `display_name` | string | no | Name to associate with this token |
| `entity_alias` | string | no | Name of the entity alias to associate with this token |
| `explicit_max_ttl` | string | no | Explicit Max TTL of this token |
| `id` | string | no | Value for the token |
| `lease` | string | no | ⚠️ Deprecated. Use 'ttl' instead |
| `meta` | object | no | Arbitrary key=value metadata to associate with the token |
| `no_default_policy` | boolean | no | Do not include default policy for this token |
| `no_parent` | boolean | no | Create the token with no parent |
| `num_uses` | integer | no | Max number of uses for this token |
| `period` | string | no | Renew period |
| `policies` | array | no | List of policies for the token |
| `renewable` | boolean (default: True) | no | Allow token to be renewed past its initial TTL up to system/mount maximum TTL |
| `ttl` | string | no | Time to live for this token |
| `type` | string | no | Token type |




#### Responses


**200**: OK



### GET /auth/token/lookup

**Operation ID:** `token-look-up-2`


This endpoint will lookup a token and its properties.


#### Responses


**200**: OK



### POST /auth/token/lookup

**Operation ID:** `token-look-up`


This endpoint will lookup a token and its properties.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token` | string | no | Token to lookup (POST request body) |




#### Responses


**200**: OK



### POST /auth/token/lookup-accessor

**Operation ID:** `token-look-up-accessor`


This endpoint will lookup a token associated with the given accessor and its properties. Response will not contain the token ID.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `accessor` | string | no | Accessor of the token to look up (request body) |




#### Responses


**200**: OK



### GET /auth/token/lookup-self

**Operation ID:** `token-look-up-self`


This endpoint will lookup a token and its properties.


#### Responses


**200**: OK



### POST /auth/token/lookup-self

**Operation ID:** `token-look-up-self2`


This endpoint will lookup a token and its properties.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token` | string | no | Token to look up (unused, does not need to be set) |




#### Responses


**200**: OK



### POST /auth/token/renew

**Operation ID:** `token-renew`


This endpoint will renew the given token and prevent expiration.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `increment` | integer (default: 0) | no | The desired increment in seconds to the token expiration |
| `token` | string | no | Token to renew (request body) |




#### Responses


**200**: OK



### POST /auth/token/renew-accessor

**Operation ID:** `token-renew-accessor`


This endpoint will renew a token associated with the given accessor and its properties. Response will not contain the token ID.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `accessor` | string | no | Accessor of the token to renew (request body) |
| `increment` | integer (default: 0) | no | The desired increment in seconds to the token expiration |




#### Responses


**200**: OK



### POST /auth/token/renew-self

**Operation ID:** `token-renew-self`


This endpoint will renew the token used to call it and prevent expiration.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `increment` | integer (default: 0) | no | The desired increment in seconds to the token expiration |
| `token` | string | no | Token to renew (unused, does not need to be set) |




#### Responses


**200**: OK



### POST /auth/token/revoke

**Operation ID:** `token-revoke`


This endpoint will delete the given token and all of its child tokens.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token` | string | no | Token to revoke (request body) |




#### Responses


**200**: OK



### POST /auth/token/revoke-accessor

**Operation ID:** `token-revoke-accessor`


This endpoint will delete the token associated with the accessor and all of its child tokens.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `accessor` | string | no | Accessor of the token (request body) |




#### Responses


**200**: OK



### POST /auth/token/revoke-orphan

**Operation ID:** `token-revoke-orphan`


This endpoint will delete the token and orphan its child tokens.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token` | string | no | Token to revoke (request body) |




#### Responses


**200**: OK



### POST /auth/token/revoke-self

**Operation ID:** `token-revoke-self`


This endpoint will delete the token used to call it and all of its child tokens.


#### Responses


**200**: OK



### GET /auth/token/roles

**Operation ID:** `token-list-roles`


This endpoint lists configured roles.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /auth/token/roles/{role_name}

**Operation ID:** `token-read-role`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role |




#### Responses


**200**: OK



### POST /auth/token/roles/{role_name}

**Operation ID:** `token-write-role`



**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allowed_entity_aliases` | array | no | String or JSON list of allowed entity aliases. If set, specifies the entity aliases which are allowed to be used during token generation. This field supports globbing. |
| `allowed_policies` | array | no | If set, tokens can be created with any subset of the policies in this list, rather than the normal semantics of tokens being a subset of the calling token's policies. The parameter is a comma-delimited string of policy names. |
| `allowed_policies_glob` | array | no | If set, tokens can be created with any subset of glob matched policies in this list, rather than the normal semantics of tokens being a subset of the calling token's policies. The parameter is a comma-delimited string of policy name globs. |
| `bound_cidrs` | array | no | ⚠️ Deprecated. Use 'token_bound_cidrs' instead. |
| `disallowed_policies` | array | no | If set, successful token creation via this role will require that no policies in the given list are requested. The parameter is a comma-delimited string of policy names. |
| `disallowed_policies_glob` | array | no | If set, successful token creation via this role will require that no requested policies glob match any of policies in this list. The parameter is a comma-delimited string of policy name globs. |
| `explicit_max_ttl` | integer | no | ⚠️ Deprecated. Use 'token_explicit_max_ttl' instead. |
| `orphan` | boolean | no | If true, tokens created via this role will be orphan tokens (have no parent) |
| `path_suffix` | string | no | If set, tokens created via this role will contain the given suffix as a part of their path. This can be used to assist use of the 'revoke-prefix' endpoint later on. The given suffix must match the regular expression.\w[\w-.]+\w |
| `period` | integer | no | ⚠️ Deprecated. Use 'token_period' instead. |
| `renewable` | boolean (default: True) | no | Tokens created via this role will be renewable or not according to this value. Defaults to "true". |
| `token_bound_cidrs` | array | no | Comma separated string or JSON list of CIDR blocks. If set, specifies the blocks of IP addresses which are allowed to use the generated token. |
| `token_explicit_max_ttl` | integer | no | If set, tokens created via this role carry an explicit maximum TTL. During renewal, the current maximum TTL values of the role and the mount are not checked for changes, and any updates to these values will have no effect on the token being renewed. |
| `token_no_default_policy` | boolean | no | If true, the 'default' policy will not automatically be added to generated tokens |
| `token_num_uses` | integer | no | The maximum number of times a token may be used, a value of zero means unlimited |
| `token_period` | integer | no | If set, tokens created via this role will have no max lifetime; instead, their renewal period will be fixed to this value. This takes an integer number of seconds, or a string duration (e.g. "24h"). |
| `token_type` | string (default: default-service) | no | The type of token to generate, service or batch |




#### Responses


**200**: OK



### DELETE /auth/token/roles/{role_name}

**Operation ID:** `token-delete-role`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role |




#### Responses


**204**: empty body



### POST /auth/token/tidy

**Operation ID:** `token-tidy`


This endpoint performs cleanup tasks that can be run if certain error
conditions have occurred.


#### Responses


**200**: OK



### POST /auth/{approle_mount_path}/login

**Operation ID:** `app-role-login`


Issue a token based on the credentials supplied


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `role_id` | string | no | Unique identifier of the Role. Required to be supplied when the 'bind_secret_id' constraint is set. |
| `secret_id` | string (default: ) | no | SecretID belong to the App role |




#### Responses


**200**: OK



### GET /auth/{approle_mount_path}/role

**Operation ID:** `app-role-list-roles`


Lists all the roles registered with the backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no |  |





### GET /auth/{approle_mount_path}/role/{role_name}

**Operation ID:** `app-role-read-role`


Register an role with the backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bind_secret_id` | boolean | no | Impose secret ID to be presented when logging in using this role. |
| `local_secret_ids` | boolean | no | If true, the secret identifiers generated using this role will be cluster local. This can only be set during role creation and once set, it can't be reset later |
| `period` | integer | no | ⚠️ Deprecated. Use "token_period" instead. If this and "token_period" are both specified, only "token_period" will be used. |
| `policies` | array | no | ⚠️ Deprecated. Use "token_policies" instead. If this and "token_policies" are both specified, only "token_policies" will be used. |
| `secret_id_bound_cidrs` | array | no | Comma separated string or list of CIDR blocks. If set, specifies the blocks of IP addresses which can perform the login operation. |
| `secret_id_num_uses` | integer | no | Number of times a secret ID can access the role, after which the secret ID will expire. |
| `secret_id_ttl` | integer | no | Duration in seconds after which the issued secret ID expires. |
| `token_bound_cidrs` | array | no | Comma separated string or JSON list of CIDR blocks. If set, specifies the blocks of IP addresses which are allowed to use the generated token. |
| `token_explicit_max_ttl` | integer | no | If set, tokens created via this role carry an explicit maximum TTL. During renewal, the current maximum TTL values of the role and the mount are not checked for changes, and any updates to these values will have no effect on the token being renewed. |
| `token_max_ttl` | integer | no | The maximum lifetime of the generated token |
| `token_no_default_policy` | boolean | no | If true, the 'default' policy will not automatically be added to generated tokens |
| `token_num_uses` | integer | no | The maximum number of times a token may be used, a value of zero means unlimited |
| `token_period` | integer | no | If set, tokens created via this role will have no max lifetime; instead, their renewal period will be fixed to this value. |
| `token_policies` | array | no | Comma-separated list of policies |
| `token_ttl` | integer | no | The initial ttl of the token to generate |
| `token_type` | string (default: default-service) | no | The type of token to generate, service or batch |





### POST /auth/{approle_mount_path}/role/{role_name}

**Operation ID:** `app-role-write-role`


Register an role with the backend.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bind_secret_id` | boolean (default: True) | no | Impose secret_id to be presented when logging in using this role. Defaults to 'true'. |
| `bound_cidr_list` | array | no | ⚠️ Deprecated. Use "secret_id_bound_cidrs" instead. |
| `local_secret_ids` | boolean | no | If set, the secret IDs generated using this role will be cluster local. This can only be set during role creation and once set, it can't be reset later. |
| `period` | integer | no | ⚠️ Deprecated. Use "token_period" instead. If this and "token_period" are both specified, only "token_period" will be used. |
| `policies` | array | no | ⚠️ Deprecated. Use "token_policies" instead. If this and "token_policies" are both specified, only "token_policies" will be used. |
| `role_id` | string | no | Identifier of the role. Defaults to a UUID. |
| `secret_id_bound_cidrs` | array | no | Comma separated string or list of CIDR blocks. If set, specifies the blocks of IP addresses which can perform the login operation. |
| `secret_id_num_uses` | integer | no | Number of times a SecretID can access the role, after which the SecretID will expire. Defaults to 0 meaning that the the secret_id is of unlimited use. |
| `secret_id_ttl` | integer | no | Duration in seconds after which the issued SecretID should expire. Defaults to 0, meaning no expiration. |
| `token_bound_cidrs` | array | no | Comma separated string or JSON list of CIDR blocks. If set, specifies the blocks of IP addresses which are allowed to use the generated token. |
| `token_explicit_max_ttl` | integer | no | If set, tokens created via this role carry an explicit maximum TTL. During renewal, the current maximum TTL values of the role and the mount are not checked for changes, and any updates to these values will have no effect on the token being renewed. |
| `token_max_ttl` | integer | no | The maximum lifetime of the generated token |
| `token_no_default_policy` | boolean | no | If true, the 'default' policy will not automatically be added to generated tokens |
| `token_num_uses` | integer | no | The maximum number of times a token may be used, a value of zero means unlimited |
| `token_period` | integer | no | If set, tokens created via this role will have no max lifetime; instead, their renewal period will be fixed to this value. This takes an integer number of seconds, or a string duration (e.g. "24h"). |
| `token_policies` | array | no | Comma-separated list of policies |
| `token_ttl` | integer | no | The initial ttl of the token to generate |
| `token_type` | string (default: default-service) | no | The type of token to generate, service or batch |




#### Responses


**200**: OK



### DELETE /auth/{approle_mount_path}/role/{role_name}

**Operation ID:** `app-role-delete-role`


Register an role with the backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### GET /auth/{approle_mount_path}/role/{role_name}/bind-secret-id

**Operation ID:** `app-role-read-bind-secret-id`


Impose secret_id to be presented during login using this role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bind_secret_id` | boolean | no | Impose secret_id to be presented when logging in using this role. Defaults to 'true'. |





### POST /auth/{approle_mount_path}/role/{role_name}/bind-secret-id

**Operation ID:** `app-role-write-bind-secret-id`


Impose secret_id to be presented during login using this role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bind_secret_id` | boolean (default: True) | no | Impose secret_id to be presented when logging in using this role. |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/bind-secret-id

**Operation ID:** `app-role-delete-bind-secret-id`


Impose secret_id to be presented during login using this role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### GET /auth/{approle_mount_path}/role/{role_name}/bound-cidr-list

**Operation ID:** `app-role-read-bound-cidr-list`


Deprecated: Comma separated list of CIDR blocks, if set, specifies blocks of IP addresses which can perform the login operation


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bound_cidr_list` | array | no | ⚠️ Deprecated. Deprecated: Please use "secret_id_bound_cidrs" instead. Comma separated string or list of CIDR blocks. If set, specifies the blocks of IP addresses which can perform the login operation. |





### POST /auth/{approle_mount_path}/role/{role_name}/bound-cidr-list

**Operation ID:** `app-role-write-bound-cidr-list`


Deprecated: Comma separated list of CIDR blocks, if set, specifies blocks of IP addresses which can perform the login operation


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bound_cidr_list` | array | no | Deprecated: Please use "secret_id_bound_cidrs" instead. Comma separated string or list of CIDR blocks. If set, specifies the blocks of IP addresses which can perform the login operation. |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/bound-cidr-list

**Operation ID:** `app-role-delete-bound-cidr-list`


Deprecated: Comma separated list of CIDR blocks, if set, specifies blocks of IP addresses which can perform the login operation


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### POST /auth/{approle_mount_path}/role/{role_name}/custom-secret-id

**Operation ID:** `app-role-write-custom-secret-id`


Assign a SecretID of choice against the role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `cidr_list` | array | no | Comma separated string or list of CIDR blocks enforcing secret IDs to be used from specific set of IP addresses. If 'bound_cidr_list' is set on the role, then the list of CIDR blocks listed here should be a subset of the CIDR blocks listed on the role. |
| `metadata` | string | no | Metadata to be tied to the SecretID. This should be a JSON formatted string containing metadata in key value pairs. |
| `num_uses` | integer | no | Number of times this SecretID can be used, after which the SecretID expires. Overrides secret_id_num_uses role option when supplied. May not be higher than role's secret_id_num_uses. |
| `secret_id` | string | no | SecretID to be attached to the role. |
| `token_bound_cidrs` | array | no | Comma separated string or list of CIDR blocks. If set, specifies the blocks of IP addresses which can use the returned token. Should be a subset of the token CIDR blocks listed on the role, if any. |
| `ttl` | integer | no | Duration in seconds after which this SecretID expires. Overrides secret_id_ttl role option when supplied. May not be longer than role's secret_id_ttl. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `secret_id` | string | no | Secret ID attached to the role. |
| `secret_id_accessor` | string | no | Accessor of the secret ID |
| `secret_id_num_uses` | integer | no | Number of times a secret ID can access the role, after which the secret ID will expire. |
| `secret_id_ttl` | integer | no | Duration in seconds after which the issued secret ID expires. |





### GET /auth/{approle_mount_path}/role/{role_name}/local-secret-ids

**Operation ID:** `app-role-read-local-secret-ids`


Enables cluster local secret IDs


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `local_secret_ids` | boolean | no | If true, the secret identifiers generated using this role will be cluster local. This can only be set during role creation and once set, it can't be reset later |





### GET /auth/{approle_mount_path}/role/{role_name}/period

**Operation ID:** `app-role-read-period`


Updates the value of 'period' on the role


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `period` | integer | no | ⚠️ Deprecated. Use "token_period" instead. If this and "token_period" are both specified, only "token_period" will be used. |
| `token_period` | integer | no | If set, tokens created via this role will have no max lifetime; instead, their renewal period will be fixed to this value. This takes an integer number of seconds, or a string duration (e.g. "24h"). |





### POST /auth/{approle_mount_path}/role/{role_name}/period

**Operation ID:** `app-role-write-period`


Updates the value of 'period' on the role


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `period` | integer | no | ⚠️ Deprecated. Use "token_period" instead. If this and "token_period" are both specified, only "token_period" will be used. |
| `token_period` | integer | no | If set, tokens created via this role will have no max lifetime; instead, their renewal period will be fixed to this value. This takes an integer number of seconds, or a string duration (e.g. "24h"). |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/period

**Operation ID:** `app-role-delete-period`


Updates the value of 'period' on the role


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### GET /auth/{approle_mount_path}/role/{role_name}/policies

**Operation ID:** `app-role-read-policies`


Policies of the role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `policies` | array | no | ⚠️ Deprecated. Use "token_policies" instead. If this and "token_policies" are both specified, only "token_policies" will be used. |
| `token_policies` | array | no | Comma-separated list of policies |





### POST /auth/{approle_mount_path}/role/{role_name}/policies

**Operation ID:** `app-role-write-policies`


Policies of the role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `policies` | array | no | ⚠️ Deprecated. Use "token_policies" instead. If this and "token_policies" are both specified, only "token_policies" will be used. |
| `token_policies` | array | no | Comma-separated list of policies |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/policies

**Operation ID:** `app-role-delete-policies`


Policies of the role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### GET /auth/{approle_mount_path}/role/{role_name}/role-id

**Operation ID:** `app-role-read-role-id`


Returns the 'role_id' of the role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `role_id` | string | no | Identifier of the role. Defaults to a UUID. |





### POST /auth/{approle_mount_path}/role/{role_name}/role-id

**Operation ID:** `app-role-write-role-id`


Returns the 'role_id' of the role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `role_id` | string | no | Identifier of the role. Defaults to a UUID. |




#### Responses


**204**: No Content



### GET /auth/{approle_mount_path}/role/{role_name}/secret-id

**Operation ID:** `app-role-list-secret-ids`


Generate a SecretID against this role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no |  |





### POST /auth/{approle_mount_path}/role/{role_name}/secret-id

**Operation ID:** `app-role-write-secret-id`


Generate a SecretID against this role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `cidr_list` | array | no | Comma separated string or list of CIDR blocks enforcing secret IDs to be used from specific set of IP addresses. If 'bound_cidr_list' is set on the role, then the list of CIDR blocks listed here should be a subset of the CIDR blocks listed on the role. |
| `metadata` | string | no | Metadata to be tied to the SecretID. This should be a JSON formatted string containing the metadata in key value pairs. |
| `num_uses` | integer | no | Number of times this SecretID can be used, after which the SecretID expires. Overrides secret_id_num_uses role option when supplied. May not be higher than role's secret_id_num_uses. |
| `token_bound_cidrs` | array | no | Comma separated string or JSON list of CIDR blocks. If set, specifies the blocks of IP addresses which are allowed to use the generated token. |
| `ttl` | integer | no | Duration in seconds after which this SecretID expires. Overrides secret_id_ttl role option when supplied. May not be longer than role's secret_id_ttl. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `secret_id` | string | no | Secret ID attached to the role. |
| `secret_id_accessor` | string | no | Accessor of the secret ID |
| `secret_id_num_uses` | integer | no | Number of times a secret ID can access the role, after which the secret ID will expire. |
| `secret_id_ttl` | integer | no | Duration in seconds after which the issued secret ID expires. |





### POST /auth/{approle_mount_path}/role/{role_name}/secret-id-accessor/destroy

**Operation ID:** `app-role-destroy-secret-id-by-accessor`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `secret_id_accessor` | string | no | Accessor of the SecretID |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/secret-id-accessor/destroy

**Operation ID:** `app-role-destroy-secret-id-by-accessor2`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### POST /auth/{approle_mount_path}/role/{role_name}/secret-id-accessor/lookup

**Operation ID:** `app-role-look-up-secret-id-by-accessor`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `secret_id_accessor` | string | no | Accessor of the SecretID |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `cidr_list` | array | no | List of CIDR blocks enforcing secret IDs to be used from specific set of IP addresses. If 'bound_cidr_list' is set on the role, then the list of CIDR blocks listed here should be a subset of the CIDR blocks listed on the role. |
| `creation_time` | string | no |  |
| `expiration_time` | string | no |  |
| `last_updated_time` | string | no |  |
| `metadata` | object | no |  |
| `secret_id_accessor` | string | no | Accessor of the secret ID |
| `secret_id_num_uses` | integer | no | Number of times a secret ID can access the role, after which the secret ID will expire. |
| `secret_id_ttl` | integer | no | Duration in seconds after which the issued secret ID expires. |
| `token_bound_cidrs` | array | no | List of CIDR blocks. If set, specifies the blocks of IP addresses which can use the returned token. Should be a subset of the token CIDR blocks listed on the role, if any. |





### GET /auth/{approle_mount_path}/role/{role_name}/secret-id-bound-cidrs

**Operation ID:** `app-role-read-secret-id-bound-cidrs`


Comma separated list of CIDR blocks, if set, specifies blocks of IP addresses which can perform the login operation


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `secret_id_bound_cidrs` | array | no | Comma separated string or list of CIDR blocks. If set, specifies the blocks of IP addresses which can perform the login operation. |





### POST /auth/{approle_mount_path}/role/{role_name}/secret-id-bound-cidrs

**Operation ID:** `app-role-write-secret-id-bound-cidrs`


Comma separated list of CIDR blocks, if set, specifies blocks of IP addresses which can perform the login operation


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `secret_id_bound_cidrs` | array | no | Comma separated string or list of CIDR blocks. If set, specifies the blocks of IP addresses which can perform the login operation. |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/secret-id-bound-cidrs

**Operation ID:** `app-role-delete-secret-id-bound-cidrs`


Comma separated list of CIDR blocks, if set, specifies blocks of IP addresses which can perform the login operation


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### GET /auth/{approle_mount_path}/role/{role_name}/secret-id-num-uses

**Operation ID:** `app-role-read-secret-id-num-uses`


Use limit of the SecretID generated against the role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `secret_id_num_uses` | integer | no | Number of times a secret ID can access the role, after which the SecretID will expire. Defaults to 0 meaning that the secret ID is of unlimited use. |





### POST /auth/{approle_mount_path}/role/{role_name}/secret-id-num-uses

**Operation ID:** `app-role-write-secret-id-num-uses`


Use limit of the SecretID generated against the role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `secret_id_num_uses` | integer | no | Number of times a SecretID can access the role, after which the SecretID will expire. |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/secret-id-num-uses

**Operation ID:** `app-role-delete-secret-id-num-uses`


Use limit of the SecretID generated against the role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### GET /auth/{approle_mount_path}/role/{role_name}/secret-id-ttl

**Operation ID:** `app-role-read-secret-id-ttl`


Duration in seconds of the SecretID generated against the role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `secret_id_ttl` | integer | no | Duration in seconds after which the issued secret ID should expire. Defaults to 0, meaning no expiration. |





### POST /auth/{approle_mount_path}/role/{role_name}/secret-id-ttl

**Operation ID:** `app-role-write-secret-id-ttl`


Duration in seconds of the SecretID generated against the role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `secret_id_ttl` | integer | no | Duration in seconds after which the issued SecretID should expire. Defaults to 0, meaning no expiration. |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/secret-id-ttl

**Operation ID:** `app-role-delete-secret-id-ttl`


Duration in seconds of the SecretID generated against the role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### POST /auth/{approle_mount_path}/role/{role_name}/secret-id/destroy

**Operation ID:** `app-role-destroy-secret-id`


Invalidate an issued secret_id


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `secret_id` | string | no | SecretID attached to the role. |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/secret-id/destroy

**Operation ID:** `app-role-destroy-secret-id2`


Invalidate an issued secret_id


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### POST /auth/{approle_mount_path}/role/{role_name}/secret-id/lookup

**Operation ID:** `app-role-look-up-secret-id`


Read the properties of an issued secret_id


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `secret_id` | string | no | SecretID attached to the role. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `cidr_list` | array | no | List of CIDR blocks enforcing secret IDs to be used from specific set of IP addresses. If 'bound_cidr_list' is set on the role, then the list of CIDR blocks listed here should be a subset of the CIDR blocks listed on the role. |
| `creation_time` | string | no |  |
| `expiration_time` | string | no |  |
| `last_updated_time` | string | no |  |
| `metadata` | object | no |  |
| `secret_id_accessor` | string | no | Accessor of the secret ID |
| `secret_id_num_uses` | integer | no | Number of times a secret ID can access the role, after which the secret ID will expire. |
| `secret_id_ttl` | integer | no | Duration in seconds after which the issued secret ID expires. |
| `token_bound_cidrs` | array | no | List of CIDR blocks. If set, specifies the blocks of IP addresses which can use the returned token. Should be a subset of the token CIDR blocks listed on the role, if any. |





### GET /auth/{approle_mount_path}/role/{role_name}/token-bound-cidrs

**Operation ID:** `app-role-read-token-bound-cidrs`


Comma separated string or list of CIDR blocks. If set, specifies the blocks of IP addresses which can use the returned token.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token_bound_cidrs` | array | no | Comma separated string or list of CIDR blocks. If set, specifies the blocks of IP addresses which can use the returned token. Should be a subset of the token CIDR blocks listed on the role, if any. |





### POST /auth/{approle_mount_path}/role/{role_name}/token-bound-cidrs

**Operation ID:** `app-role-write-token-bound-cidrs`


Comma separated string or list of CIDR blocks. If set, specifies the blocks of IP addresses which can use the returned token.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token_bound_cidrs` | array | no | Comma separated string or JSON list of CIDR blocks. If set, specifies the blocks of IP addresses which are allowed to use the generated token. |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/token-bound-cidrs

**Operation ID:** `app-role-delete-token-bound-cidrs`


Comma separated string or list of CIDR blocks. If set, specifies the blocks of IP addresses which can use the returned token.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### GET /auth/{approle_mount_path}/role/{role_name}/token-max-ttl

**Operation ID:** `app-role-read-token-max-ttl`


Duration in seconds, the maximum lifetime of the tokens issued by using the SecretIDs that were generated against this role, after which the tokens are not allowed to be renewed.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token_max_ttl` | integer | no | The maximum lifetime of the generated token |





### POST /auth/{approle_mount_path}/role/{role_name}/token-max-ttl

**Operation ID:** `app-role-write-token-max-ttl`


Duration in seconds, the maximum lifetime of the tokens issued by using the SecretIDs that were generated against this role, after which the tokens are not allowed to be renewed.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token_max_ttl` | integer | no | The maximum lifetime of the generated token |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/token-max-ttl

**Operation ID:** `app-role-delete-token-max-ttl`


Duration in seconds, the maximum lifetime of the tokens issued by using the SecretIDs that were generated against this role, after which the tokens are not allowed to be renewed.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### GET /auth/{approle_mount_path}/role/{role_name}/token-num-uses

**Operation ID:** `app-role-read-token-num-uses`


Number of times issued tokens can be used


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token_num_uses` | integer | no | The maximum number of times a token may be used, a value of zero means unlimited |





### POST /auth/{approle_mount_path}/role/{role_name}/token-num-uses

**Operation ID:** `app-role-write-token-num-uses`


Number of times issued tokens can be used


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token_num_uses` | integer | no | The maximum number of times a token may be used, a value of zero means unlimited |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/token-num-uses

**Operation ID:** `app-role-delete-token-num-uses`


Number of times issued tokens can be used


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### GET /auth/{approle_mount_path}/role/{role_name}/token-ttl

**Operation ID:** `app-role-read-token-ttl`


Duration in seconds, the lifetime of the token issued by using the SecretID that is generated against this role, before which the token needs to be renewed.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token_ttl` | integer | no | The initial ttl of the token to generate |





### POST /auth/{approle_mount_path}/role/{role_name}/token-ttl

**Operation ID:** `app-role-write-token-ttl`


Duration in seconds, the lifetime of the token issued by using the SecretID that is generated against this role, before which the token needs to be renewed.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token_ttl` | integer | no | The initial ttl of the token to generate |




#### Responses


**204**: No Content



### DELETE /auth/{approle_mount_path}/role/{role_name}/token-ttl

**Operation ID:** `app-role-delete-token-ttl`


Duration in seconds, the lifetime of the token issued by using the SecretID that is generated against this role, before which the token needs to be renewed.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role_name` | string | path | yes | Name of the role. Must be less than 4096 bytes. |
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### POST /auth/{approle_mount_path}/tidy/secret-id

**Operation ID:** `app-role-tidy-secret-id`


Trigger the clean-up of expired SecretID entries.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `approle_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**202**: Accepted



### GET /auth/{jwt_mount_path}/config

**Operation ID:** `jwt-read-configuration`


Read the current JWT authentication backend configuration.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `jwt_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /auth/{jwt_mount_path}/config

**Operation ID:** `jwt-configure`


Configure the JWT authentication backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `jwt_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bound_issuer` | string | no | The value against which to match the 'iss' claim in a JWT. Optional. |
| `default_role` | string | no | The default role to use if none is provided during login. If not set, a role is required during login. |
| `jwks_ca_pem` | string | no | The CA certificate or chain of certificates, in PEM format, to use to validate connections to the JWKS URL. If not set, system certificates are used. |
| `jwks_pairs` | array | no | Set of JWKS Url and CA certificate (or chain of certificates) pairs. CA certificates must be in PEM format. Cannot be used with "jwks_url" or "jwks_ca_pem". |
| `jwks_url` | string | no | JWKS URL to use to authenticate signatures. Cannot be used with "oidc_discovery_url" or "jwt_validation_pubkeys". |
| `jwt_supported_algs` | array | no | A list of supported signing algorithms. Defaults to RS256. |
| `jwt_validation_pubkeys` | array | no | A list of PEM-encoded public keys to use to authenticate signatures locally. Cannot be used with "jwks_url" or "oidc_discovery_url". |
| `namespace_in_state` | boolean | no | Pass namespace in the OIDC state parameter instead of as a separate query parameter. With this setting, the allowed redirect URL(s) in Vault and on the provider side should not contain a namespace query parameter. This means only one redirect URL entry needs to be maintained on the provider side for all vault namespaces that will be authenticating against it. Defaults to true for new configs. |
| `oidc_client_id` | string | no | The OAuth Client ID configured with your OIDC provider. |
| `oidc_client_secret` | string | no | The OAuth Client Secret configured with your OIDC provider. |
| `oidc_discovery_ca_pem` | string | no | The CA certificate or chain of certificates, in PEM format, to use to validate connections to the OIDC Discovery URL. If not set, system certificates are used. |
| `oidc_discovery_url` | string | no | OIDC Discovery URL, without any .well-known component (base path). Cannot be used with "jwks_url" or "jwt_validation_pubkeys". |
| `oidc_response_mode` | string | no | The response mode to be used in the OAuth2 request. Allowed values are 'query' and 'form_post'. |
| `oidc_response_types` | array | no | The response types to request. Allowed values are 'code' and 'id_token'. Defaults to 'code'. |
| `provider_config` | object | no | Provider-specific configuration. Optional. |
| `unsupported_critical_cert_extensions` | array | no | A list of ASN1 OIDs of certificate extensions marked Critical that are unsupported by Vault and should be ignored. This option should very rarely be needed except in specialized PKI environments. |




#### Responses


**200**: OK



### POST /auth/{jwt_mount_path}/login

**Operation ID:** `jwt-login`


Authenticates to Vault using a JWT (or OIDC) token.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `jwt_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `distributed_claim_access_token` | string | no | An optional token used to fetch group memberships specified by the distributed claim source in the jwt. This is supported only on Azure/Entra ID |
| `jwt` | string | no | The signed JWT to validate. |
| `role` | string | no | The role to log in against. |




#### Responses


**200**: OK



### POST /auth/{jwt_mount_path}/oidc/auth_url

**Operation ID:** `jwt-oidc-request-authorization-url`


Request an authorization URL to start an OIDC login flow.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `jwt_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `client_nonce` | string | no | Optional client-provided nonce that must match during callback, if present. |
| `redirect_uri` | string | no | The OAuth redirect_uri to use in the authorization URL. |
| `role` | string | no | The role to issue an OIDC authorization URL against. |




#### Responses


**200**: OK



### GET /auth/{jwt_mount_path}/oidc/callback

**Operation ID:** `jwt-oidc-callback`


Callback endpoint to complete an OIDC login.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `client_nonce` | string | query | no |  |
| `code` | string | query | no |  |
| `state` | string | query | no |  |
| `jwt_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /auth/{jwt_mount_path}/oidc/callback

**Operation ID:** `jwt-oidc-callback-form-post`


Callback endpoint to handle form_posts.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `client_nonce` | string | query | no |  |
| `code` | string | query | no |  |
| `state` | string | query | no |  |
| `jwt_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `id_token` | string | no |  |




#### Responses


**200**: OK



### GET /auth/{jwt_mount_path}/role

**Operation ID:** `jwt-list-roles`


Lists all the roles registered with the backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `jwt_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /auth/{jwt_mount_path}/role/{name}

**Operation ID:** `jwt-read-role`


Read an existing role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `jwt_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /auth/{jwt_mount_path}/role/{name}

**Operation ID:** `jwt-write-role`


Register an role with the backend.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `jwt_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allowed_redirect_uris` | array | no | Comma-separated list of allowed values for redirect_uri |
| `bound_audiences` | array | no | Comma-separated list of 'aud' claims that are valid for login; any match is sufficient |
| `bound_cidrs` | array | no | ⚠️ Deprecated. Use "token_bound_cidrs" instead. If this and "token_bound_cidrs" are both specified, only "token_bound_cidrs" will be used. |
| `bound_claims` | object | no | Map of claims/values which must match for login |
| `bound_claims_type` | string (default: string) | no | How to interpret values in the map of claims/values (which must match for login): allowed values are 'string' or 'glob' |
| `bound_subject` | string | no | The 'sub' claim that is valid for login. Optional. |
| `claim_mappings` | object | no | Mappings of claims (key) that will be copied to a metadata field (value) |
| `clock_skew_leeway` | integer (default: 60000000000) | no | Duration in seconds of leeway when validating all claims to account for clock skew. Defaults to 60 (1 minute) if set to 0 and can be disabled if set to -1. |
| `expiration_leeway` | integer (default: 150) | no | Duration in seconds of leeway when validating expiration of a token to account for clock skew. Defaults to 150 (2.5 minutes) if set to 0 and can be disabled if set to -1. |
| `groups_claim` | string | no | The claim to use for the Identity group alias names |
| `max_age` | integer | no | Specifies the allowable elapsed time in seconds since the last time the user was actively authenticated. |
| `max_ttl` | integer | no | ⚠️ Deprecated. Use "token_max_ttl" instead. If this and "token_max_ttl" are both specified, only "token_max_ttl" will be used. |
| `not_before_leeway` | integer (default: 150) | no | Duration in seconds of leeway when validating not before values of a token to account for clock skew. Defaults to 150 (2.5 minutes) if set to 0 and can be disabled if set to -1. |
| `num_uses` | integer | no | ⚠️ Deprecated. Use "token_num_uses" instead. If this and "token_num_uses" are both specified, only "token_num_uses" will be used. |
| `oidc_scopes` | array | no | Comma-separated list of OIDC scopes |
| `period` | integer | no | ⚠️ Deprecated. Use "token_period" instead. If this and "token_period" are both specified, only "token_period" will be used. |
| `policies` | array | no | ⚠️ Deprecated. Use "token_policies" instead. If this and "token_policies" are both specified, only "token_policies" will be used. |
| `role_type` | string | no | Type of the role, either 'jwt' or 'oidc'. |
| `token_bound_cidrs` | array | no | Comma separated string or JSON list of CIDR blocks. If set, specifies the blocks of IP addresses which are allowed to use the generated token. |
| `token_explicit_max_ttl` | integer | no | If set, tokens created via this role carry an explicit maximum TTL. During renewal, the current maximum TTL values of the role and the mount are not checked for changes, and any updates to these values will have no effect on the token being renewed. |
| `token_max_ttl` | integer | no | The maximum lifetime of the generated token |
| `token_no_default_policy` | boolean | no | If true, the 'default' policy will not automatically be added to generated tokens |
| `token_num_uses` | integer | no | The maximum number of times a token may be used, a value of zero means unlimited |
| `token_period` | integer | no | If set, tokens created via this role will have no max lifetime; instead, their renewal period will be fixed to this value. This takes an integer number of seconds, or a string duration (e.g. "24h"). |
| `token_policies` | array | no | Comma-separated list of policies |
| `token_ttl` | integer | no | The initial ttl of the token to generate |
| `token_type` | string (default: default-service) | no | The type of token to generate, service or batch |
| `ttl` | integer | no | ⚠️ Deprecated. Use "token_ttl" instead. If this and "token_ttl" are both specified, only "token_ttl" will be used. |
| `user_claim` | string | no | The claim to use for the Identity entity alias name |
| `user_claim_json_pointer` | boolean | no | If true, the user_claim value will use JSON pointer syntax for referencing claims. |
| `verbose_oidc_logging` | boolean | no | Log received OIDC tokens and claims when debug-level logging is active. Not recommended in production since sensitive information may be present in OIDC responses. |




#### Responses


**200**: OK



### DELETE /auth/{jwt_mount_path}/role/{name}

**Operation ID:** `jwt-delete-role`


Delete an existing role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `jwt_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### GET /auth/{kubernetes_mount_path}/config

**Operation ID:** `kubernetes-read-auth-configuration`


Configures the JWT Public Key and Kubernetes API information.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /auth/{kubernetes_mount_path}/config

**Operation ID:** `kubernetes-configure-auth`


Configures the JWT Public Key and Kubernetes API information.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `disable_iss_validation` | boolean (default: True) | no | ⚠️ Deprecated. Disable JWT issuer validation (Deprecated, will be removed in a future release) |
| `disable_local_ca_jwt` | boolean (default: False) | no | Disable defaulting to the local CA cert and service account JWT when running in a Kubernetes pod |
| `issuer` | string | no | ⚠️ Deprecated. Optional JWT issuer. If no issuer is specified, then this plugin will use kubernetes.io/serviceaccount as the default issuer. (Deprecated, will be removed in a future release) |
| `kubernetes_ca_cert` | string | no | Optional PEM encoded CA cert for use by the TLS client used to talk with the API. If it is not set and disable_local_ca_jwt is true, the system's trusted CA certificate pool will be used. |
| `kubernetes_host` | string | no | Host must be a host string, a host:port pair, or a URL to the base of the Kubernetes API server. |
| `pem_keys` | array | no | Optional list of PEM-formated public keys or certificates used to verify the signatures of kubernetes service account JWTs. If a certificate is given, its public key will be extracted. Not every installation of Kubernetes exposes these keys. |
| `token_reviewer_jwt` | string | no | A service account JWT (or other token) used as a bearer token to access the TokenReview API to validate other JWTs during login. If not set the JWT used for login will be used to access the API. |
| `use_annotations_as_alias_metadata` | boolean (default: False) | no | Use annotations from the client token's associated service account as alias metadata for the Vault entity. Only annotations with the prefix "vault.hashicorp.com/alias-metadata-" will be used. Note that Vault will need permission to read service accounts from the Kubernetes API. |




#### Responses


**200**: OK



### POST /auth/{kubernetes_mount_path}/login

**Operation ID:** `kubernetes-login`


Authenticates Kubernetes service accounts with Vault.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `jwt` | string | no | A signed JWT for authenticating a service account. This field is required. |
| `role` | string | no | Name of the role against which the login is being attempted. This field is required |




#### Responses


**200**: OK



### GET /auth/{kubernetes_mount_path}/role

**Operation ID:** `kubernetes-list-auth-roles`


Lists all the roles registered with the backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /auth/{kubernetes_mount_path}/role/{name}

**Operation ID:** `kubernetes-read-auth-role`


Register an role with the backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /auth/{kubernetes_mount_path}/role/{name}

**Operation ID:** `kubernetes-write-auth-role`


Register an role with the backend.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alias_name_source` | string (default: serviceaccount_uid) | no | Source to use when deriving the Alias name. valid choices: "serviceaccount_uid" : <token.uid> e.g. 474b11b5-0f20-4f9d-8ca5-65715ab325e0 (most secure choice) "serviceaccount_name" : <namespace>/<serviceaccount> e.g. vault/vault-agent default: "serviceaccount_uid" |
| `audience` | string | no | Optional Audience claim to verify in the jwt. |
| `bound_cidrs` | array | no | ⚠️ Deprecated. Use "token_bound_cidrs" instead. If this and "token_bound_cidrs" are both specified, only "token_bound_cidrs" will be used. |
| `bound_service_account_names` | array | no | List of service account names able to access this role. If set to "*" all names are allowed. |
| `bound_service_account_namespace_selector` | string | no | A label selector for Kubernetes namespaces which are allowed to access this role. Accepts either a JSON or YAML object. If set with bound_service_account_namespaces, the conditions are ORed. |
| `bound_service_account_namespaces` | array | no | List of namespaces allowed to access this role. If set to "*" all namespaces are allowed. |
| `max_ttl` | integer | no | ⚠️ Deprecated. Use "token_max_ttl" instead. If this and "token_max_ttl" are both specified, only "token_max_ttl" will be used. |
| `num_uses` | integer | no | ⚠️ Deprecated. Use "token_num_uses" instead. If this and "token_num_uses" are both specified, only "token_num_uses" will be used. |
| `period` | integer | no | ⚠️ Deprecated. Use "token_period" instead. If this and "token_period" are both specified, only "token_period" will be used. |
| `policies` | array | no | ⚠️ Deprecated. Use "token_policies" instead. If this and "token_policies" are both specified, only "token_policies" will be used. |
| `token_bound_cidrs` | array | no | Comma separated string or JSON list of CIDR blocks. If set, specifies the blocks of IP addresses which are allowed to use the generated token. |
| `token_explicit_max_ttl` | integer | no | If set, tokens created via this role carry an explicit maximum TTL. During renewal, the current maximum TTL values of the role and the mount are not checked for changes, and any updates to these values will have no effect on the token being renewed. |
| `token_max_ttl` | integer | no | The maximum lifetime of the generated token |
| `token_no_default_policy` | boolean | no | If true, the 'default' policy will not automatically be added to generated tokens |
| `token_num_uses` | integer | no | The maximum number of times a token may be used, a value of zero means unlimited |
| `token_period` | integer | no | If set, tokens created via this role will have no max lifetime; instead, their renewal period will be fixed to this value. This takes an integer number of seconds, or a string duration (e.g. "24h"). |
| `token_policies` | array | no | Comma-separated list of policies |
| `token_ttl` | integer | no | The initial ttl of the token to generate |
| `token_type` | string (default: default-service) | no | The type of token to generate, service or batch |
| `ttl` | integer | no | ⚠️ Deprecated. Use "token_ttl" instead. If this and "token_ttl" are both specified, only "token_ttl" will be used. |




#### Responses


**200**: OK



### DELETE /auth/{kubernetes_mount_path}/role/{name}

**Operation ID:** `kubernetes-delete-auth-role`


Register an role with the backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### GET /auth/{ldap_mount_path}/config

**Operation ID:** `ldap-read-auth-configuration`


Configure the LDAP server to connect to, along with its options.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /auth/{ldap_mount_path}/config

**Operation ID:** `ldap-configure-auth`


Configure the LDAP server to connect to, along with its options.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `anonymous_group_search` | boolean (default: False) | no | Use anonymous binds when performing LDAP group searches (if true the initial credentials will still be used for the initial connection test). |
| `binddn` | string | no | LDAP DN for searching for the user DN (optional) |
| `bindpass` | string | no | LDAP password for searching for the user DN (optional) |
| `case_sensitive_names` | boolean | no | If true, case sensitivity will be used when comparing usernames and groups for matching policies. |
| `certificate` | string | no | CA certificate to use when verifying LDAP server certificate, must be x509 PEM encoded (optional) |
| `client_tls_cert` | string | no | Client certificate to provide to the LDAP server, must be x509 PEM encoded (optional) |
| `client_tls_key` | string | no | Client certificate key to provide to the LDAP server, must be x509 PEM encoded (optional) |
| `connection_timeout` | integer (default: 30s) | no | Timeout, in seconds, when attempting to connect to the LDAP server before trying the next URL in the configuration. |
| `deny_null_bind` | boolean (default: True) | no | ⚠️ Deprecated. Denies an unauthenticated LDAP bind request if the user's password is empty; defaults to true |
| `dereference_aliases` | string (never, finding, searching, always) (default: never) | no | When aliases should be dereferenced on search operations. Accepted values are 'never', 'finding', 'searching', 'always'. Defaults to 'never'. |
| `discoverdn` | boolean | no | Use anonymous bind to discover the bind DN of a user (optional) |
| `enable_samaccountname_login` | boolean (default: False) | no | If true, matching sAMAccountName attribute values will be allowed to login when upndomain is defined. |
| `groupattr` | string (default: cn) | no | LDAP attribute to follow on objects returned by <groupfilter> in order to enumerate user group membership. Examples: "cn" or "memberOf", etc. Default: cn |
| `groupdn` | string | no | LDAP search base to use for group membership search (eg: ou=Groups,dc=example,dc=org) |
| `groupfilter` | string (default: (|(memberUid={{.Username}})(member={{.UserDN}})(uniqueMember={{.UserDN}}))) | no | Go template for querying group membership of user (optional) The template can access the following context variables: UserDN, Username Example: (&(objectClass=group)(member:1.2.840.113556.1.4.1941:={{.UserDN}})) Default: (|(memberUid={{.Username}})(member={{.UserDN}})(uniqueMember={{.UserDN}})) |
| `insecure_tls` | boolean | no | Skip LDAP server SSL Certificate verification - VERY insecure (optional) |
| `max_page_size` | integer (default: 0) | no | If set to a value greater than 0, the LDAP backend will use the LDAP server's paged search control to request pages of up to the given size. This can be used to avoid hitting the LDAP server's maximum result size limit. Otherwise, the LDAP backend will not use the paged search control. |
| `request_timeout` | integer (default: 90s) | no | Timeout, in seconds, for the connection when making requests against the server before returning back an error. |
| `starttls` | boolean | no | Issue a StartTLS command after establishing unencrypted connection (optional) |
| `tls_max_version` | string (tls10, tls11, tls12, tls13) (default: tls12) | no | Maximum TLS version to use. Accepted values are 'tls10', 'tls11', 'tls12' or 'tls13'. Defaults to 'tls12' |
| `tls_min_version` | string (tls10, tls11, tls12, tls13) (default: tls12) | no | Minimum TLS version to use. Accepted values are 'tls10', 'tls11', 'tls12' or 'tls13'. Defaults to 'tls12' |
| `token_bound_cidrs` | array | no | Comma separated string or JSON list of CIDR blocks. If set, specifies the blocks of IP addresses which are allowed to use the generated token. |
| `token_explicit_max_ttl` | integer | no | If set, tokens created via this role carry an explicit maximum TTL. During renewal, the current maximum TTL values of the role and the mount are not checked for changes, and any updates to these values will have no effect on the token being renewed. |
| `token_max_ttl` | integer | no | The maximum lifetime of the generated token |
| `token_no_default_policy` | boolean | no | If true, the 'default' policy will not automatically be added to generated tokens |
| `token_num_uses` | integer | no | The maximum number of times a token may be used, a value of zero means unlimited |
| `token_period` | integer | no | If set, tokens created via this role will have no max lifetime; instead, their renewal period will be fixed to this value. This takes an integer number of seconds, or a string duration (e.g. "24h"). |
| `token_policies` | array | no | Comma-separated list of policies. This will apply to all tokens generated by this auth method, in addition to any configured for specific users/groups. |
| `token_ttl` | integer | no | The initial ttl of the token to generate |
| `token_type` | string (default: default-service) | no | The type of token to generate, service or batch |
| `upndomain` | string | no | Enables userPrincipalDomain login with [username]@UPNDomain (optional) |
| `url` | string (default: ldap://127.0.0.1) | no | LDAP URL to connect to (default: ldap://127.0.0.1). Multiple URLs can be specified by concatenating them with commas; they will be tried in-order. |
| `use_pre111_group_cn_behavior` | boolean | no | In Vault 1.1.1 a fix for handling group CN values of different cases unfortunately introduced a regression that could cause previously defined groups to not be found due to a change in the resulting name. If set true, the pre-1.1.1 behavior for matching group CNs will be used. This is only needed in some upgrade scenarios for backwards compatibility. It is enabled by default if the config is upgraded but disabled by default on new configurations. |
| `use_token_groups` | boolean (default: False) | no | If true, use the Active Directory tokenGroups constructed attribute of the user to find the group memberships. This will find all security groups including nested ones. |
| `userattr` | string (default: cn) | no | Attribute used for users (default: cn) |
| `userdn` | string | no | LDAP domain to use for users (eg: ou=People,dc=example,dc=org) |
| `userfilter` | string (default: ({{.UserAttr}}={{.Username}})) | no | Go template for LDAP user search filer (optional) The template can access the following context variables: UserAttr, Username Default: ({{.UserAttr}}={{.Username}}) |
| `username_as_alias` | boolean (default: False) | no | If true, sets the alias name to the username |




#### Responses


**200**: OK



### GET /auth/{ldap_mount_path}/groups

**Operation ID:** `ldap-list-groups`


Manage additional groups for users allowed to authenticate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /auth/{ldap_mount_path}/groups/{name}

**Operation ID:** `ldap-read-group`


Manage additional groups for users allowed to authenticate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the LDAP group. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /auth/{ldap_mount_path}/groups/{name}

**Operation ID:** `ldap-write-group`


Manage additional groups for users allowed to authenticate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the LDAP group. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `policies` | array | no | Comma-separated list of policies associated to the group. |




#### Responses


**200**: OK



### DELETE /auth/{ldap_mount_path}/groups/{name}

**Operation ID:** `ldap-delete-group`


Manage additional groups for users allowed to authenticate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the LDAP group. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /auth/{ldap_mount_path}/login/{username}

**Operation ID:** `ldap-login`


Log in with a username and password.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `username` | string | path | yes | DN (distinguished name) to be used for login. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `password` | string | no | Password for this user. |




#### Responses


**200**: OK



### GET /auth/{ldap_mount_path}/users

**Operation ID:** `ldap-list-users`


Manage users allowed to authenticate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /auth/{ldap_mount_path}/users/{name}

**Operation ID:** `ldap-read-user`


Manage users allowed to authenticate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the LDAP user. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /auth/{ldap_mount_path}/users/{name}

**Operation ID:** `ldap-write-user`


Manage users allowed to authenticate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the LDAP user. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `groups` | array | no | Comma-separated list of additional groups associated with the user. |
| `policies` | array | no | Comma-separated list of policies associated with the user. |




#### Responses


**200**: OK



### DELETE /auth/{ldap_mount_path}/users/{name}

**Operation ID:** `ldap-delete-user`


Manage users allowed to authenticate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the LDAP user. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /auth/{userpass_mount_path}/login/{username}

**Operation ID:** `userpass-login`


Log in with a username and password.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `username` | string | path | yes | Username of the user. |
| `userpass_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `password` | string | no | Password for this user. |




#### Responses


**200**: OK



### POST /auth/{userpass_mount_path}/password_policy/{policy_name}

**Operation ID:** `userpass-update-policies_password`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `policy_name` | string | path | yes | Policy password name for userpass. |
| `userpass_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /auth/{userpass_mount_path}/users

**Operation ID:** `userpass-list-users`


Manage users allowed to authenticate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `userpass_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /auth/{userpass_mount_path}/users/{username}

**Operation ID:** `userpass-read-user`


Manage users allowed to authenticate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `username` | string | path | yes | Username for this user. |
| `userpass_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /auth/{userpass_mount_path}/users/{username}

**Operation ID:** `userpass-write-user`


Manage users allowed to authenticate.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `username` | string | path | yes | Username for this user. |
| `userpass_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bound_cidrs` | array | no | ⚠️ Deprecated. Use "token_bound_cidrs" instead. If this and "token_bound_cidrs" are both specified, only "token_bound_cidrs" will be used. |
| `max_ttl` | integer | no | ⚠️ Deprecated. Use "token_max_ttl" instead. If this and "token_max_ttl" are both specified, only "token_max_ttl" will be used. |
| `password` | string | no | Password for this user. |
| `policies` | array | no | ⚠️ Deprecated. Use "token_policies" instead. If this and "token_policies" are both specified, only "token_policies" will be used. |
| `token_bound_cidrs` | array | no | Comma separated string or JSON list of CIDR blocks. If set, specifies the blocks of IP addresses which are allowed to use the generated token. |
| `token_explicit_max_ttl` | integer | no | If set, tokens created via this role carry an explicit maximum TTL. During renewal, the current maximum TTL values of the role and the mount are not checked for changes, and any updates to these values will have no effect on the token being renewed. |
| `token_max_ttl` | integer | no | The maximum lifetime of the generated token |
| `token_no_default_policy` | boolean | no | If true, the 'default' policy will not automatically be added to generated tokens |
| `token_num_uses` | integer | no | The maximum number of times a token may be used, a value of zero means unlimited |
| `token_period` | integer | no | If set, tokens created via this role will have no max lifetime; instead, their renewal period will be fixed to this value. This takes an integer number of seconds, or a string duration (e.g. "24h"). |
| `token_policies` | array | no | Comma-separated list of policies |
| `token_ttl` | integer | no | The initial ttl of the token to generate |
| `token_type` | string (default: default-service) | no | The type of token to generate, service or batch |
| `ttl` | integer | no | ⚠️ Deprecated. Use "token_ttl" instead. If this and "token_ttl" are both specified, only "token_ttl" will be used. |




#### Responses


**200**: OK



### DELETE /auth/{userpass_mount_path}/users/{username}

**Operation ID:** `userpass-delete-user`


Manage users allowed to authenticate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `username` | string | path | yes | Username for this user. |
| `userpass_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /auth/{userpass_mount_path}/users/{username}/password

**Operation ID:** `userpass-reset-password`


Reset user's password.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `username` | string | path | yes | Username for this user. |
| `userpass_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `password` | string | no | Password for this user. |




#### Responses


**200**: OK



### POST /auth/{userpass_mount_path}/users/{username}/policies

**Operation ID:** `userpass-update-policies`


Update the policies associated with the username.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `username` | string | path | yes | Username for this user. |
| `userpass_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `policies` | array | no | ⚠️ Deprecated. Use "token_policies" instead. If this and "token_policies" are both specified, only "token_policies" will be used. |
| `token_policies` | array | no | Comma-separated list of policies |




#### Responses


**200**: OK



### GET /auth/{webauthn_mount_path}/config

**Operation ID:** `webauthn-read-config`


Read WebAuthn configuration


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `webauthn_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /auth/{webauthn_mount_path}/config

**Operation ID:** `webauthn-write-config`


Configure WebAuthn backend


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `webauthn_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `auto_registration` | boolean | no | If true (default), new users can self-register. If false, only pre-created users (via user/ path) can register. |
| `rp_display_name` | string | no | Human-readable name for the Relying Party. |
| `rp_id` | string | no | Relying Party ID (e.g. localhost or your domain). Must match the origin's host. |
| `rp_origins` | array | no | Allowed origins for WebAuthn (e.g. https://vault.example.com, http://localhost:8200). |




#### Responses


**200**: OK



### POST /auth/{webauthn_mount_path}/login/begin

**Operation ID:** `webauthn-write-login-begin`


Start WebAuthn login


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `webauthn_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `username` | string | no | Username to authenticate (omit for discoverable/passkey flow: browser will show passkey picker) |




#### Responses


**200**: OK



### POST /auth/{webauthn_mount_path}/login/finish

**Operation ID:** `webauthn-write-login-finish`


Finish WebAuthn login


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `webauthn_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `credential` | object | no | Assertion response from the authenticator (PublicKeyCredential with response) |
| `username` | string | no | Username (omit if using discoverable flow; user is identified from assertion userHandle) |




#### Responses


**200**: OK



### POST /auth/{webauthn_mount_path}/register/begin

**Operation ID:** `webauthn-write-register-begin`


Start WebAuthn registration


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `webauthn_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `username` | string | no | Username to register for WebAuthn |




#### Responses


**200**: OK



### POST /auth/{webauthn_mount_path}/register/finish

**Operation ID:** `webauthn-write-register-finish`


Finish WebAuthn registration


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `webauthn_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `credential` | object | no | Credential creation response from the authenticator (PublicKeyCredential with response) |
| `username` | string | no | Username being registered |




#### Responses


**200**: OK



### GET /auth/{webauthn_mount_path}/user

**Operation ID:** `webauthn-list-user`


List registered users


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `webauthn_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /auth/{webauthn_mount_path}/user/{name}

**Operation ID:** `webauthn-read-user-name`


Read a registered user


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Username |
| `webauthn_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /auth/{webauthn_mount_path}/user/{name}

**Operation ID:** `webauthn-write-user-name`


Update a user


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Username |
| `webauthn_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `display_name` | string | no | Display name for the user (defaults to username) |
| `token_bound_cidrs` | array | no | Comma separated string or JSON list of CIDR blocks. If set, specifies the blocks of IP addresses which are allowed to use the generated token. |
| `token_max_ttl` | integer | no | The maximum lifetime of the generated token |
| `token_no_default_policy` | boolean | no | If true, the 'default' policy will not automatically be added to generated tokens |
| `token_period` | integer | no | If set, tokens created via this role will have no max lifetime; instead, their renewal period will be fixed to this value. This takes an integer number of seconds, or a string duration (e.g. "24h"). |
| `token_policies` | array | no | Comma-separated list of policies |
| `token_ttl` | integer | no | The initial ttl of the token to generate |




#### Responses


**200**: OK



### DELETE /auth/{webauthn_mount_path}/user/{name}

**Operation ID:** `webauthn-delete-user-name`


Delete a registered user


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Username |
| `webauthn_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### DELETE /auth/{webauthn_mount_path}/user/{name}/credential/{credential_id}

**Operation ID:** `webauthn-delete-user-name-credential-credential_id`


Remove a credential from a user


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `credential_id` | string | path | yes | Credential ID (base64url encoded) |
| `name` | string | path | yes | Username |
| `webauthn_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /auth/{webauthn_mount_path}/user/{name}/policies

**Operation ID:** `webauthn-write-user-name-policies`


Update user token policies


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Username |
| `webauthn_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token_policies` | array | no | Comma-separated list of policies for the generated token |




#### Responses


**200**: OK



{% endraw %}
