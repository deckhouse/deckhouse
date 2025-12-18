---
title: "API"
permalink: en/stronghold/documentation/api/openapi.html
search: true
sitemap_include: false
description: API reference
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
| `bound_audiences` | array | no | Comma-separated list of 'aud' claims that are valid for login. Any match is sufficient |
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
| `deny_null_bind` | boolean (default: True) | no | Denies an unauthenticated LDAP bind request if the user's password is empty; defaults to true |
| `dereference_aliases` | string (never, finding, searching, always) (default: never) | no | When aliases should be dereferenced on search operations. Accepted values are 'never', 'finding', 'searching', 'always'. Defaults to 'never'. |
| `discoverdn` | boolean | no | Use anonymous bind to discover the bind DN of a user (optional) |
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




---


## identity


### POST /identity/alias

**Operation ID:** `alias-create`


Create a new alias.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `canonical_id` | string | no | Entity ID to which this alias belongs to |
| `entity_id` | string | no | Entity ID to which this alias belongs to. This field is deprecated in favor of 'canonical_id'. |
| `id` | string | no | ID of the alias |
| `mount_accessor` | string | no | Mount accessor to which this alias belongs to |
| `name` | string | no | Name of the alias |




#### Responses


**200**: OK



### GET /identity/alias/id

**Operation ID:** `alias-list-by-id`


List all the alias IDs.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/alias/id/{id}

**Operation ID:** `alias-read-by-id`


Update, read or delete an alias ID.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the alias |




#### Responses


**200**: OK



### POST /identity/alias/id/{id}

**Operation ID:** `alias-update-by-id`


Update, read or delete an alias ID.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the alias |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `canonical_id` | string | no | Entity ID to which this alias should be tied to |
| `entity_id` | string | no | Entity ID to which this alias should be tied to. This field is deprecated in favor of 'canonical_id'. |
| `mount_accessor` | string | no | Mount accessor to which this alias belongs to |
| `name` | string | no | Name of the alias |




#### Responses


**200**: OK



### DELETE /identity/alias/id/{id}

**Operation ID:** `alias-delete-by-id`


Update, read or delete an alias ID.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the alias |




#### Responses


**204**: empty body



### POST /identity/entity

**Operation ID:** `entity-create`


Create a new entity


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `disabled` | boolean | no | If set true, tokens tied to this identity will not be able to be used (but will not be revoked). |
| `id` | string | no | ID of the entity. If set, updates the corresponding existing entity. |
| `metadata` | object | no | Metadata to be associated with the entity. In CLI, this parameter can be repeated multiple times, and it all gets merged together. For example: stronghold <command> <path> metadata=key1=value1 metadata=key2=value2 |
| `name` | string | no | Name of the entity |
| `policies` | array | no | Policies to be tied to the entity. |




#### Responses


**200**: OK



### POST /identity/entity-alias

**Operation ID:** `entity-create-alias`


Create a new alias.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `canonical_id` | string | no | Entity ID to which this alias belongs |
| `custom_metadata` | object | no | User provided key-value pairs |
| `entity_id` | string | no | Entity ID to which this alias belongs. This field is deprecated, use canonical_id. |
| `id` | string | no | ID of the entity alias. If set, updates the corresponding entity alias. |
| `mount_accessor` | string | no | Mount accessor to which this alias belongs to; unused for a modify |
| `name` | string | no | Name of the alias; unused for a modify |




#### Responses


**200**: OK



### GET /identity/entity-alias/id

**Operation ID:** `entity-list-aliases-by-id`


List all the alias IDs.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/entity-alias/id/{id}

**Operation ID:** `entity-read-alias-by-id`


Update, read or delete an alias ID.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the alias |




#### Responses


**200**: OK



### POST /identity/entity-alias/id/{id}

**Operation ID:** `entity-update-alias-by-id`


Update, read or delete an alias ID.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the alias |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `canonical_id` | string | no | Entity ID to which this alias should be tied to |
| `custom_metadata` | object | no | User provided key-value pairs |
| `entity_id` | string | no | Entity ID to which this alias belongs to. This field is deprecated, use canonical_id. |
| `mount_accessor` | string | no | (Unused) |
| `name` | string | no | (Unused) |




#### Responses


**200**: OK



### DELETE /identity/entity-alias/id/{id}

**Operation ID:** `entity-delete-alias-by-id`


Update, read or delete an alias ID.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the alias |




#### Responses


**204**: empty body



### POST /identity/entity/batch-delete

**Operation ID:** `entity-batch-delete`


Delete all of the entities provided


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `entity_ids` | array | no | Entity IDs to delete |




#### Responses


**200**: OK



### GET /identity/entity/id

**Operation ID:** `entity-list-by-id`


List all the entity IDs


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/entity/id/{id}

**Operation ID:** `entity-read-by-id`


Update, read or delete an entity using entity ID


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the entity. If set, updates the corresponding existing entity. |




#### Responses


**200**: OK



### POST /identity/entity/id/{id}

**Operation ID:** `entity-update-by-id`


Update, read or delete an entity using entity ID


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the entity. If set, updates the corresponding existing entity. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `disabled` | boolean | no | If set true, tokens tied to this identity will not be able to be used (but will not be revoked). |
| `metadata` | object | no | Metadata to be associated with the entity. In CLI, this parameter can be repeated multiple times, and it all gets merged together. For example: stronghold <command> <path> metadata=key1=value1 metadata=key2=value2 |
| `name` | string | no | Name of the entity |
| `policies` | array | no | Policies to be tied to the entity. |




#### Responses


**200**: OK



### DELETE /identity/entity/id/{id}

**Operation ID:** `entity-delete-by-id`


Update, read or delete an entity using entity ID


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the entity. If set, updates the corresponding existing entity. |




#### Responses


**204**: empty body



### POST /identity/entity/merge

**Operation ID:** `entity-merge`


Merge two or more entities together


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `conflicting_alias_ids_to_keep` | array | no | Alias IDs to keep in case of conflicting aliases. Ignored if no conflicting aliases found |
| `force` | boolean | no | Setting this will follow the 'mine' strategy for merging MFA secrets. If there are secrets of the same type both in entities that are merged from and in entity into which all others are getting merged, secrets in the destination will be unaltered. If not set, this API will throw an error containing all the conflicts. |
| `from_entity_ids` | array | no | Entity IDs which need to get merged |
| `to_entity_id` | string | no | Entity ID into which all the other entities need to get merged |




#### Responses


**200**: OK



### GET /identity/entity/name

**Operation ID:** `entity-list-by-name`


List all the entity names


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/entity/name/{name}

**Operation ID:** `entity-read-by-name`


Update, read or delete an entity using entity name


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the entity |




#### Responses


**200**: OK



### POST /identity/entity/name/{name}

**Operation ID:** `entity-update-by-name`


Update, read or delete an entity using entity name


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the entity |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `disabled` | boolean | no | If set true, tokens tied to this identity will not be able to be used (but will not be revoked). |
| `id` | string | no | ID of the entity. If set, updates the corresponding existing entity. |
| `metadata` | object | no | Metadata to be associated with the entity. In CLI, this parameter can be repeated multiple times, and it all gets merged together. For example: stronghold <command> <path> metadata=key1=value1 metadata=key2=value2 |
| `policies` | array | no | Policies to be tied to the entity. |




#### Responses


**200**: OK



### DELETE /identity/entity/name/{name}

**Operation ID:** `entity-delete-by-name`


Update, read or delete an entity using entity name


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the entity |




#### Responses


**204**: empty body



### POST /identity/group

**Operation ID:** `group-create`


Create a new group.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `id` | string | no | ID of the group. If set, updates the corresponding existing group. |
| `member_entity_ids` | array | no | Entity IDs to be assigned as group members. |
| `member_group_ids` | array | no | Group IDs to be assigned as group members. |
| `metadata` | object | no | Metadata to be associated with the group. In CLI, this parameter can be repeated multiple times, and it all gets merged together. For example: stronghold <command> <path> metadata=key1=value1 metadata=key2=value2 |
| `name` | string | no | Name of the group. |
| `policies` | array | no | Policies to be tied to the group. |
| `type` | string | no | Type of the group, 'internal' or 'external'. Defaults to 'internal' |




#### Responses


**200**: OK



### POST /identity/group-alias

**Operation ID:** `group-create-alias`


Creates a new group alias, or updates an existing one.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `canonical_id` | string | no | ID of the group to which this is an alias. |
| `id` | string | no | ID of the group alias. |
| `mount_accessor` | string | no | Mount accessor to which this alias belongs to. |
| `name` | string | no | Alias of the group. |




#### Responses


**200**: OK



### GET /identity/group-alias/id

**Operation ID:** `group-list-aliases-by-id`


List all the group alias IDs.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/group-alias/id/{id}

**Operation ID:** `group-read-alias-by-id`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the group alias. |




#### Responses


**200**: OK



### POST /identity/group-alias/id/{id}

**Operation ID:** `group-update-alias-by-id`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the group alias. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `canonical_id` | string | no | ID of the group to which this is an alias. |
| `mount_accessor` | string | no | Mount accessor to which this alias belongs to. |
| `name` | string | no | Alias of the group. |




#### Responses


**200**: OK



### DELETE /identity/group-alias/id/{id}

**Operation ID:** `group-delete-alias-by-id`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the group alias. |




#### Responses


**204**: empty body



### GET /identity/group/id

**Operation ID:** `group-list-by-id`


List all the group IDs.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/group/id/{id}

**Operation ID:** `group-read-by-id`


Update or delete an existing group using its ID.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the group. If set, updates the corresponding existing group. |




#### Responses


**200**: OK



### POST /identity/group/id/{id}

**Operation ID:** `group-update-by-id`


Update or delete an existing group using its ID.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the group. If set, updates the corresponding existing group. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `member_entity_ids` | array | no | Entity IDs to be assigned as group members. |
| `member_group_ids` | array | no | Group IDs to be assigned as group members. |
| `metadata` | object | no | Metadata to be associated with the group. In CLI, this parameter can be repeated multiple times, and it all gets merged together. For example: stronghold <command> <path> metadata=key1=value1 metadata=key2=value2 |
| `name` | string | no | Name of the group. |
| `policies` | array | no | Policies to be tied to the group. |
| `type` | string | no | Type of the group, 'internal' or 'external'. Defaults to 'internal' |




#### Responses


**200**: OK



### DELETE /identity/group/id/{id}

**Operation ID:** `group-delete-by-id`


Update or delete an existing group using its ID.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the group. If set, updates the corresponding existing group. |




#### Responses


**204**: empty body



### GET /identity/group/name

**Operation ID:** `group-list-by-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/group/name/{name}

**Operation ID:** `group-read-by-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the group. |




#### Responses


**200**: OK



### POST /identity/group/name/{name}

**Operation ID:** `group-update-by-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the group. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `id` | string | no | ID of the group. If set, updates the corresponding existing group. |
| `member_entity_ids` | array | no | Entity IDs to be assigned as group members. |
| `member_group_ids` | array | no | Group IDs to be assigned as group members. |
| `metadata` | object | no | Metadata to be associated with the group. In CLI, this parameter can be repeated multiple times, and it all gets merged together. For example: stronghold <command> <path> metadata=key1=value1 metadata=key2=value2 |
| `policies` | array | no | Policies to be tied to the group. |
| `type` | string | no | Type of the group, 'internal' or 'external'. Defaults to 'internal' |




#### Responses


**200**: OK



### DELETE /identity/group/name/{name}

**Operation ID:** `group-delete-by-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the group. |




#### Responses


**204**: empty body



### POST /identity/lookup/entity

**Operation ID:** `entity-look-up`


Query entities based on various properties.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alias_id` | string | no | ID of the alias. |
| `alias_mount_accessor` | string | no | Accessor of the mount to which the alias belongs to. This should be supplied in conjunction with 'alias_name'. |
| `alias_name` | string | no | Name of the alias. This should be supplied in conjunction with 'alias_mount_accessor'. |
| `id` | string | no | ID of the entity. |
| `name` | string | no | Name of the entity. |




#### Responses


**200**: OK



### POST /identity/lookup/group

**Operation ID:** `group-look-up`


Query groups based on various properties.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alias_id` | string | no | ID of the alias. |
| `alias_mount_accessor` | string | no | Accessor of the mount to which the alias belongs to. This should be supplied in conjunction with 'alias_name'. |
| `alias_name` | string | no | Name of the alias. This should be supplied in conjunction with 'alias_mount_accessor'. |
| `id` | string | no | ID of the group. |
| `name` | string | no | Name of the group. |




#### Responses


**200**: OK



### GET /identity/mfa/login-enforcement

**Operation ID:** `mfa-list-login-enforcements`


List login enforcements


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/mfa/login-enforcement/{name}

**Operation ID:** `mfa-read-login-enforcement`


Read the current login enforcement


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name for this login enforcement configuration |




#### Responses


**200**: OK



### POST /identity/mfa/login-enforcement/{name}

**Operation ID:** `mfa-write-login-enforcement`


Create or update a login enforcement


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name for this login enforcement configuration |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `auth_method_accessors` | array | no | Array of auth mount accessor IDs |
| `auth_method_types` | array | no | Array of auth mount types |
| `identity_entity_ids` | array | no | Array of identity entity IDs |
| `identity_group_ids` | array | no | Array of identity group IDs |
| `mfa_method_ids` | array | yes | Array of Method IDs that determine what methods will be enforced |




#### Responses


**200**: OK



### DELETE /identity/mfa/login-enforcement/{name}

**Operation ID:** `mfa-delete-login-enforcement`


Delete a login enforcement


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name for this login enforcement configuration |




#### Responses


**204**: empty body



### GET /identity/mfa/method

**Operation ID:** `mfa-list-methods`


List MFA method configurations for all MFA methods


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/mfa/method/duo

**Operation ID:** `mfa-list-duo-methods`


List MFA method configurations for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/mfa/method/duo/{method_id}

**Operation ID:** `mfa-read-duo-method-configuration`


Read the current configuration for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Responses


**200**: OK



### POST /identity/mfa/method/duo/{method_id}

**Operation ID:** `mfa-configure-duo-method`


Update or create a configuration for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `api_hostname` | string | no | API host name for Duo. |
| `integration_key` | string | no | Integration key for Duo. |
| `method_name` | string | no | The unique name identifier for this MFA method. |
| `push_info` | string | no | Push information for Duo. |
| `secret_key` | string | no | Secret key for Duo. |
| `use_passcode` | boolean | no | If true, the user is reminded to use the passcode upon MFA validation. This option does not enforce using the passcode. Defaults to false. |
| `username_format` | string | no | A template string for mapping Identity names to MFA method names. Values to subtitute should be placed in {{}}. For example, "{{alias.name}}@example.com". Currently-supported mappings: alias.name: The name returned by the mount configured via the mount_accessor parameter If blank, the Alias's name field will be used as-is. |




#### Responses


**200**: OK



### DELETE /identity/mfa/method/duo/{method_id}

**Operation ID:** `mfa-delete-duo-method`


Delete a configuration for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Responses


**204**: empty body



### GET /identity/mfa/method/okta

**Operation ID:** `mfa-list-okta-methods`


List MFA method configurations for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/mfa/method/okta/{method_id}

**Operation ID:** `mfa-read-okta-method-configuration`


Read the current configuration for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Responses


**200**: OK



### POST /identity/mfa/method/okta/{method_id}

**Operation ID:** `mfa-configure-okta-method`


Update or create a configuration for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `api_token` | string | no | Okta API key. |
| `base_url` | string | no | The base domain to use for the Okta API. When not specified in the configuration, "okta.com" is used. |
| `method_name` | string | no | The unique name identifier for this MFA method. |
| `org_name` | string | no | Name of the organization to be used in the Okta API. |
| `primary_email` | boolean | no | If true, the username will only match the primary email for the account. Defaults to false. |
| `production` | boolean | no | (DEPRECATED) Use base_url instead. |
| `username_format` | string | no | A template string for mapping Identity names to MFA method names. Values to substitute should be placed in {{}}. For example, "{{entity.name}}@example.com". If blank, the Entity's name field will be used as-is. |




#### Responses


**200**: OK



### DELETE /identity/mfa/method/okta/{method_id}

**Operation ID:** `mfa-delete-okta-method`


Delete a configuration for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Responses


**204**: empty body



### GET /identity/mfa/method/pingid

**Operation ID:** `mfa-list-ping-id-methods`


List MFA method configurations for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/mfa/method/pingid/{method_id}

**Operation ID:** `mfa-read-ping-id-method-configuration`


Read the current configuration for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Responses


**200**: OK



### POST /identity/mfa/method/pingid/{method_id}

**Operation ID:** `mfa-configure-ping-id-method`


Update or create a configuration for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `method_name` | string | no | The unique name identifier for this MFA method. |
| `settings_file_base64` | string | no | The settings file provided by Ping, Base64-encoded. This must be a settings file suitable for third-party clients, not the PingID SDK or PingFederate. |
| `username_format` | string | no | A template string for mapping Identity names to MFA method names. Values to subtitute should be placed in {{}}. For example, "{{alias.name}}@example.com". Currently-supported mappings: alias.name: The name returned by the mount configured via the mount_accessor parameter If blank, the Alias's name field will be used as-is. |




#### Responses


**200**: OK



### DELETE /identity/mfa/method/pingid/{method_id}

**Operation ID:** `mfa-delete-ping-id-method`


Delete a configuration for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Responses


**204**: empty body



### GET /identity/mfa/method/totp

**Operation ID:** `mfa-list-totp-methods`


List MFA method configurations for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### POST /identity/mfa/method/totp/admin-destroy

**Operation ID:** `mfa-admin-destroy-totp-secret`


Destroys a TOTP secret for the given MFA method ID on the given entity


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `entity_id` | string | yes | Identifier of the entity from which the MFA method secret needs to be removed. |
| `method_id` | string | yes | The unique identifier for this MFA method. |




#### Responses


**200**: OK



### POST /identity/mfa/method/totp/admin-generate

**Operation ID:** `mfa-admin-generate-totp-secret`


Update or create TOTP secret for the given method ID on the given entity.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `entity_id` | string | yes | Entity ID on which the generated secret needs to get stored. |
| `method_id` | string | yes | The unique identifier for this MFA method. |




#### Responses


**200**: OK



### POST /identity/mfa/method/totp/generate

**Operation ID:** `mfa-generate-totp-secret`


Update or create TOTP secret for the given method ID on the given entity.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `method_id` | string | yes | The unique identifier for this MFA method. |




#### Responses


**200**: OK



### GET /identity/mfa/method/totp/{method_id}

**Operation ID:** `mfa-read-totp-method-configuration`


Read the current configuration for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Responses


**200**: OK



### POST /identity/mfa/method/totp/{method_id}

**Operation ID:** `mfa-configure-totp-method`


Update or create a configuration for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm` | string (default: SHA1) | no | The hashing algorithm used to generate the TOTP token. Options include SHA1, SHA256 and SHA512. |
| `digits` | integer (default: 6) | no | The number of digits in the generated TOTP token. This value can either be 6 or 8. |
| `issuer` | string | no | The name of the key's issuing organization. |
| `key_size` | integer (default: 20) | no | Determines the size in bytes of the generated key. |
| `max_validation_attempts` | integer | no | Max number of allowed validation attempts. |
| `method_name` | string | no | The unique name identifier for this MFA method. |
| `period` | integer (default: 30) | no | The length of time used to generate a counter for the TOTP token calculation. |
| `qr_size` | integer (default: 200) | no | The pixel size of the generated square QR code. |
| `skew` | integer (default: 1) | no | The number of delay periods that are allowed when validating a TOTP token. This value can either be 0 or 1. |




#### Responses


**200**: OK



### DELETE /identity/mfa/method/totp/{method_id}

**Operation ID:** `mfa-delete-totp-method`


Delete a configuration for the given MFA method


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Responses


**204**: empty body



### GET /identity/mfa/method/{method_id}

**Operation ID:** `mfa-read-method-configuration`


Read the current configuration for the given ID regardless of the MFA method type


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `method_id` | string | path | yes | The unique identifier for this MFA method. |




#### Responses


**200**: OK



### GET /identity/oidc/.well-known/keys

**Operation ID:** `oidc-read-public-keys`


Retrieve public keys


**Available without authentication:** yes


#### Responses


**200**: OK



### GET /identity/oidc/.well-known/openid-configuration

**Operation ID:** `oidc-read-open-id-configuration`


Query OIDC configurations


**Available without authentication:** yes


#### Responses


**200**: OK



### GET /identity/oidc/assignment

**Operation ID:** `oidc-list-assignments`


List OIDC assignments


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/oidc/assignment/{name}

**Operation ID:** `oidc-read-assignment`


CRUD operations for OIDC assignments.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the assignment |




#### Responses


**200**: OK



### POST /identity/oidc/assignment/{name}

**Operation ID:** `oidc-write-assignment`


CRUD operations for OIDC assignments.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the assignment |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `entity_ids` | array | no | Comma separated string or array of identity entity IDs |
| `group_ids` | array | no | Comma separated string or array of identity group IDs |




#### Responses


**200**: OK



### DELETE /identity/oidc/assignment/{name}

**Operation ID:** `oidc-delete-assignment`


CRUD operations for OIDC assignments.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the assignment |




#### Responses


**204**: empty body



### GET /identity/oidc/client

**Operation ID:** `oidc-list-clients`


List OIDC clients


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/oidc/client/{name}

**Operation ID:** `oidc-read-client`


CRUD operations for OIDC clients.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the client. |




#### Responses


**200**: OK



### POST /identity/oidc/client/{name}

**Operation ID:** `oidc-write-client`


CRUD operations for OIDC clients.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the client. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `access_token_ttl` | integer (default: 24h) | no | The time-to-live for access tokens obtained by the client. |
| `assignments` | array | no | Comma separated string or array of assignment resources. |
| `client_type` | string (default: confidential) | no | The client type based on its ability to maintain confidentiality of credentials. The following client types are supported: 'confidential', 'public'. Defaults to 'confidential'. |
| `id_token_ttl` | integer (default: 24h) | no | The time-to-live for ID tokens obtained by the client. |
| `key` | string (default: default) | no | A reference to a named key resource. Cannot be modified after creation. Defaults to the 'default' key. |
| `redirect_uris` | array | no | Comma separated string or array of redirect URIs used by the client. One of these values must exactly match the redirect_uri parameter value used in each authentication request. |




#### Responses


**200**: OK



### DELETE /identity/oidc/client/{name}

**Operation ID:** `oidc-delete-client`


CRUD operations for OIDC clients.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the client. |




#### Responses


**204**: empty body



### GET /identity/oidc/config

**Operation ID:** `oidc-read-configuration`


OIDC configuration


#### Responses


**200**: OK



### POST /identity/oidc/config

**Operation ID:** `oidc-configure`


OIDC configuration


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `issuer` | string | no | Issuer URL to be used in the iss claim of the token. If not set, app_addr will be used. |




#### Responses


**200**: OK



### POST /identity/oidc/introspect

**Operation ID:** `oidc-introspect`


Verify the authenticity of an OIDC token


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `client_id` | string | no | Optional client_id to verify |
| `token` | string | no | Token to verify |




#### Responses


**200**: OK



### GET /identity/oidc/key

**Operation ID:** `oidc-list-keys`


List OIDC keys


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/oidc/key/{name}

**Operation ID:** `oidc-read-key`


CRUD operations for OIDC keys.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |




#### Responses


**200**: OK



### POST /identity/oidc/key/{name}

**Operation ID:** `oidc-write-key`


CRUD operations for OIDC keys.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm` | string (default: RS256) | no | Signing algorithm to use. This will default to RS256. |
| `allowed_client_ids` | array | no | Comma separated string or array of role client ids allowed to use this key for signing. If empty no roles are allowed. If "*" all roles are allowed. |
| `rotation_period` | integer (default: 24h) | no | How often to generate a new keypair. |
| `verification_ttl` | integer (default: 24h) | no | Controls how long the public portion of a key will be available for verification after being rotated. |




#### Responses


**200**: OK



### DELETE /identity/oidc/key/{name}

**Operation ID:** `oidc-delete-key`


CRUD operations for OIDC keys.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |




#### Responses


**204**: empty body



### POST /identity/oidc/key/{name}/rotate

**Operation ID:** `oidc-rotate-key`


Rotate a named OIDC key.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `verification_ttl` | integer | no | Controls how long the public portion of a key will be available for verification after being rotated. Setting verification_ttl here will override the verification_ttl set on the key. |




#### Responses


**200**: OK



### GET /identity/oidc/provider

**Operation ID:** `oidc-list-providers`


List OIDC providers


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `allowed_client_id` | string | query | no | Filters the list of OIDC providers to those that allow the given client ID in their set of allowed_client_ids. |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/oidc/provider/{name}

**Operation ID:** `oidc-read-provider`


CRUD operations for OIDC providers.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the provider |




#### Responses


**200**: OK



### POST /identity/oidc/provider/{name}

**Operation ID:** `oidc-write-provider`


CRUD operations for OIDC providers.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the provider |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allowed_client_ids` | array | no | The client IDs that are permitted to use the provider |
| `issuer` | string | no | Specifies what will be used for the iss claim of ID tokens. |
| `scopes_supported` | array | no | The scopes supported for requesting on the provider |




#### Responses


**200**: OK



### DELETE /identity/oidc/provider/{name}

**Operation ID:** `oidc-delete-provider`


CRUD operations for OIDC providers.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the provider |




#### Responses


**204**: empty body



### GET /identity/oidc/provider/{name}/.well-known/keys

**Operation ID:** `oidc-read-provider-public-keys`


Retrieve public keys


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the provider |




#### Responses


**200**: OK



### GET /identity/oidc/provider/{name}/.well-known/openid-configuration

**Operation ID:** `oidc-read-provider-open-id-configuration`


Query OIDC configurations


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the provider |




#### Responses


**200**: OK



### GET /identity/oidc/provider/{name}/authorize

**Operation ID:** `oidc-provider-authorize`


Provides the OIDC Authorization Endpoint.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the provider |




#### Responses


**200**: OK



### POST /identity/oidc/provider/{name}/authorize

**Operation ID:** `oidc-provider-authorize-with-parameters`


Provides the OIDC Authorization Endpoint.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the provider |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `client_id` | string | yes | The ID of the requesting client. |
| `code_challenge` | string | no | The code challenge derived from the code verifier. |
| `code_challenge_method` | string (default: plain) | no | The method that was used to derive the code challenge. The following methods are supported: 'S256', 'plain'. Defaults to 'plain'. |
| `max_age` | integer | no | The allowable elapsed time in seconds since the last time the end-user was actively authenticated. |
| `nonce` | string | no | The value that will be returned in the ID token nonce claim after a token exchange. |
| `redirect_uri` | string | yes | The redirection URI to which the response will be sent. |
| `response_type` | string | yes | The OIDC authentication flow to be used. The following response types are supported: 'code' |
| `scope` | string | yes | A space-delimited, case-sensitive list of scopes to be requested. The 'openid' scope is required. |
| `state` | string | no | The value used to maintain state between the authentication request and client. |




#### Responses


**200**: OK



### POST /identity/oidc/provider/{name}/token

**Operation ID:** `oidc-provider-token`


Provides the OIDC Token Endpoint.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the provider |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `client_id` | string | no | The ID of the requesting client. |
| `client_secret` | string | no | The secret of the requesting client. |
| `code` | string | yes | The authorization code received from the provider's authorization endpoint. |
| `code_verifier` | string | no | The code verifier associated with the authorization code. |
| `grant_type` | string | yes | The authorization grant type. The following grant types are supported: 'authorization_code'. |
| `redirect_uri` | string | yes | The callback location where the authentication response was sent. |




#### Responses


**200**: OK



### GET /identity/oidc/provider/{name}/userinfo

**Operation ID:** `oidc-provider-user-info`


Provides the OIDC UserInfo Endpoint.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the provider |




#### Responses


**200**: OK



### POST /identity/oidc/provider/{name}/userinfo

**Operation ID:** `oidc-provider-user-info2`


Provides the OIDC UserInfo Endpoint.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the provider |




#### Responses


**200**: OK



### GET /identity/oidc/role

**Operation ID:** `oidc-list-roles`


List configured OIDC roles


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/oidc/role/{name}

**Operation ID:** `oidc-read-role`


CRUD operations on OIDC Roles


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |




#### Responses


**200**: OK



### POST /identity/oidc/role/{name}

**Operation ID:** `oidc-write-role`


CRUD operations on OIDC Roles


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `client_id` | string | no | Optional client_id |
| `key` | string | yes | The OIDC key to use for generating tokens. The specified key must already exist. |
| `template` | string | no | The template string to use for generating tokens. This may be in string-ified JSON or base64 format. |
| `ttl` | integer (default: 24h) | no | TTL of the tokens generated against the role. |




#### Responses


**200**: OK



### DELETE /identity/oidc/role/{name}

**Operation ID:** `oidc-delete-role`


CRUD operations on OIDC Roles


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |




#### Responses


**204**: empty body



### GET /identity/oidc/scope

**Operation ID:** `oidc-list-scopes`


List OIDC scopes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/oidc/scope/{name}

**Operation ID:** `oidc-read-scope`


CRUD operations for OIDC scopes.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the scope |




#### Responses


**200**: OK



### POST /identity/oidc/scope/{name}

**Operation ID:** `oidc-write-scope`


CRUD operations for OIDC scopes.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the scope |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `description` | string | no | The description of the scope |
| `template` | string | no | The template string to use for the scope. This may be in string-ified JSON or base64 format. |




#### Responses


**200**: OK



### DELETE /identity/oidc/scope/{name}

**Operation ID:** `oidc-delete-scope`


CRUD operations for OIDC scopes.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the scope |




#### Responses


**204**: empty body



### GET /identity/oidc/token/{name}

**Operation ID:** `oidc-generate-token`


Generate an OIDC token


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |




#### Responses


**200**: OK



### POST /identity/persona

**Operation ID:** `persona-create`


Create a new alias.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `entity_id` | string | no | Entity ID to which this persona belongs to |
| `id` | string | no | ID of the persona |
| `metadata` | object | no | Metadata to be associated with the persona. In CLI, this parameter can be repeated multiple times, and it all gets merged together. For example: stronghold <command> <path> metadata=key1=value1 metadata=key2=value2 |
| `mount_accessor` | string | no | Mount accessor to which this persona belongs to |
| `name` | string | no | Name of the persona |




#### Responses


**200**: OK



### GET /identity/persona/id

**Operation ID:** `persona-list-by-id`


List all the alias IDs.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /identity/persona/id/{id}

**Operation ID:** `persona-read-by-id`


Update, read or delete an alias ID.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the persona |




#### Responses


**200**: OK



### POST /identity/persona/id/{id}

**Operation ID:** `persona-update-by-id`


Update, read or delete an alias ID.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the persona |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `entity_id` | string | no | Entity ID to which this persona should be tied to |
| `metadata` | object | no | Metadata to be associated with the persona. In CLI, this parameter can be repeated multiple times, and it all gets merged together. For example: stronghold <command> <path> metadata=key1=value1 metadata=key2=value2 |
| `mount_accessor` | string | no | Mount accessor to which this persona belongs to |
| `name` | string | no | Name of the persona |




#### Responses


**200**: OK



### DELETE /identity/persona/id/{id}

**Operation ID:** `persona-delete-by-id`


Update, read or delete an alias ID.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `id` | string | path | yes | ID of the persona |




#### Responses


**204**: empty body




---


## secrets


### GET /cubbyhole/{path}

**Operation ID:** `cubbyhole-read`


Retrieve the secret at the specified location.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Specifies the path of the secret. |
| `list` | string | query | no | Return a list if `true` |




#### Responses


**200**: OK



### POST /cubbyhole/{path}

**Operation ID:** `cubbyhole-write`


Store a secret at the specified location.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Specifies the path of the secret. |




#### Responses


**200**: OK



### DELETE /cubbyhole/{path}

**Operation ID:** `cubbyhole-delete`


Deletes the secret at the specified location.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Specifies the path of the secret. |




#### Responses


**204**: empty body



### GET /{database_mount_path}/config

**Operation ID:** `database-list-connections`


Configure connection details to a database plugin.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /{database_mount_path}/config/{name}

**Operation ID:** `database-read-connection-configuration`


Configure connection details to a database plugin.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of this database connection |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{database_mount_path}/config/{name}

**Operation ID:** `database-configure-connection`


Configure connection details to a database plugin.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of this database connection |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allowed_roles` | array | no | Comma separated string or array of the role names allowed to get creds from this database connection. If empty no roles are allowed. If "*" all roles are allowed. |
| `password_policy` | string | no | Password policy to use when generating passwords. |
| `plugin_name` | string | no | The name of a builtin or previously registered plugin known to vault. This endpoint will create an instance of that plugin type. |
| `plugin_version` | string | no | The version of the plugin to use. |
| `root_rotation_statements` | array | no | Specifies the database statements to be executed to rotate the root user's credentials. See the plugin's API page for more information on support and formatting for this parameter. |
| `verify_connection` | boolean (default: True) | no | If true, the connection details are verified by actually connecting to the database. Defaults to true. |




#### Responses


**200**: OK



### DELETE /{database_mount_path}/config/{name}

**Operation ID:** `database-delete-connection-configuration`


Configure connection details to a database plugin.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of this database connection |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### GET /{database_mount_path}/creds/{name}

**Operation ID:** `database-generate-credentials`


Request database credentials for a certain role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{database_mount_path}/reset/{name}

**Operation ID:** `database-reset-connection`


Resets a database plugin.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of this database connection |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{database_mount_path}/roles

**Operation ID:** `database-list-roles`


Manage the roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /{database_mount_path}/roles/{name}

**Operation ID:** `database-read-role`


Manage the roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{database_mount_path}/roles/{name}

**Operation ID:** `database-write-role`


Manage the roles that can be created with this backend.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `creation_statements` | array | no | Specifies the database statements executed to create and configure a user. See the plugin's API page for more information on support and formatting for this parameter. |
| `credential_config` | object | no | The configuration for the given credential_type. |
| `credential_type` | string (default: password) | no | The type of credential to manage. Options include: 'password', 'rsa_private_key'. Defaults to 'password'. |
| `db_name` | string | no | Name of the database this role acts on. |
| `default_ttl` | integer | no | Default ttl for role. |
| `max_ttl` | integer | no | Maximum time a credential is valid for |
| `renew_statements` | array | no | Specifies the database statements to be executed to renew a user. Not every plugin type will support this functionality. See the plugin's API page for more information on support and formatting for this parameter. |
| `revocation_statements` | array | no | Specifies the database statements to be executed to revoke a user. See the plugin's API page for more information on support and formatting for this parameter. |
| `rollback_statements` | array | no | Specifies the database statements to be executed rollback a create operation in the event of an error. Not every plugin type will support this functionality. See the plugin's API page for more information on support and formatting for this parameter. |




#### Responses


**200**: OK



### DELETE /{database_mount_path}/roles/{name}

**Operation ID:** `database-delete-role`


Manage the roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /{database_mount_path}/rotate-role/{name}

**Operation ID:** `database-rotate-static-role-credentials`


Request to rotate the credentials for a static user account.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the static role |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{database_mount_path}/rotate-root/{name}

**Operation ID:** `database-rotate-root-credentials`


Request to rotate the root credentials for a certain database connection.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of this database connection |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{database_mount_path}/static-creds/{name}

**Operation ID:** `database-read-static-role-credentials`


Request database credentials for a certain static role. These credentials are
rotated periodically.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the static role. |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{database_mount_path}/static-roles

**Operation ID:** `database-list-static-roles`


Manage the static roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /{database_mount_path}/static-roles/{name}

**Operation ID:** `database-read-static-role`


Manage the static roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{database_mount_path}/static-roles/{name}

**Operation ID:** `database-write-static-role`


Manage the static roles that can be created with this backend.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `credential_config` | object | no | The configuration for the given credential_type. |
| `credential_type` | string (default: password) | no | The type of credential to manage. Options include: 'password', 'rsa_private_key'. Defaults to 'password'. |
| `db_name` | string | no | Name of the database this role acts on. |
| `rotation_period` | integer | no | Period for automatic credential rotation of the given username. Not valid unless used with "username". |
| `rotation_statements` | array | no | Specifies the database statements to be executed to rotate the accounts credentials. Not every plugin type will support this functionality. See the plugin's API page for more information on support and formatting for this parameter. |
| `username` | string | no | Name of the static user account for Vault to manage. Requires "rotation_period" to be specified |




#### Responses


**200**: OK



### DELETE /{database_mount_path}/static-roles/{name}

**Operation ID:** `database-delete-static-role`


Manage the static roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `database_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### GET /{kubernetes_mount_path}/check

**Operation ID:** `kubernetes-check-configuration`


Checks the Kubernetes configuration is valid.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{kubernetes_mount_path}/config

**Operation ID:** `kubernetes-read-configuration`


Configure the Kubernetes secret engine plugin.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{kubernetes_mount_path}/config

**Operation ID:** `kubernetes-configure`


Configure the Kubernetes secret engine plugin.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `disable_local_ca_jwt` | boolean (default: False) | no | Disable defaulting to the local CA certificate and service account JWT when running in a Kubernetes pod. |
| `kubernetes_ca_cert` | string | no | PEM encoded CA certificate to use to verify the Kubernetes API server certificate. Defaults to the local pod's CA if found. |
| `kubernetes_host` | string | no | Kubernetes API URL to connect to. Defaults to https://$KUBERNETES_SERVICE_HOST:KUBERNETES_SERVICE_PORT if those environment variables are set. |
| `service_account_jwt` | string | no | The JSON web token of the service account used by the secret engine to manage Kubernetes credentials. Defaults to the local pod's JWT if found. |




#### Responses


**200**: OK



### DELETE /{kubernetes_mount_path}/config

**Operation ID:** `kubernetes-delete-configuration`


Configure the Kubernetes secret engine plugin.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /{kubernetes_mount_path}/creds/{name}

**Operation ID:** `kubernetes-generate-credentials`


Request Kubernetes service account credentials for a given Vault role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the Vault role |
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `audiences` | array | no | The intended audiences of the generated credentials |
| `cluster_role_binding` | boolean | no | If true, generate a ClusterRoleBinding to grant permissions across the whole cluster instead of within a namespace. Requires the Vault role to have kubernetes_role_type set to ClusterRole. |
| `kubernetes_namespace` | string | yes | The name of the Kubernetes namespace in which to generate the credentials |
| `ttl` | integer | no | The TTL of the generated credentials |




#### Responses


**200**: OK



### GET /{kubernetes_mount_path}/roles

**Operation ID:** `kubernetes-list-roles`


List the existing roles in this secrets engine.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /{kubernetes_mount_path}/roles/{name}

**Operation ID:** `kubernetes-read-role`


Manage the roles that can be created with this secrets engine.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{kubernetes_mount_path}/roles/{name}

**Operation ID:** `kubernetes-write-role`


Manage the roles that can be created with this secrets engine.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allowed_kubernetes_namespace_selector` | string | no | A label selector for Kubernetes namespaces in which credentials can be generated. Accepts either a JSON or YAML object. If set with allowed_kubernetes_namespaces, the conditions are conjuncted. |
| `allowed_kubernetes_namespaces` | array | no | A list of the Kubernetes namespaces in which credentials can be generated. If set to "*" all namespaces are allowed. |
| `extra_annotations` | object | no | Additional annotations to apply to all generated Kubernetes objects. |
| `extra_labels` | object | no | Additional labels to apply to all generated Kubernetes objects. |
| `generated_role_rules` | string | no | The Role or ClusterRole rules to use when generating a role. Accepts either a JSON or YAML object. If set, the entire chain of Kubernetes objects will be generated. |
| `kubernetes_role_name` | string | no | The pre-existing Role or ClusterRole to bind a generated service account to. If set, Kubernetes token, service account, and role binding objects will be created. |
| `kubernetes_role_type` | string (default: Role) | no | Specifies whether the Kubernetes role is a Role or ClusterRole. |
| `name_template` | string | no | The name template to use when generating service accounts, roles and role bindings. If unset, a default template is used. |
| `service_account_name` | string | no | The pre-existing service account to generate tokens for. Mutually exclusive with all role parameters. If set, only a Kubernetes service account token will be created. |
| `token_default_audiences` | array | no | The default audiences for generated Kubernetes service account tokens. If not set or set to "", will use k8s cluster default. |
| `token_default_ttl` | integer | no | The default ttl for generated Kubernetes service account tokens. If not set or set to 0, will use system default. |
| `token_max_ttl` | integer | no | The maximum ttl for generated Kubernetes service account tokens. If not set or set to 0, will use system default. |




#### Responses


**200**: OK



### DELETE /{kubernetes_mount_path}/roles/{name}

**Operation ID:** `kubernetes-delete-role`


Manage the roles that can be created with this secrets engine.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |
| `kubernetes_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### GET /{kv_v1_mount_path}/{path}

**Operation ID:** `kv-v1-read`


Pass-through secret storage to the storage backend, allowing you to read/write arbitrary data into secret storage.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v1_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string | query | no | Return a list if `true` |




#### Responses


**200**: OK



### POST /{kv_v1_mount_path}/{path}

**Operation ID:** `kv-v1-write`


Pass-through secret storage to the storage backend, allowing you to read/write arbitrary data into secret storage.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v1_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### DELETE /{kv_v1_mount_path}/{path}

**Operation ID:** `kv-v1-delete`


Pass-through secret storage to the storage backend, allowing you to read/write arbitrary data into secret storage.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v1_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### GET /{kv_v2_mount_path}/config

**Operation ID:** `kv-v2-read-configuration`


Read the backend level settings.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kv_v2_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `cas_required` | boolean | no | If true, the backend will require the cas parameter to be set for each write |
| `delete_version_after` | integer | no | The length of time before a version is deleted. |
| `max_versions` | integer | no | The number of versions to keep for each key. |





### POST /{kv_v2_mount_path}/config

**Operation ID:** `kv-v2-configure`


Configure backend level settings that are applied to every key in the key-value store.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kv_v2_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `cas_required` | boolean | no | If true, the backend will require the cas parameter to be set for each write |
| `delete_version_after` | integer | no | If set, the length of time before a version is deleted. A negative duration disables the use of delete_version_after on all keys. A zero duration clears the current setting. Accepts a Go duration format string. |
| `max_versions` | integer | no | The number of versions to keep for each key. Defaults to 10 |




#### Responses


**204**: No Content



### GET /{kv_v2_mount_path}/data/{path}

**Operation ID:** `kv-v2-read`


Write, Patch, Read, and Delete data in the Key-Value Store.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v2_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `data` | object | no |  |
| `metadata` | object | no |  |





### POST /{kv_v2_mount_path}/data/{path}

**Operation ID:** `kv-v2-write`


Write, Patch, Read, and Delete data in the Key-Value Store.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v2_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `data` | object | no | The contents of the data map will be stored and returned on read. |
| `options` | object | no | Options for writing a KV entry. Set the "cas" value to use a Check-And-Set operation. If not set the write will be allowed. If set to 0 a write will only be allowed if the key doesn’t exist. If the index is non-zero the write will only be allowed if the key’s current version matches the version specified in the cas parameter. |
| `override_version` | integer | no | Only replication!!!!!!!! |
| `version` | integer | no | If provided during a read, the value at the version number will be returned |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `created_time` | string | no |  |
| `custom_metadata` | object | no |  |
| `deletion_time` | string | no |  |
| `destroyed` | boolean | no |  |
| `version` | integer | no |  |





### DELETE /{kv_v2_mount_path}/data/{path}

**Operation ID:** `kv-v2-delete`


Write, Patch, Read, and Delete data in the Key-Value Store.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v2_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### POST /{kv_v2_mount_path}/delete/{path}

**Operation ID:** `kv-v2-delete-versions`


Marks one or more versions as deleted in the KV store.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v2_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `versions` | array | no | The versions to be archived. The versioned data will not be deleted, but it will no longer be returned in normal get requests. |




#### Responses


**204**: No Content



### POST /{kv_v2_mount_path}/destroy/{path}

**Operation ID:** `kv-v2-destroy-versions`


Permanently removes one or more versions in the KV store


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v2_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `versions` | array | no | The versions to destroy. Their data will be permanently deleted. |




#### Responses


**204**: No Content



### GET /{kv_v2_mount_path}/metadata/{path}

**Operation ID:** `kv-v2-read-metadata`


Configures settings for the KV store


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v2_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string | query | no | Return a list if `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `cas_required` | boolean | no |  |
| `created_time` | string | no |  |
| `current_version` | integer | no |  |
| `custom_metadata` | object | no | User-provided key-value pairs that are used to describe arbitrary and version-agnostic information about a secret. |
| `delete_version_after` | integer | no | The length of time before a version is deleted. |
| `last_sync_time` | string | no |  |
| `max_versions` | integer | no | The number of versions to keep |
| `oldest_version` | integer | no |  |
| `updated_time` | string | no |  |
| `versions` | object | no |  |





### POST /{kv_v2_mount_path}/metadata/{path}

**Operation ID:** `kv-v2-write-metadata`


Configures settings for the KV store


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v2_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `cas_required` | boolean | no | If true the key will require the cas parameter to be set on all write requests. If false, the backend’s configuration will be used. |
| `custom_metadata` | object | no | User-provided key-value pairs that are used to describe arbitrary and version-agnostic information about a secret. |
| `delete_version_after` | integer | no | The length of time before a version is deleted. If not set, the backend's configured delete_version_after is used. Cannot be greater than the backend's delete_version_after. A zero duration clears the current setting. A negative duration will cause an error. |
| `max_versions` | integer | no | The number of versions to keep. If not set, the backend’s configured max version is used. |




#### Responses


**204**: No Content



### DELETE /{kv_v2_mount_path}/metadata/{path}

**Operation ID:** `kv-v2-delete-metadata`


Configures settings for the KV store


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v2_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### GET /{kv_v2_mount_path}/subkeys/{path}

**Operation ID:** `kv-v2-read-subkeys`


Read the structure of a secret entry from the Key-Value store with the values removed.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v2_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `metadata` | object | no |  |
| `subkeys` | object | no |  |





### POST /{kv_v2_mount_path}/undelete/{path}

**Operation ID:** `kv-v2-undelete-versions`


Undeletes one or more versions from the KV store.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Location of the secret. |
| `kv_v2_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `versions` | array | no | The versions to unarchive. The versions will be restored and their data will be returned on normal get requests. |




#### Responses


**204**: No Content



### GET /{ldap_mount_path}/config

**Operation ID:** `ldap-read-configuration`


Configure the LDAP secrets engine plugin.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{ldap_mount_path}/config

**Operation ID:** `ldap-configure`


Configure the LDAP secrets engine plugin.


**Creation supported:** yes


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
| `deny_null_bind` | boolean (default: True) | no | Denies an unauthenticated LDAP bind request if the user's password is empty; defaults to true |
| `dereference_aliases` | string (never, finding, searching, always) (default: never) | no | When aliases should be dereferenced on search operations. Accepted values are 'never', 'finding', 'searching', 'always'. Defaults to 'never'. |
| `discoverdn` | boolean | no | Use anonymous bind to discover the bind DN of a user (optional) |
| `groupattr` | string (default: cn) | no | LDAP attribute to follow on objects returned by <groupfilter> in order to enumerate user group membership. Examples: "cn" or "memberOf", etc. Default: cn |
| `groupdn` | string | no | LDAP search base to use for group membership search (eg: ou=Groups,dc=example,dc=org) |
| `groupfilter` | string (default: (|(memberUid={{.Username}})(member={{.UserDN}})(uniqueMember={{.UserDN}}))) | no | Go template for querying group membership of user (optional) The template can access the following context variables: UserDN, Username Example: (&(objectClass=group)(member:1.2.840.113556.1.4.1941:={{.UserDN}})) Default: (|(memberUid={{.Username}})(member={{.UserDN}})(uniqueMember={{.UserDN}})) |
| `insecure_tls` | boolean | no | Skip LDAP server SSL Certificate verification - VERY insecure (optional) |
| `length` | integer | no | ⚠️ Deprecated. The desired length of passwords that Vault generates. |
| `max_page_size` | integer (default: 0) | no | If set to a value greater than 0, the LDAP backend will use the LDAP server's paged search control to request pages of up to the given size. This can be used to avoid hitting the LDAP server's maximum result size limit. Otherwise, the LDAP backend will not use the paged search control. |
| `max_ttl` | integer | no | The maximum password time-to-live. |
| `password_policy` | string | no | Password policy to use to generate passwords |
| `request_timeout` | integer (default: 90s) | no | Timeout, in seconds, for the connection when making requests against the server before returning back an error. |
| `schema` | string (default: openldap) | no | The desired LDAP schema used when modifying user account passwords. |
| `starttls` | boolean | no | Issue a StartTLS command after establishing unencrypted connection (optional) |
| `tls_max_version` | string (tls10, tls11, tls12, tls13) (default: tls12) | no | Maximum TLS version to use. Accepted values are 'tls10', 'tls11', 'tls12' or 'tls13'. Defaults to 'tls12' |
| `tls_min_version` | string (tls10, tls11, tls12, tls13) (default: tls12) | no | Minimum TLS version to use. Accepted values are 'tls10', 'tls11', 'tls12' or 'tls13'. Defaults to 'tls12' |
| `ttl` | integer | no | The default password time-to-live. |
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



### DELETE /{ldap_mount_path}/config

**Operation ID:** `ldap-delete-configuration`


Configure the LDAP secrets engine plugin.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### GET /{ldap_mount_path}/creds/{name}

**Operation ID:** `ldap-request-dynamic-role-credentials`


Request LDAP credentials for a dynamic role. These credentials are created within the LDAP system when querying this endpoint.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the dynamic role. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{ldap_mount_path}/library

**Operation ID:** `ldap-library-list`


List the name of each set of service accounts currently stored.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### POST /{ldap_mount_path}/library/manage/{name}/check-in

**Operation ID:** `ldap-library-force-check-in`


Check service accounts in to the library.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the set. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `service_account_names` | array | no | The username/logon name for the service accounts to check in. |




#### Responses


**200**: OK



### GET /{ldap_mount_path}/library/{name}

**Operation ID:** `ldap-library-read`


Read a library set.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the set. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{ldap_mount_path}/library/{name}

**Operation ID:** `ldap-library-configure`


Update a library set.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the set. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `disable_check_in_enforcement` | boolean (default: False) | no | Disable the default behavior of requiring that check-ins are performed by the entity that checked them out. |
| `max_ttl` | integer (default: 86400) | no | In seconds, the max amount of time a check-out's renewals should last. Defaults to 24 hours. |
| `service_account_names` | array | no | The username/logon name for the service accounts with which this set will be associated. |
| `ttl` | integer (default: 86400) | no | In seconds, the amount of time a check-out should last. Defaults to 24 hours. |




#### Responses


**200**: OK



### DELETE /{ldap_mount_path}/library/{name}

**Operation ID:** `ldap-library-delete`


Delete a library set.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the set. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /{ldap_mount_path}/library/{name}/check-in

**Operation ID:** `ldap-library-check-in`


Check service accounts in to the library.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the set. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `service_account_names` | array | no | The username/logon name for the service accounts to check in. |




#### Responses


**200**: OK



### POST /{ldap_mount_path}/library/{name}/check-out

**Operation ID:** `ldap-library-check-out`


Check a service account out from the library.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the set |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ttl` | integer | no | The length of time before the check-out will expire, in seconds. |




#### Responses


**200**: OK



### GET /{ldap_mount_path}/library/{name}/status

**Operation ID:** `ldap-library-check-status`


Check the status of the service accounts in a library set.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the set. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{ldap_mount_path}/role

**Operation ID:** `ldap-list-dynamic-roles`


List all the dynamic roles Vault is currently managing in LDAP.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /{ldap_mount_path}/role/{name}

**Operation ID:** `ldap-read-dynamic-role`


Manage the static roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role (lowercase) |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{ldap_mount_path}/role/{name}

**Operation ID:** `ldap-write-dynamic-role`


Manage the static roles that can be created with this backend.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role (lowercase) |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `creation_ldif` | string | yes | LDIF string used to create new entities within the LDAP system. This LDIF can be templated. |
| `default_ttl` | integer | no | Default TTL for dynamic credentials |
| `deletion_ldif` | string | yes | LDIF string used to delete entities created within the LDAP system. This LDIF can be templated. |
| `max_ttl` | integer | no | Max TTL a dynamic credential can be extended to |
| `rollback_ldif` | string | no | LDIF string used to rollback changes in the event of a failure to create credentials. This LDIF can be templated. |
| `username_template` | string | no | The template used to create a username |




#### Responses


**200**: OK



### DELETE /{ldap_mount_path}/role/{name}

**Operation ID:** `ldap-delete-dynamic-role`


Manage the static roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role (lowercase) |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /{ldap_mount_path}/rotate-role/{name}

**Operation ID:** `ldap-rotate-static-role`


Request to rotate the credentials for a static user account.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the static role |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{ldap_mount_path}/rotate-root

**Operation ID:** `ldap-rotate-root-credentials`


Request to rotate the root credentials Vault uses for the LDAP administrator account.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{ldap_mount_path}/static-cred/{name}

**Operation ID:** `ldap-request-static-role-credentials`


Request LDAP credentials for a certain static role. These credentials are rotated periodically.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the static role. |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{ldap_mount_path}/static-role

**Operation ID:** `ldap-list-static-roles`


This path lists all the static roles Vault is currently managing within the LDAP system.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /{ldap_mount_path}/static-role/{name}

**Operation ID:** `ldap-read-static-role`


Manage the static roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{ldap_mount_path}/static-role/{name}

**Operation ID:** `ldap-write-static-role`


Manage the static roles that can be created with this backend.


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `dn` | string | no | The distinguished name of the entry to manage. |
| `rotation_period` | integer | no | Period for automatic credential rotation of the given entry. |
| `username` | string | no | The username/logon name for the entry with which this role will be associated. |




#### Responses


**200**: OK



### DELETE /{ldap_mount_path}/static-role/{name}

**Operation ID:** `ldap-delete-static-role`


Manage the static roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |
| `ldap_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /{pki_mount_path}/acme/account/{kid}

**Operation ID:** `pki-write-acme-account-kid`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kid` | string | path | yes | The key identifier provided by the CA |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/acme/authorization/{auth_id}

**Operation ID:** `pki-write-acme-authorization-auth_id`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `auth_id` | string | path | yes | ACME authorization identifier value |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/acme/challenge/{auth_id}/{challenge_type}

**Operation ID:** `pki-write-acme-challenge-auth_id-challenge_type`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `auth_id` | string | path | yes | ACME authorization identifier value |
| `challenge_type` | string | path | yes | ACME challenge type |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### GET /{pki_mount_path}/acme/directory

**Operation ID:** `pki-read-acme-directory`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{pki_mount_path}/acme/new-account

**Operation ID:** `pki-write-acme-new-account`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/acme/new-eab

**Operation ID:** `pki-generate-eab-key`


Generate external account bindings to be used for ACME


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `acme_directory` | string | no | The ACME directory to which the key belongs |
| `created_on` | string | no | An RFC3339 formatted date time when the EAB token was created |
| `id` | string | no | The EAB key identifier |
| `key` | string | no | The EAB hmac key |
| `key_type` | string | no | The EAB key type |





### GET /{pki_mount_path}/acme/new-nonce

**Operation ID:** `pki-read-acme-new-nonce`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{pki_mount_path}/acme/new-order

**Operation ID:** `pki-write-acme-new-order`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/acme/order/{order_id}

**Operation ID:** `pki-write-acme-order-order_id`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `order_id` | string | path | yes | The ACME order identifier to fetch |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/acme/order/{order_id}/cert

**Operation ID:** `pki-write-acme-order-order_id-cert`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `order_id` | string | path | yes | The ACME order identifier to fetch |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/acme/order/{order_id}/finalize

**Operation ID:** `pki-write-acme-order-order_id-finalize`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `order_id` | string | path | yes | The ACME order identifier to fetch |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/acme/orders

**Operation ID:** `pki-write-acme-orders`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/acme/revoke-cert

**Operation ID:** `pki-write-acme-revoke-cert`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### GET /{pki_mount_path}/ca

**Operation ID:** `pki-read-ca-der`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/ca/pem

**Operation ID:** `pki-read-ca-pem`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/ca_chain

**Operation ID:** `pki-read-ca-chain-pem`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/cert/ca_chain

**Operation ID:** `pki-read-cert-ca-chain`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/cert/crl

**Operation ID:** `pki-read-cert-crl`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/cert/delta-crl

**Operation ID:** `pki-read-cert-delta-crl`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/cert/unified-crl

**Operation ID:** `pki-read-cert-unified-crl`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/cert/unified-delta-crl

**Operation ID:** `pki-read-cert-unified-delta-crl`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/cert/{serial}

**Operation ID:** `pki-read-cert`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `serial` | string | path | yes | Certificate serial number, in colon- or hyphen-separated octal |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/cert/{serial}/raw

**Operation ID:** `pki-read-cert-raw-der`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `serial` | string | path | yes | Certificate serial number, in colon- or hyphen-separated octal |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/cert/{serial}/raw/pem

**Operation ID:** `pki-read-cert-raw-pem`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `serial` | string | path | yes | Certificate serial number, in colon- or hyphen-separated octal |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/certs

**Operation ID:** `pki-list-certs`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no | A list of keys |





### GET /{pki_mount_path}/certs/revocation-queue

**Operation ID:** `pki-list-certs-revocation-queue`


List all pending, cross-cluster revocations known to the local cluster.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /{pki_mount_path}/certs/revoked

**Operation ID:** `pki-list-revoked-certs`


List all revoked serial numbers within the local cluster


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no | List of Keys |





### GET /{pki_mount_path}/certs/unified-revoked

**Operation ID:** `pki-list-unified-revoked-certs`


List all revoked serial numbers within this cluster's unified storage area.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_info` | string | no | Key information |
| `keys` | array | no | List of Keys |





### GET /{pki_mount_path}/config/acme

**Operation ID:** `pki-read-acme-configuration`


Configuration of ACME Endpoints


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{pki_mount_path}/config/acme

**Operation ID:** `pki-configure-acme`


Configuration of ACME Endpoints


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allow_role_ext_key_usage` | boolean (default: False) | no | whether the ExtKeyUsage field from a role is used, defaults to false meaning that certificate will be signed with ServerAuth. |
| `allowed_issuers` | array (default: ['*']) | no | which issuers are allowed for use with ACME; by default, this will only be the primary (default) issuer |
| `allowed_roles` | array (default: ['*']) | no | which roles are allowed for use with ACME; by default via '*', these will be all roles including sign-verbatim; when concrete role names are specified, any default_directory_policy role must be included to allow usage of the default acme directories under /pki/acme/directory and /pki/issuer/:issuer_id/acme/directory. |
| `default_directory_policy` | string (default: sign-verbatim) | no | the policy to be used for non-role-qualified ACME requests; by default ACME issuance will be otherwise unrestricted, equivalent to the sign-verbatim endpoint; one may also specify a role to use as this policy, as "role:<role_name>", the specified role must be allowed by allowed_roles |
| `dns_resolver` | string (default: ) | no | DNS resolver to use for domain resolution on this mount. Defaults to using the default system resolver. Must be in the format <host>:<port>, with both parts mandatory. |
| `eab_policy` | string (default: always-required) | no | Specify the policy to use for external account binding behaviour, 'not-required', 'new-account-required' or 'always-required' |
| `enabled` | boolean (default: False) | no | whether ACME is enabled, defaults to false meaning that clusters will by default not get ACME support |




#### Responses


**200**: OK



### GET /{pki_mount_path}/config/auto-tidy

**Operation ID:** `pki-read-auto-tidy-configuration`


Modifies the current configuration for automatic tidy execution.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `acme_account_safety_buffer` | integer | no | Safety buffer after creation after which accounts lacking orders are revoked |
| `enabled` | boolean | no | Specifies whether automatic tidy is enabled or not |
| `interval_duration` | integer | no | Specifies the duration between automatic tidy operation |
| `issuer_safety_buffer` | integer | no | Issuer safety buffer |
| `maintain_stored_certificate_counts` | boolean | no |  |
| `pause_duration` | string | no | Duration to pause between tidying certificates |
| `publish_stored_certificate_count_metrics` | boolean | no |  |
| `revocation_queue_safety_buffer` | integer | no |  |
| `safety_buffer` | integer | no | Safety buffer time duration |
| `tidy_acme` | boolean | no | Tidy Unused Acme Accounts, and Orders |
| `tidy_cert_store` | boolean | no | Specifies whether to tidy up the certificate store |
| `tidy_cross_cluster_revoked_certs` | boolean | no |  |
| `tidy_expired_issuers` | boolean | no | Specifies whether tidy expired issuers |
| `tidy_move_legacy_ca_bundle` | boolean | no |  |
| `tidy_revocation_queue` | boolean | no |  |
| `tidy_revoked_cert_issuer_associations` | boolean | no | Specifies whether to associate revoked certificates with their corresponding issuers |
| `tidy_revoked_certs` | boolean | no | Specifies whether to remove all invalid and expired certificates from storage |





### POST /{pki_mount_path}/config/auto-tidy

**Operation ID:** `pki-configure-auto-tidy`


Modifies the current configuration for automatic tidy execution.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `acme_account_safety_buffer` | integer (default: 2592000) | no | The amount of time that must pass after creation that an account with no orders is marked revoked, and the amount of time after being marked revoked or deactivated. |
| `enabled` | boolean | no | Set to true to enable automatic tidy operations. |
| `interval_duration` | integer (default: 43200) | no | Interval at which to run an auto-tidy operation. This is the time between tidy invocations (after one finishes to the start of the next). Running a manual tidy will reset this duration. |
| `issuer_safety_buffer` | integer (default: 31536000) | no | The amount of extra time that must have passed beyond issuer's expiration before it is removed from the backend storage. Defaults to 8760 hours (1 year). |
| `maintain_stored_certificate_counts` | boolean (default: False) | no | This configures whether stored certificates are counted upon initialization of the backend, and whether during normal operation, a running count of certificates stored is maintained. |
| `pause_duration` | string (default: 0s) | no | The amount of time to wait between processing certificates. This allows operators to change the execution profile of tidy to take consume less resources by slowing down how long it takes to run. Note that the entire list of certificates will be stored in memory during the entire tidy operation, but resources to read/process/update existing entries will be spread out over a greater period of time. By default this is zero seconds. |
| `publish_stored_certificate_count_metrics` | boolean (default: False) | no | This configures whether the stored certificate count is published to the metrics consumer. It does not affect if the stored certificate count is maintained, and if maintained, it will be available on the tidy-status endpoint. |
| `revocation_queue_safety_buffer` | integer (default: 172800) | no | The amount of time that must pass from the cross-cluster revocation request being initiated to when it will be slated for removal. Setting this too low may remove valid revocation requests before the owning cluster has a chance to process them, especially if the cluster is offline. |
| `safety_buffer` | integer (default: 259200) | no | The amount of extra time that must have passed beyond certificate expiration before it is removed from the backend storage and/or revocation list. Defaults to 72 hours. |
| `tidy_acme` | boolean (default: False) | no | Set to true to enable tidying ACME accounts, orders and authorizations. ACME orders are tidied (deleted) safety_buffer after the certificate associated with them expires, or after the order and relevant authorizations have expired if no certificate was produced. Authorizations are tidied with the corresponding order. When a valid ACME Account is at least acme_account_safety_buffer old, and has no remaining orders associated with it, the account is marked as revoked. After another acme_account_safety_buffer has passed from the revocation or deactivation date, a revoked or deactivated ACME account is deleted. |
| `tidy_cert_store` | boolean | no | Set to true to enable tidying up the certificate store |
| `tidy_cross_cluster_revoked_certs` | boolean | no | Set to true to enable tidying up the cross-cluster revoked certificate store. Only runs on the active primary node. |
| `tidy_expired_issuers` | boolean | no | Set to true to automatically remove expired issuers past the issuer_safety_buffer. No keys will be removed as part of this operation. |
| `tidy_move_legacy_ca_bundle` | boolean | no | Set to true to move the legacy ca_bundle from /config/ca_bundle to /config/ca_bundle.bak. This prevents downgrades to pre-Vault 1.11 versions (as older PKI engines do not know about the new multi-issuer storage layout), but improves the performance on seal wrapped PKI mounts. This will only occur if at least issuer_safety_buffer time has occurred after the initial storage migration. This backup is saved in case of an issue in future migrations. Operators may consider removing it via sys/raw if they desire. The backup will be removed via a DELETE /root call, but note that this removes ALL issuers within the mount (and is thus not desirable in most operational scenarios). |
| `tidy_revocation_list` | boolean | no | Deprecated; synonym for 'tidy_revoked_certs |
| `tidy_revocation_queue` | boolean (default: False) | no | Set to true to remove stale revocation queue entries that haven't been confirmed by any active cluster. Only runs on the active primary node |
| `tidy_revoked_cert_issuer_associations` | boolean | no | Set to true to validate issuer associations on revocation entries. This helps increase the performance of CRL building and OCSP responses. |
| `tidy_revoked_certs` | boolean | no | Set to true to expire all revoked and expired certificates, removing them both from the CRL and from storage. The CRL will be rotated if this causes any values to be removed. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `acme_account_safety_buffer` | integer | no | Safety buffer after creation after which accounts lacking orders are revoked |
| `enabled` | boolean | no | Specifies whether automatic tidy is enabled or not |
| `interval_duration` | integer | no | Specifies the duration between automatic tidy operation |
| `issuer_safety_buffer` | integer | no | Issuer safety buffer |
| `maintain_stored_certificate_counts` | boolean | no |  |
| `pause_duration` | string | no | Duration to pause between tidying certificates |
| `publish_stored_certificate_count_metrics` | boolean | no |  |
| `revocation_queue_safety_buffer` | integer | no |  |
| `safety_buffer` | integer | no | Safety buffer time duration |
| `tidy_acme` | boolean | no | Tidy Unused Acme Accounts, and Orders |
| `tidy_cert_store` | boolean | no | Specifies whether to tidy up the certificate store |
| `tidy_cross_cluster_revoked_certs` | boolean | no | Tidy the cross-cluster revoked certificate store |
| `tidy_expired_issuers` | boolean | no | Specifies whether tidy expired issuers |
| `tidy_move_legacy_ca_bundle` | boolean | no |  |
| `tidy_revocation_queue` | boolean | no |  |
| `tidy_revoked_cert_issuer_associations` | boolean | no | Specifies whether to associate revoked certificates with their corresponding issuers |
| `tidy_revoked_certs` | boolean | no | Specifies whether to remove all invalid and expired certificates from storage |





### POST /{pki_mount_path}/config/ca

**Operation ID:** `pki-configure-ca`


Set the CA certificate and private key used for generated credentials.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `pem_bundle` | string | no | PEM-format, concatenated unencrypted secret key and certificate. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `existing_issuers` | array | no | Existing issuers specified as part of the import bundle of this request |
| `existing_keys` | array | no | Existing keys specified as part of the import bundle of this request |
| `imported_issuers` | array | no | Net-new issuers imported as a part of this request |
| `imported_keys` | array | no | Net-new keys imported as a part of this request |
| `mapping` | object | no | A mapping of issuer_id to key_id for all issuers included in this request |





### GET /{pki_mount_path}/config/cluster

**Operation ID:** `pki-read-cluster-configuration`


Set cluster-local configuration, including address to this PR cluster.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `aia_path` | string | no | Optional URI to this mount's AIA distribution point; may refer to an external non-Vault responder. This is for resolving AIA URLs and providing the {{cluster_aia_path}} template parameter and will not be used for other purposes. As such, unlike path above, this could safely be an insecure transit mechanism (like HTTP without TLS). For example: http://cdn.example.com/pr1/pki |
| `path` | string | no | Canonical URI to this mount on this performance replication cluster's external address. This is for resolving AIA URLs and providing the {{cluster_path}} template parameter but might be used for other purposes in the future. This should only point back to this particular PR replica and should not ever point to another PR cluster. It may point to any node in the PR replica, including standby nodes, and need not always point to the active node. For example: https://pr1.vault.example.com:8200/v1/pki |





### POST /{pki_mount_path}/config/cluster

**Operation ID:** `pki-configure-cluster`


Set cluster-local configuration, including address to this PR cluster.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `aia_path` | string | no | Optional URI to this mount's AIA distribution point; may refer to an external non-Vault responder. This is for resolving AIA URLs and providing the {{cluster_aia_path}} template parameter and will not be used for other purposes. As such, unlike path above, this could safely be an insecure transit mechanism (like HTTP without TLS). For example: http://cdn.example.com/pr1/pki |
| `path` | string | no | Canonical URI to this mount on this performance replication cluster's external address. This is for resolving AIA URLs and providing the {{cluster_path}} template parameter but might be used for other purposes in the future. This should only point back to this particular PR replica and should not ever point to another PR cluster. It may point to any node in the PR replica, including standby nodes, and need not always point to the active node. For example: https://pr1.vault.example.com:8200/v1/pki |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `aia_path` | string | no | Optional URI to this mount's AIA distribution point; may refer to an external non-Vault responder. This is for resolving AIA URLs and providing the {{cluster_aia_path}} template parameter and will not be used for other purposes. As such, unlike path above, this could safely be an insecure transit mechanism (like HTTP without TLS). For example: http://cdn.example.com/pr1/pki |
| `path` | string | no | Canonical URI to this mount on this performance replication cluster's external address. This is for resolving AIA URLs and providing the {{cluster_path}} template parameter but might be used for other purposes in the future. This should only point back to this particular PR replica and should not ever point to another PR cluster. It may point to any node in the PR replica, including standby nodes, and need not always point to the active node. For example: https://pr1.vault.example.com:8200/v1/pki |





### GET /{pki_mount_path}/config/crl

**Operation ID:** `pki-read-crl-configuration`


Configure the CRL expiration.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `auto_rebuild` | boolean | no | If set to true, enables automatic rebuilding of the CRL |
| `auto_rebuild_grace_period` | string | no | The time before the CRL expires to automatically rebuild it, when enabled. Must be shorter than the CRL expiry. Defaults to 12h. |
| `cross_cluster_revocation` | boolean | no | Whether to enable a global, cross-cluster revocation queue. Must be used with auto_rebuild=true. |
| `delta_rebuild_interval` | string | no | The time between delta CRL rebuilds if a new revocation has occurred. Must be shorter than the CRL expiry. Defaults to 15m. |
| `disable` | boolean | no | If set to true, disables generating the CRL entirely. |
| `enable_delta` | boolean | no | Whether to enable delta CRLs between authoritative CRL rebuilds |
| `expiry` | string | no | The amount of time the generated CRL should be valid; defaults to 72 hours |
| `ocsp_disable` | boolean | no | If set to true, ocsp unauthorized responses will be returned. |
| `ocsp_expiry` | string | no | The amount of time an OCSP response will be valid (controls the NextUpdate field); defaults to 12 hours |
| `unified_crl` | boolean | no | If set to true enables global replication of revocation entries, also enabling unified versions of OCSP and CRLs if their respective features are enabled. disable for CRLs and ocsp_disable for OCSP. |
| `unified_crl_on_existing_paths` | boolean | no | If set to true, existing CRL and OCSP paths will return the unified CRL instead of a response based on cluster-local data |





### POST /{pki_mount_path}/config/crl

**Operation ID:** `pki-configure-crl`


Configure the CRL expiration.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `auto_rebuild` | boolean | no | If set to true, enables automatic rebuilding of the CRL |
| `auto_rebuild_grace_period` | string (default: 12h) | no | The time before the CRL expires to automatically rebuild it, when enabled. Must be shorter than the CRL expiry. Defaults to 12h. |
| `cross_cluster_revocation` | boolean | no | Whether to enable a global, cross-cluster revocation queue. Must be used with auto_rebuild=true. |
| `delta_rebuild_interval` | string (default: 15m) | no | The time between delta CRL rebuilds if a new revocation has occurred. Must be shorter than the CRL expiry. Defaults to 15m. |
| `disable` | boolean | no | If set to true, disables generating the CRL entirely. |
| `enable_delta` | boolean | no | Whether to enable delta CRLs between authoritative CRL rebuilds |
| `expiry` | string (default: 72h) | no | The amount of time the generated CRL should be valid; defaults to 72 hours |
| `ocsp_disable` | boolean | no | If set to true, ocsp unauthorized responses will be returned. |
| `ocsp_expiry` | string (default: 1h) | no | The amount of time an OCSP response will be valid (controls the NextUpdate field); defaults to 12 hours |
| `unified_crl` | boolean (default: false) | no | If set to true enables global replication of revocation entries, also enabling unified versions of OCSP and CRLs if their respective features are enabled. disable for CRLs and ocsp_disable for OCSP. |
| `unified_crl_on_existing_paths` | boolean (default: false) | no | If set to true, existing CRL and OCSP paths will return the unified CRL instead of a response based on cluster-local data |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `auto_rebuild` | boolean | no | If set to true, enables automatic rebuilding of the CRL |
| `auto_rebuild_grace_period` | string (default: 12h) | no | The time before the CRL expires to automatically rebuild it, when enabled. Must be shorter than the CRL expiry. Defaults to 12h. |
| `cross_cluster_revocation` | boolean | no | Whether to enable a global, cross-cluster revocation queue. Must be used with auto_rebuild=true. |
| `delta_rebuild_interval` | string (default: 15m) | no | The time between delta CRL rebuilds if a new revocation has occurred. Must be shorter than the CRL expiry. Defaults to 15m. |
| `disable` | boolean | no | If set to true, disables generating the CRL entirely. |
| `enable_delta` | boolean | no | Whether to enable delta CRLs between authoritative CRL rebuilds |
| `expiry` | string (default: 72h) | no | The amount of time the generated CRL should be valid; defaults to 72 hours |
| `ocsp_disable` | boolean | no | If set to true, ocsp unauthorized responses will be returned. |
| `ocsp_expiry` | string (default: 1h) | no | The amount of time an OCSP response will be valid (controls the NextUpdate field); defaults to 12 hours |
| `unified_crl` | boolean | no | If set to true enables global replication of revocation entries, also enabling unified versions of OCSP and CRLs if their respective features are enabled. disable for CRLs and ocsp_disable for OCSP. |
| `unified_crl_on_existing_paths` | boolean | no | If set to true, existing CRL and OCSP paths will return the unified CRL instead of a response based on cluster-local data |





### GET /{pki_mount_path}/config/issuers

**Operation ID:** `pki-read-issuers-configuration`


Read and set the default issuer certificate for signing.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `default` | string | no | Reference (name or identifier) to the default issuer. |
| `default_follows_latest_issuer` | boolean | no | Whether the default issuer should automatically follow the latest generated or imported issuer. Defaults to false. |





### POST /{pki_mount_path}/config/issuers

**Operation ID:** `pki-configure-issuers`


Read and set the default issuer certificate for signing.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `default` | string | no | Reference (name or identifier) to the default issuer. |
| `default_follows_latest_issuer` | boolean (default: False) | no | Whether the default issuer should automatically follow the latest generated or imported issuer. Defaults to false. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `default` | string | no | Reference (name or identifier) to the default issuer. |
| `default_follows_latest_issuer` | boolean | no | Whether the default issuer should automatically follow the latest generated or imported issuer. Defaults to false. |





### GET /{pki_mount_path}/config/keys

**Operation ID:** `pki-read-keys-configuration`


Read and set the default key used for signing


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `default` | string | no | Reference (name or identifier) to the default issuer. |





### POST /{pki_mount_path}/config/keys

**Operation ID:** `pki-configure-keys`


Read and set the default key used for signing


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `default` | string | no | Reference (name or identifier) of the default key. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `default` | string | no | Reference (name or identifier) to the default issuer. |





### GET /{pki_mount_path}/config/urls

**Operation ID:** `pki-read-urls-configuration`


Set the URLs for the issuing CA, CRL distribution points, and OCSP servers.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl_distribution_points` | array | no | Comma-separated list of URLs to be used for the CRL distribution points attribute. See also RFC 5280 Section 4.2.1.13. |
| `enable_templating` | boolean | no | Whether or not to enable templating of the above AIA fields. When templating is enabled the special values '{{issuer_id}}' and '{{cluster_path}}' are available, but the addresses are not checked for URI validity until issuance time. This requires /config/cluster's path to be set on all PR Secondary clusters. |
| `issuing_certificates` | array | no | Comma-separated list of URLs to be used for the issuing certificate attribute. See also RFC 5280 Section 4.2.2.1. |
| `ocsp_servers` | array | no | Comma-separated list of URLs to be used for the OCSP servers attribute. See also RFC 5280 Section 4.2.2.1. |





### POST /{pki_mount_path}/config/urls

**Operation ID:** `pki-configure-urls`


Set the URLs for the issuing CA, CRL distribution points, and OCSP servers.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl_distribution_points` | array | no | Comma-separated list of URLs to be used for the CRL distribution points attribute. See also RFC 5280 Section 4.2.1.13. |
| `enable_templating` | boolean (default: False) | no | Whether or not to enabling templating of the above AIA fields. When templating is enabled the special values '{{issuer_id}}', '{{cluster_path}}', and '{{cluster_aia_path}}' are available, but the addresses are not checked for URI validity until issuance time. Using '{{cluster_path}}' requires /config/cluster's 'path' member to be set on all PR Secondary clusters and using '{{cluster_aia_path}}' requires /config/cluster's 'aia_path' member to be set on all PR secondary clusters. |
| `issuing_certificates` | array | no | Comma-separated list of URLs to be used for the issuing certificate attribute. See also RFC 5280 Section 4.2.2.1. |
| `ocsp_servers` | array | no | Comma-separated list of URLs to be used for the OCSP servers attribute. See also RFC 5280 Section 4.2.2.1. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl_distribution_points` | array | no | Comma-separated list of URLs to be used for the CRL distribution points attribute. See also RFC 5280 Section 4.2.1.13. |
| `enable_templating` | boolean (default: False) | no | Whether or not to enabling templating of the above AIA fields. When templating is enabled the special values '{{issuer_id}}' and '{{cluster_path}}' are available, but the addresses are not checked for URI validity until issuance time. This requires /config/cluster's path to be set on all PR Secondary clusters. |
| `issuing_certificates` | array | no | Comma-separated list of URLs to be used for the issuing certificate attribute. See also RFC 5280 Section 4.2.2.1. |
| `ocsp_servers` | array | no | Comma-separated list of URLs to be used for the OCSP servers attribute. See also RFC 5280 Section 4.2.2.1. |





### GET /{pki_mount_path}/crl

**Operation ID:** `pki-read-crl-der`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/crl/delta

**Operation ID:** `pki-read-crl-delta`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/crl/delta/pem

**Operation ID:** `pki-read-crl-delta-pem`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/crl/pem

**Operation ID:** `pki-read-crl-pem`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | string | no | Issuing CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | ID of the issuer |
| `revocation_time` | integer | no | Revocation time |
| `revocation_time_rfc3339` | string | no | Revocation time RFC 3339 formatted |





### GET /{pki_mount_path}/crl/rotate

**Operation ID:** `pki-rotate-crl`


Force a rebuild of the CRL.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `success` | boolean | no | Whether rotation was successful |





### GET /{pki_mount_path}/crl/rotate-delta

**Operation ID:** `pki-rotate-delta-crl`


Force a rebuild of the delta CRL.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `success` | boolean | no | Whether rotation was successful |





### GET /{pki_mount_path}/eab

**Operation ID:** `pki-list-eab-keys`


list external account bindings to be used for ACME


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_info` | object | no | EAB details keyed by the eab key id |
| `keys` | array | no | A list of unused eab keys |





### DELETE /{pki_mount_path}/eab/{key_id}

**Operation ID:** `pki-delete-eab-key`


Delete an external account binding id prior to its use within an ACME account


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `key_id` | string | path | yes | EAB key identifier |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /{pki_mount_path}/intermediate/cross-sign

**Operation ID:** `pki-cross-sign-intermediate`


Generate a new CSR and private key used for signing.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `add_basic_constraints` | boolean | no | Whether to add a Basic Constraints extension with CA: true. Only needed as a workaround in some compatibility scenarios with Active Directory Certificate Services. |
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. May contain both DNS names and email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If not specified when signing, the common name will be taken from the CSR; other names must still be specified in alt_names or ip_sans. |
| `country` | array | no | If set, Country will be set to this value. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `exported` | string (internal, external, kms) | no | Must be "internal", "exported" or "kms". If set to "exported", the generated private key will be returned. This is your *only* chance to retrieve the private key! |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `key_bits` | integer (default: 0) | no | The number of bits to use. Allowed values are 0 (universal default); with rsa key_type: 2048 (default), 3072, or 4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with ed25519. |
| `key_name` | string | no | Provide a name to the generated or existing key, the name must be unique across all keys and not be the reserved value 'default' |
| `key_ref` | string (default: default) | no | Reference to a existing key; either "default" for the configured default key, an identifier or the name assigned to the key. |
| `key_type` | string (rsa, ec, ed25519) (default: rsa) | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |
| `locality` | array | no | If set, Locality will be set to this value. |
| `managed_key_id` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_name is required. Ignored for other types. |
| `managed_key_name` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_id is required. Ignored for other types. |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `not_before_duration` | integer (default: 30) | no | The duration before now which the certificate needs to be backdated by. |
| `organization` | array | no | If set, O (Organization) will be set to this value. |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `ou` | array | no | If set, OU (OrganizationalUnit) will be set to this value. |
| `postal_code` | array | no | If set, Postal Code will be set to this value. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `province` | array | no | If set, Province will be set to this value. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `street_address` | array | no | If set, Street Address will be set to this value. |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the mount max TTL. Note: this only has an effect when generating a CA cert or signing a CA cert, not when generating a CSR for an intermediate CA. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `csr` | string | no | Certificate signing request. |
| `key_id` | string | no | Id of the key. |
| `private_key` | string | no | Generated private key. |
| `private_key_type` | string | no | Specifies the format used for marshaling the private key. |





### POST /{pki_mount_path}/intermediate/generate/{exported}

**Operation ID:** `pki-generate-intermediate`


Generate a new CSR and private key used for signing.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `exported` | string (internal, external, kms) | path | yes | Must be "internal", "exported" or "kms". If set to "exported", the generated private key will be returned. This is your *only* chance to retrieve the private key! |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `add_basic_constraints` | boolean | no | Whether to add a Basic Constraints extension with CA: true. Only needed as a workaround in some compatibility scenarios with Active Directory Certificate Services. |
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. May contain both DNS names and email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If not specified when signing, the common name will be taken from the CSR; other names must still be specified in alt_names or ip_sans. |
| `country` | array | no | If set, Country will be set to this value. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `key_bits` | integer (default: 0) | no | The number of bits to use. Allowed values are 0 (universal default); with rsa key_type: 2048 (default), 3072, or 4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with ed25519. |
| `key_name` | string | no | Provide a name to the generated or existing key, the name must be unique across all keys and not be the reserved value 'default' |
| `key_ref` | string (default: default) | no | Reference to a existing key; either "default" for the configured default key, an identifier or the name assigned to the key. |
| `key_type` | string (rsa, ec, ed25519) (default: rsa) | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |
| `locality` | array | no | If set, Locality will be set to this value. |
| `managed_key_id` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_name is required. Ignored for other types. |
| `managed_key_name` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_id is required. Ignored for other types. |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `not_before_duration` | integer (default: 30) | no | The duration before now which the certificate needs to be backdated by. |
| `organization` | array | no | If set, O (Organization) will be set to this value. |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `ou` | array | no | If set, OU (OrganizationalUnit) will be set to this value. |
| `postal_code` | array | no | If set, Postal Code will be set to this value. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `province` | array | no | If set, Province will be set to this value. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `street_address` | array | no | If set, Street Address will be set to this value. |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the mount max TTL. Note: this only has an effect when generating a CA cert or signing a CA cert, not when generating a CSR for an intermediate CA. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `csr` | string | no | Certificate signing request. |
| `key_id` | string | no | Id of the key. |
| `private_key` | string | no | Generated private key. |
| `private_key_type` | string | no | Specifies the format used for marshaling the private key. |





### POST /{pki_mount_path}/intermediate/set-signed

**Operation ID:** `pki-set-signed-intermediate`


Provide the signed intermediate CA cert.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `certificate` | string | no | PEM-format certificate. This must be a CA certificate with a public key matching the previously-generated key from the generation endpoint. Additional parent CAs may be optionally appended to the bundle. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `existing_issuers` | array | no | Existing issuers specified as part of the import bundle of this request |
| `existing_keys` | array | no | Existing keys specified as part of the import bundle of this request |
| `imported_issuers` | array | no | Net-new issuers imported as a part of this request |
| `imported_keys` | array | no | Net-new keys imported as a part of this request |
| `mapping` | object | no | A mapping of issuer_id to key_id for all issuers included in this request |





### POST /{pki_mount_path}/issue/{role}

**Operation ID:** `pki-issue-with-role`


Request a certificate using a certain role with the provided details.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | The desired role with configuration for this request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. If email protection is enabled for the role, this may contain email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If email protection is enabled in the role, this may be an email address. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `issuer_ref` | string (default: default) | no | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `remove_roots_from_chain` | boolean (default: False) | no | Whether or not to remove self-signed CA certificates in the output of the ca_chain field. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the role max TTL. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `user_ids` | array | no | The requested user_ids value to place in the subject, if any, in a comma-delimited list. Restricted by allowed_user_ids. Any values are added with OID 0.9.2342.19200300.100.1.1. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | Certificate Chain |
| `certificate` | string | no | Certificate |
| `expiration` | integer | no | Time of expiration |
| `issuing_ca` | string | no | Issuing Certificate Authority |
| `private_key` | string | no | Private key |
| `private_key_type` | string | no | Private key type |
| `serial_number` | string | no | Serial Number |





### GET /{pki_mount_path}/issuer/{issuer_ref}

**Operation ID:** `pki-read-issuer`


Fetch a single issuer certificate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | CA Chain |
| `certificate` | string | no | Certificate |
| `crl_distribution_points` | array | no | CRL Distribution Points |
| `enable_aia_url_templating` | boolean | no | Whether or not templating is enabled for AIA fields |
| `issuer_id` | string | no | Issuer Id |
| `issuer_name` | string | no | Issuer Name |
| `issuing_certificates` | array | no | Issuing Certificates |
| `key_id` | string | no | Key Id |
| `leaf_not_after_behavior` | string | no | Leaf Not After Behavior |
| `manual_chain` | array | no | Manual Chain |
| `ocsp_servers` | array | no | OCSP Servers |
| `revocation_signature_algorithm` | string | no | Revocation Signature Alogrithm |
| `revocation_time` | integer | no |  |
| `revocation_time_rfc3339` | string | no |  |
| `revoked` | boolean | no | Revoked |
| `usage` | string | no | Usage |





### POST /{pki_mount_path}/issuer/{issuer_ref}

**Operation ID:** `pki-write-issuer`


Fetch a single issuer certificate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl_distribution_points` | array | no | Comma-separated list of URLs to be used for the CRL distribution points attribute. See also RFC 5280 Section 4.2.1.13. |
| `enable_aia_url_templating` | boolean (default: False) | no | Whether or not to enabling templating of the above AIA fields. When templating is enabled the special values '{{issuer_id}}', '{{cluster_path}}', '{{cluster_aia_path}}' are available, but the addresses are not checked for URL validity until issuance time. Using '{{cluster_path}}' requires /config/cluster's 'path' member to be set on all PR Secondary clusters and using '{{cluster_aia_path}}' requires /config/cluster's 'aia_path' member to be set on all PR secondary clusters. |
| `issuer_name` | string | no | Provide a name to the generated or existing issuer, the name must be unique across all issuers and not be the reserved value 'default' |
| `issuing_certificates` | array | no | Comma-separated list of URLs to be used for the issuing certificate attribute. See also RFC 5280 Section 4.2.2.1. |
| `leaf_not_after_behavior` | string (default: err) | no | Behavior of leaf's NotAfter fields: "err" to error if the computed NotAfter date exceeds that of this issuer; "truncate" to silently truncate to that of this issuer; or "permit" to allow this issuance to succeed (with NotAfter exceeding that of an issuer). Note that not all values will results in certificates that can be validated through the entire validity period. It is suggested to use "truncate" for intermediate CAs and "permit" only for root CAs. |
| `manual_chain` | array | no | Chain of issuer references to use to build this issuer's computed CAChain field, when non-empty. |
| `ocsp_servers` | array | no | Comma-separated list of URLs to be used for the OCSP servers attribute. See also RFC 5280 Section 4.2.2.1. |
| `revocation_signature_algorithm` | string (default: ) | no | Which x509.SignatureAlgorithm name to use for signing CRLs. This parameter allows differentiation between PKCS#1v1.5 and PSS keys and choice of signature hash algorithm. The default (empty string) value is for Go to select the signature algorithm. This can fail if the underlying key does not support the requested signature algorithm, which may not be known at modification time (such as with PKCS#11 managed RSA keys). |
| `usage` | array (default: ['read-only', 'issuing-certificates', 'crl-signing', 'ocsp-signing']) | no | Comma-separated list (or string slice) of usages for this issuer; valid values are "read-only", "issuing-certificates", "crl-signing", and "ocsp-signing". Multiple values may be specified. Read-only is implicit and always set. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | CA Chain |
| `certificate` | string | no | Certificate |
| `crl_distribution_points` | array | no | CRL Distribution Points |
| `enable_aia_url_templating` | boolean | no | Whether or not templating is enabled for AIA fields |
| `issuer_id` | string | no | Issuer Id |
| `issuer_name` | string | no | Issuer Name |
| `issuing_certificates` | array | no | Issuing Certificates |
| `key_id` | string | no | Key Id |
| `leaf_not_after_behavior` | string | no | Leaf Not After Behavior |
| `manual_chain` | array | no | Manual Chain |
| `ocsp_servers` | array | no | OCSP Servers |
| `revocation_signature_algorithm` | string | no | Revocation Signature Alogrithm |
| `revocation_time` | integer | no |  |
| `revocation_time_rfc3339` | string | no |  |
| `revoked` | boolean | no | Revoked |
| `usage` | string | no | Usage |





### DELETE /{pki_mount_path}/issuer/{issuer_ref}

**Operation ID:** `pki-delete-issuer`


Fetch a single issuer certificate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### POST /{pki_mount_path}/issuer/{issuer_ref}/acme/account/{kid}

**Operation ID:** `pki-write-issuer-issuer_ref-acme-account-kid`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `kid` | string | path | yes | The key identifier provided by the CA |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/acme/authorization/{auth_id}

**Operation ID:** `pki-write-issuer-issuer_ref-acme-authorization-auth_id`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `auth_id` | string | path | yes | ACME authorization identifier value |
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/acme/challenge/{auth_id}/{challenge_type}

**Operation ID:** `pki-write-issuer-issuer_ref-acme-challenge-auth_id-challenge_type`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `auth_id` | string | path | yes | ACME authorization identifier value |
| `challenge_type` | string | path | yes | ACME challenge type |
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### GET /{pki_mount_path}/issuer/{issuer_ref}/acme/directory

**Operation ID:** `pki-read-issuer-issuer_ref-acme-directory`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/acme/new-account

**Operation ID:** `pki-write-issuer-issuer_ref-acme-new-account`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/acme/new-eab

**Operation ID:** `pki-generate-eab-key-for-issuer`


Generate external account bindings to be used for ACME


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `acme_directory` | string | no | The ACME directory to which the key belongs |
| `created_on` | string | no | An RFC3339 formatted date time when the EAB token was created |
| `id` | string | no | The EAB key identifier |
| `key` | string | no | The EAB hmac key |
| `key_type` | string | no | The EAB key type |





### GET /{pki_mount_path}/issuer/{issuer_ref}/acme/new-nonce

**Operation ID:** `pki-read-issuer-issuer_ref-acme-new-nonce`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/acme/new-order

**Operation ID:** `pki-write-issuer-issuer_ref-acme-new-order`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/acme/order/{order_id}

**Operation ID:** `pki-write-issuer-issuer_ref-acme-order-order_id`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `order_id` | string | path | yes | The ACME order identifier to fetch |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/acme/order/{order_id}/cert

**Operation ID:** `pki-write-issuer-issuer_ref-acme-order-order_id-cert`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `order_id` | string | path | yes | The ACME order identifier to fetch |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/acme/order/{order_id}/finalize

**Operation ID:** `pki-write-issuer-issuer_ref-acme-order-order_id-finalize`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `order_id` | string | path | yes | The ACME order identifier to fetch |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/acme/orders

**Operation ID:** `pki-write-issuer-issuer_ref-acme-orders`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/acme/revoke-cert

**Operation ID:** `pki-write-issuer-issuer_ref-acme-revoke-cert`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### GET /{pki_mount_path}/issuer/{issuer_ref}/crl

**Operation ID:** `pki-issuer-read-crl`


Fetch an issuer's Certificate Revocation Log (CRL).


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no |  |





### GET /{pki_mount_path}/issuer/{issuer_ref}/crl/delta

**Operation ID:** `pki-issuer-read-crl-delta`


Fetch an issuer's Certificate Revocation Log (CRL).


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no |  |





### GET /{pki_mount_path}/issuer/{issuer_ref}/crl/delta/der

**Operation ID:** `pki-issuer-read-crl-delta-der`


Fetch an issuer's Certificate Revocation Log (CRL).


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no |  |





### GET /{pki_mount_path}/issuer/{issuer_ref}/crl/delta/pem

**Operation ID:** `pki-issuer-read-crl-delta-pem`


Fetch an issuer's Certificate Revocation Log (CRL).


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no |  |





### GET /{pki_mount_path}/issuer/{issuer_ref}/crl/der

**Operation ID:** `pki-issuer-read-crl-der`


Fetch an issuer's Certificate Revocation Log (CRL).


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no |  |





### GET /{pki_mount_path}/issuer/{issuer_ref}/crl/pem

**Operation ID:** `pki-issuer-read-crl-pem`


Fetch an issuer's Certificate Revocation Log (CRL).


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no |  |





### GET /{pki_mount_path}/issuer/{issuer_ref}/der

**Operation ID:** `pki-read-issuer-der`


Fetch a single issuer certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | Issuer Id |
| `issuer_name` | string | no | Issuer Name |





**304**: Not Modified



### POST /{pki_mount_path}/issuer/{issuer_ref}/issue/{role}

**Operation ID:** `pki-issuer-issue-with-role`


Request a certificate using a certain role with the provided details.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `role` | string | path | yes | The desired role with configuration for this request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. If email protection is enabled for the role, this may contain email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If email protection is enabled in the role, this may be an email address. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `remove_roots_from_chain` | boolean (default: False) | no | Whether or not to remove self-signed CA certificates in the output of the ca_chain field. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the role max TTL. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `user_ids` | array | no | The requested user_ids value to place in the subject, if any, in a comma-delimited list. Restricted by allowed_user_ids. Any values are added with OID 0.9.2342.19200300.100.1.1. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | Certificate Chain |
| `certificate` | string | no | Certificate |
| `expiration` | integer | no | Time of expiration |
| `issuing_ca` | string | no | Issuing Certificate Authority |
| `private_key` | string | no | Private key |
| `private_key_type` | string | no | Private key type |
| `serial_number` | string | no | Serial Number |





### GET /{pki_mount_path}/issuer/{issuer_ref}/json

**Operation ID:** `pki-read-issuer-json`


Fetch a single issuer certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | Issuer Id |
| `issuer_name` | string | no | Issuer Name |





**304**: Not Modified



### GET /{pki_mount_path}/issuer/{issuer_ref}/pem

**Operation ID:** `pki-read-issuer-pem`


Fetch a single issuer certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | CA Chain |
| `certificate` | string | no | Certificate |
| `issuer_id` | string | no | Issuer Id |
| `issuer_name` | string | no | Issuer Name |





**304**: Not Modified



### POST /{pki_mount_path}/issuer/{issuer_ref}/resign-crls

**Operation ID:** `pki-issuer-resign-crls`


Combine and sign with the provided issuer different CRLs


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl_number` | integer | no | The sequence number to be written within the CRL Number extension. |
| `crls` | array | no | A list of PEM encoded CRLs to combine, originally signed by the requested issuer. |
| `delta_crl_base_number` | integer (default: -1) | no | Using a zero or greater value specifies the base CRL revision number to encode within a Delta CRL indicator extension, otherwise the extension will not be added. |
| `format` | string (default: pem) | no | The format of the combined CRL, can be "pem" or "der". If "der", the value will be base64 encoded. Defaults to "pem". |
| `next_update` | string (default: 72h) | no | The amount of time the generated CRL should be valid; defaults to 72 hours. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no | CRL |





### POST /{pki_mount_path}/issuer/{issuer_ref}/revoke

**Operation ID:** `pki-revoke-issuer`


Revoke the specified issuer certificate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | Certificate Authority Chain |
| `certificate` | string | no | Certificate |
| `crl_distribution_points` | array | no | Specifies the URL values for the CRL Distribution Points field |
| `issuer_id` | string | no | ID of the issuer |
| `issuer_name` | string | no | Name of the issuer |
| `issuing_certificates` | array | no | Specifies the URL values for the Issuing Certificate field |
| `key_id` | string | no | ID of the Key |
| `leaf_not_after_behavior` | string | no |  |
| `manual_chain` | array | no | Manual Chain |
| `ocsp_servers` | array | no | Specifies the URL values for the OCSP Servers field |
| `revocation_signature_algorithm` | string | no | Which signature algorithm to use when building CRLs |
| `revocation_time` | integer | no | Time of revocation |
| `revocation_time_rfc3339` | string | no | RFC formatted time of revocation |
| `revoked` | boolean | no | Whether the issuer was revoked |
| `usage` | string | no | Allowed usage |





### POST /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/account/{kid}

**Operation ID:** `pki-write-issuer-issuer_ref-roles-role-acme-account-kid`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `kid` | string | path | yes | The key identifier provided by the CA |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/authorization/{auth_id}

**Operation ID:** `pki-write-issuer-issuer_ref-roles-role-acme-authorization-auth_id`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `auth_id` | string | path | yes | ACME authorization identifier value |
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/challenge/{auth_id}/{challenge_type}

**Operation ID:** `pki-write-issuer-issuer_ref-roles-role-acme-challenge-auth_id-challenge_type`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `auth_id` | string | path | yes | ACME authorization identifier value |
| `challenge_type` | string | path | yes | ACME challenge type |
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### GET /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/directory

**Operation ID:** `pki-read-issuer-issuer_ref-roles-role-acme-directory`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/new-account

**Operation ID:** `pki-write-issuer-issuer_ref-roles-role-acme-new-account`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/new-eab

**Operation ID:** `pki-generate-eab-key-for-issuer-and-role`


Generate external account bindings to be used for ACME


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `acme_directory` | string | no | The ACME directory to which the key belongs |
| `created_on` | string | no | An RFC3339 formatted date time when the EAB token was created |
| `id` | string | no | The EAB key identifier |
| `key` | string | no | The EAB hmac key |
| `key_type` | string | no | The EAB key type |





### GET /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/new-nonce

**Operation ID:** `pki-read-issuer-issuer_ref-roles-role-acme-new-nonce`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/new-order

**Operation ID:** `pki-write-issuer-issuer_ref-roles-role-acme-new-order`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/order/{order_id}

**Operation ID:** `pki-write-issuer-issuer_ref-roles-role-acme-order-order_id`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `order_id` | string | path | yes | The ACME order identifier to fetch |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/order/{order_id}/cert

**Operation ID:** `pki-write-issuer-issuer_ref-roles-role-acme-order-order_id-cert`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `order_id` | string | path | yes | The ACME order identifier to fetch |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/order/{order_id}/finalize

**Operation ID:** `pki-write-issuer-issuer_ref-roles-role-acme-order-order_id-finalize`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `order_id` | string | path | yes | The ACME order identifier to fetch |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/orders

**Operation ID:** `pki-write-issuer-issuer_ref-roles-role-acme-orders`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/roles/{role}/acme/revoke-cert

**Operation ID:** `pki-write-issuer-issuer_ref-roles-role-acme-revoke-cert`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to an existing issuer name or issuer id |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/issuer/{issuer_ref}/sign-intermediate

**Operation ID:** `pki-issuer-sign-intermediate`


Issue an intermediate CA certificate based on the provided CSR.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. May contain both DNS names and email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If not specified when signing, the common name will be taken from the CSR; other names must still be specified in alt_names or ip_sans. |
| `country` | array | no | If set, Country will be set to this value. |
| `csr` | string (default: ) | no | PEM-format CSR to be signed. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `issuer_name` | string | no | Provide a name to the generated or existing issuer, the name must be unique across all issuers and not be the reserved value 'default' |
| `locality` | array | no | If set, Locality will be set to this value. |
| `max_path_length` | integer (default: -1) | no | The maximum allowable path length |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `not_before_duration` | integer (default: 30) | no | The duration before now which the certificate needs to be backdated by. |
| `organization` | array | no | If set, O (Organization) will be set to this value. |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `ou` | array | no | If set, OU (OrganizationalUnit) will be set to this value. |
| `permitted_dns_domains` | array | no | Domains for which this certificate is allowed to sign or issue child certificates. If set, all DNS names (subject and alt) on child certs must be exact matches or subsets of the given domains (see https://tools.ietf.org/html/rfc5280#section-4.2.1.10). |
| `postal_code` | array | no | If set, Postal Code will be set to this value. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `province` | array | no | If set, Province will be set to this value. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `skid` | string (default: ) | no | Value for the Subject Key Identifier field (RFC 5280 Section 4.2.1.2). This value should ONLY be used when cross-signing to mimic the existing certificate's SKID value; this is necessary to allow certain TLS implementations (such as OpenSSL) which use SKID/AKID matches in chain building to restrict possible valid chains. Specified as a string in hex format. Default is empty, allowing Vault to automatically calculate the SKID according to method one in the above RFC section. |
| `street_address` | array | no | If set, Street Address will be set to this value. |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the mount max TTL. Note: this only has an effect when generating a CA cert or signing a CA cert, not when generating a CSR for an intermediate CA. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `use_csr_values` | boolean (default: False) | no | If true, then: 1) Subject information, including names and alternate names, will be preserved from the CSR rather than using values provided in the other parameters to this path; 2) Any key usages requested in the CSR will be added to the basic set of key usages used for CA certs signed by this path; for instance, the non-repudiation flag; 3) Extensions requested in the CSR will be copied into the issued certificate. |
| `use_pss` | boolean (default: False) | no | Whether or not to use PSS signatures when using a RSA key-type issuer. Defaults to false. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | CA Chain |
| `certificate` | string | no | Certificate |
| `expiration` | integer | no | Expiration Time |
| `issuing_ca` | string | no | Issuing CA |
| `serial_number` | string | no | Serial Number |





### POST /{pki_mount_path}/issuer/{issuer_ref}/sign-revocation-list

**Operation ID:** `pki-issuer-sign-revocation-list`


Generate and sign a CRL based on the provided parameters.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl_number` | integer | no | The sequence number to be written within the CRL Number extension. |
| `delta_crl_base_number` | integer (default: -1) | no | Using a zero or greater value specifies the base CRL revision number to encode within a Delta CRL indicator extension, otherwise the extension will not be added. |
| `extensions` | array | no | A list of maps containing extensions with keys id (string), critical (bool), value (string) |
| `format` | string (default: pem) | no | The format of the combined CRL, can be "pem" or "der". If "der", the value will be base64 encoded. Defaults to "pem". |
| `next_update` | string (default: 72h) | no | The amount of time the generated CRL should be valid; defaults to 72 hours. |
| `revoked_certs` | array | no | A list of maps containing the keys serial_number (string), revocation_time (string), and extensions (map with keys id (string), critical (bool), value (string)) |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no | CRL |





### POST /{pki_mount_path}/issuer/{issuer_ref}/sign-self-issued

**Operation ID:** `pki-issuer-sign-self-issued`


Re-issue a self-signed certificate based on the provided certificate.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `certificate` | string | no | PEM-format self-issued certificate to be signed. |
| `require_matching_certificate_algorithms` | boolean (default: False) | no | If true, require the public key algorithm of the signer to match that of the self issued certificate. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `certificate` | string | no | Certificate |
| `issuing_ca` | string | no | Issuing CA |





### POST /{pki_mount_path}/issuer/{issuer_ref}/sign-verbatim

**Operation ID:** `pki-issuer-sign-verbatim`


Issue a certificate directly based on the provided CSR.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. If email protection is enabled for the role, this may contain email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If email protection is enabled in the role, this may be an email address. |
| `csr` | string (default: ) | no | PEM-format CSR to be signed. Values will be taken verbatim from the CSR, except for basic constraints. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `ext_key_usage` | array (default: []) | no | A comma-separated string or list of extended key usages. Valid values can be found at https://golang.org/pkg/crypto/x509/#ExtKeyUsage -- simply drop the "ExtKeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. |
| `ext_key_usage_oids` | array | no | A comma-separated string or list of extended key usage oids. |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `key_usage` | array (default: ['DigitalSignature', 'KeyAgreement', 'KeyEncipherment']) | no | A comma-separated string or list of key usages (not extended key usages). Valid values can be found at https://golang.org/pkg/crypto/x509/#KeyUsage -- simply drop the "KeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `remove_roots_from_chain` | boolean (default: False) | no | Whether or not to remove self-signed CA certificates in the output of the ca_chain field. |
| `role` | string | no | The desired role with configuration for this request |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the role max TTL. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `use_pss` | boolean (default: False) | no | Whether or not to use PSS signatures when using a RSA key-type issuer. Defaults to false. |
| `user_ids` | array | no | The requested user_ids value to place in the subject, if any, in a comma-delimited list. Restricted by allowed_user_ids. Any values are added with OID 0.9.2342.19200300.100.1.1. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | Certificate Chain |
| `certificate` | string | no | Certificate |
| `expiration` | integer | no | Time of expiration |
| `issuing_ca` | string | no | Issuing Certificate Authority |
| `private_key` | string | no | Private key |
| `private_key_type` | string | no | Private key type |
| `serial_number` | string | no | Serial Number |





### POST /{pki_mount_path}/issuer/{issuer_ref}/sign-verbatim/{role}

**Operation ID:** `pki-issuer-sign-verbatim-with-role`


Issue a certificate directly based on the provided CSR.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `role` | string | path | yes | The desired role with configuration for this request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. If email protection is enabled for the role, this may contain email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If email protection is enabled in the role, this may be an email address. |
| `csr` | string (default: ) | no | PEM-format CSR to be signed. Values will be taken verbatim from the CSR, except for basic constraints. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `ext_key_usage` | array (default: []) | no | A comma-separated string or list of extended key usages. Valid values can be found at https://golang.org/pkg/crypto/x509/#ExtKeyUsage -- simply drop the "ExtKeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. |
| `ext_key_usage_oids` | array | no | A comma-separated string or list of extended key usage oids. |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `key_usage` | array (default: ['DigitalSignature', 'KeyAgreement', 'KeyEncipherment']) | no | A comma-separated string or list of key usages (not extended key usages). Valid values can be found at https://golang.org/pkg/crypto/x509/#KeyUsage -- simply drop the "KeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `remove_roots_from_chain` | boolean (default: False) | no | Whether or not to remove self-signed CA certificates in the output of the ca_chain field. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the role max TTL. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `use_pss` | boolean (default: False) | no | Whether or not to use PSS signatures when using a RSA key-type issuer. Defaults to false. |
| `user_ids` | array | no | The requested user_ids value to place in the subject, if any, in a comma-delimited list. Restricted by allowed_user_ids. Any values are added with OID 0.9.2342.19200300.100.1.1. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | Certificate Chain |
| `certificate` | string | no | Certificate |
| `expiration` | integer | no | Time of expiration |
| `issuing_ca` | string | no | Issuing Certificate Authority |
| `private_key` | string | no | Private key |
| `private_key_type` | string | no | Private key type |
| `serial_number` | string | no | Serial Number |





### POST /{pki_mount_path}/issuer/{issuer_ref}/sign/{role}

**Operation ID:** `pki-issuer-sign-with-role`


Request certificates using a certain role with the provided details.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `role` | string | path | yes | The desired role with configuration for this request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. If email protection is enabled for the role, this may contain email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If email protection is enabled in the role, this may be an email address. |
| `csr` | string (default: ) | no | PEM-format CSR to be signed. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `remove_roots_from_chain` | boolean (default: False) | no | Whether or not to remove self-signed CA certificates in the output of the ca_chain field. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the role max TTL. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `user_ids` | array | no | The requested user_ids value to place in the subject, if any, in a comma-delimited list. Restricted by allowed_user_ids. Any values are added with OID 0.9.2342.19200300.100.1.1. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | Certificate Chain |
| `certificate` | string | no | Certificate |
| `expiration` | integer | no | Time of expiration |
| `issuing_ca` | string | no | Issuing Certificate Authority |
| `private_key` | string | no | Private key |
| `private_key_type` | string | no | Private key type |
| `serial_number` | string | no | Serial Number |





### GET /{pki_mount_path}/issuer/{issuer_ref}/unified-crl

**Operation ID:** `pki-issuer-read-unified-crl`


Fetch an issuer's Certificate Revocation Log (CRL).


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no |  |





### GET /{pki_mount_path}/issuer/{issuer_ref}/unified-crl/delta

**Operation ID:** `pki-issuer-read-unified-crl-delta`


Fetch an issuer's Certificate Revocation Log (CRL).


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no |  |





### GET /{pki_mount_path}/issuer/{issuer_ref}/unified-crl/delta/der

**Operation ID:** `pki-issuer-read-unified-crl-delta-der`


Fetch an issuer's Certificate Revocation Log (CRL).


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no |  |





### GET /{pki_mount_path}/issuer/{issuer_ref}/unified-crl/delta/pem

**Operation ID:** `pki-issuer-read-unified-crl-delta-pem`


Fetch an issuer's Certificate Revocation Log (CRL).


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no |  |





### GET /{pki_mount_path}/issuer/{issuer_ref}/unified-crl/der

**Operation ID:** `pki-issuer-read-unified-crl-der`


Fetch an issuer's Certificate Revocation Log (CRL).


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no |  |





### GET /{pki_mount_path}/issuer/{issuer_ref}/unified-crl/pem

**Operation ID:** `pki-issuer-read-unified-crl-pem`


Fetch an issuer's Certificate Revocation Log (CRL).


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `issuer_ref` | string | path | yes | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `crl` | string | no |  |





### GET /{pki_mount_path}/issuers

**Operation ID:** `pki-list-issuers`


Fetch a list of CA certificates.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_info` | object | no | Key info with issuer name |
| `keys` | array | no | A list of keys |





### POST /{pki_mount_path}/issuers/generate/intermediate/{exported}

**Operation ID:** `pki-issuers-generate-intermediate`


Generate a new CSR and private key used for signing.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `exported` | string (internal, external, kms) | path | yes | Must be "internal", "exported" or "kms". If set to "exported", the generated private key will be returned. This is your *only* chance to retrieve the private key! |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `add_basic_constraints` | boolean | no | Whether to add a Basic Constraints extension with CA: true. Only needed as a workaround in some compatibility scenarios with Active Directory Certificate Services. |
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. May contain both DNS names and email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If not specified when signing, the common name will be taken from the CSR; other names must still be specified in alt_names or ip_sans. |
| `country` | array | no | If set, Country will be set to this value. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `key_bits` | integer (default: 0) | no | The number of bits to use. Allowed values are 0 (universal default); with rsa key_type: 2048 (default), 3072, or 4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with ed25519. |
| `key_name` | string | no | Provide a name to the generated or existing key, the name must be unique across all keys and not be the reserved value 'default' |
| `key_ref` | string (default: default) | no | Reference to a existing key; either "default" for the configured default key, an identifier or the name assigned to the key. |
| `key_type` | string (rsa, ec, ed25519) (default: rsa) | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |
| `locality` | array | no | If set, Locality will be set to this value. |
| `managed_key_id` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_name is required. Ignored for other types. |
| `managed_key_name` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_id is required. Ignored for other types. |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `not_before_duration` | integer (default: 30) | no | The duration before now which the certificate needs to be backdated by. |
| `organization` | array | no | If set, O (Organization) will be set to this value. |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `ou` | array | no | If set, OU (OrganizationalUnit) will be set to this value. |
| `postal_code` | array | no | If set, Postal Code will be set to this value. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `province` | array | no | If set, Province will be set to this value. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `street_address` | array | no | If set, Street Address will be set to this value. |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the mount max TTL. Note: this only has an effect when generating a CA cert or signing a CA cert, not when generating a CSR for an intermediate CA. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `csr` | string | no | Certificate signing request. |
| `key_id` | string | no | Id of the key. |
| `private_key` | string | no | Generated private key. |
| `private_key_type` | string | no | Specifies the format used for marshaling the private key. |





### POST /{pki_mount_path}/issuers/generate/root/{exported}

**Operation ID:** `pki-issuers-generate-root`


Generate a new CA certificate and private key used for signing.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `exported` | string (internal, external, kms) | path | yes | Must be "internal", "exported" or "kms". If set to "exported", the generated private key will be returned. This is your *only* chance to retrieve the private key! |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. May contain both DNS names and email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If not specified when signing, the common name will be taken from the CSR; other names must still be specified in alt_names or ip_sans. |
| `country` | array | no | If set, Country will be set to this value. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `issuer_name` | string | no | Provide a name to the generated or existing issuer, the name must be unique across all issuers and not be the reserved value 'default' |
| `key_bits` | integer (default: 0) | no | The number of bits to use. Allowed values are 0 (universal default); with rsa key_type: 2048 (default), 3072, or 4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with ed25519. |
| `key_name` | string | no | Provide a name to the generated or existing key, the name must be unique across all keys and not be the reserved value 'default' |
| `key_ref` | string (default: default) | no | Reference to a existing key; either "default" for the configured default key, an identifier or the name assigned to the key. |
| `key_type` | string (rsa, ec, ed25519) (default: rsa) | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |
| `locality` | array | no | If set, Locality will be set to this value. |
| `managed_key_id` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_name is required. Ignored for other types. |
| `managed_key_name` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_id is required. Ignored for other types. |
| `max_path_length` | integer (default: -1) | no | The maximum allowable path length |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `not_before_duration` | integer (default: 30) | no | The duration before now which the certificate needs to be backdated by. |
| `organization` | array | no | If set, O (Organization) will be set to this value. |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `ou` | array | no | If set, OU (OrganizationalUnit) will be set to this value. |
| `permitted_dns_domains` | array | no | Domains for which this certificate is allowed to sign or issue child certificates. If set, all DNS names (subject and alt) on child certs must be exact matches or subsets of the given domains (see https://tools.ietf.org/html/rfc5280#section-4.2.1.10). |
| `postal_code` | array | no | If set, Postal Code will be set to this value. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `province` | array | no | If set, Province will be set to this value. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `street_address` | array | no | If set, Street Address will be set to this value. |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the mount max TTL. Note: this only has an effect when generating a CA cert or signing a CA cert, not when generating a CSR for an intermediate CA. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `use_pss` | boolean (default: False) | no | Whether or not to use PSS signatures when using a RSA key-type issuer. Defaults to false. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `certificate` | string | no | The generated self-signed CA certificate. |
| `expiration` | integer | no | The expiration of the given issuer. |
| `issuer_id` | string | no | The ID of the issuer |
| `issuer_name` | string | no | The name of the issuer. |
| `issuing_ca` | string | no | The issuing certificate authority. |
| `key_id` | string | no | The ID of the key. |
| `key_name` | string | no | The key name if given. |
| `private_key` | string | no | The private key if exported was specified. |
| `serial_number` | string | no | The requested Subject's named serial number. |





### POST /{pki_mount_path}/issuers/import/bundle

**Operation ID:** `pki-issuers-import-bundle`


Import the specified issuing certificates.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `pem_bundle` | string | no | PEM-format, concatenated unencrypted secret-key (optional) and certificates. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `existing_issuers` | array | no | Existing issuers specified as part of the import bundle of this request |
| `existing_keys` | array | no | Existing keys specified as part of the import bundle of this request |
| `imported_issuers` | array | no | Net-new issuers imported as a part of this request |
| `imported_keys` | array | no | Net-new keys imported as a part of this request |
| `mapping` | object | no | A mapping of issuer_id to key_id for all issuers included in this request |





### POST /{pki_mount_path}/issuers/import/cert

**Operation ID:** `pki-issuers-import-cert`


Import the specified issuing certificates.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `pem_bundle` | string | no | PEM-format, concatenated unencrypted secret-key (optional) and certificates. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `existing_issuers` | array | no | Existing issuers specified as part of the import bundle of this request |
| `existing_keys` | array | no | Existing keys specified as part of the import bundle of this request |
| `imported_issuers` | array | no | Net-new issuers imported as a part of this request |
| `imported_keys` | array | no | Net-new keys imported as a part of this request |
| `mapping` | object | no | A mapping of issuer_id to key_id for all issuers included in this request |





### GET /{pki_mount_path}/key/{key_ref}

**Operation ID:** `pki-read-key`


Fetch a single issuer key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `key_ref` | string | path | yes | Reference to key; either "default" for the configured default key, an identifier of a key, or the name assigned to the key. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_id` | string | no | Key Id |
| `key_name` | string | no | Key Name |
| `key_type` | string | no | Key Type |
| `managed_key_id` | string | no | Managed Key Id |
| `managed_key_name` | string | no | Managed Key Name |
| `subject_key_id` | string | no | RFC 5280 Subject Key Identifier of the public counterpart |





### POST /{pki_mount_path}/key/{key_ref}

**Operation ID:** `pki-write-key`


Fetch a single issuer key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `key_ref` | string | path | yes | Reference to key; either "default" for the configured default key, an identifier of a key, or the name assigned to the key. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_name` | string | no | Human-readable name for this key. |




#### Responses


**204**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_id` | string | no | Key Id |
| `key_name` | string | no | Key Name |
| `key_type` | string | no | Key Type |





### DELETE /{pki_mount_path}/key/{key_ref}

**Operation ID:** `pki-delete-key`


Fetch a single issuer key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `key_ref` | string | path | yes | Reference to key; either "default" for the configured default key, an identifier of a key, or the name assigned to the key. |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### GET /{pki_mount_path}/keys

**Operation ID:** `pki-list-keys`


Fetch a list of all issuer keys


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_info` | object | no | Key info with issuer name |
| `keys` | array | no | A list of keys |





### POST /{pki_mount_path}/keys/generate/exported

**Operation ID:** `pki-generate-exported-key`


Generate a new private key used for signing.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_bits` | integer (default: 0) | no | The number of bits to use. Allowed values are 0 (universal default); with rsa key_type: 2048 (default), 3072, or 4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with ed25519. |
| `key_name` | string | no | Optional name to be used for this key |
| `key_type` | string (rsa, ec, ed25519) (default: rsa) | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |
| `managed_key_id` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_name is required. Ignored for other types. |
| `managed_key_name` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_id is required. Ignored for other types. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_id` | string | no | ID assigned to this key. |
| `key_name` | string | no | Name assigned to this key. |
| `key_type` | string | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |
| `private_key` | string | no | The private key string |





### POST /{pki_mount_path}/keys/generate/internal

**Operation ID:** `pki-generate-internal-key`


Generate a new private key used for signing.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_bits` | integer (default: 0) | no | The number of bits to use. Allowed values are 0 (universal default); with rsa key_type: 2048 (default), 3072, or 4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with ed25519. |
| `key_name` | string | no | Optional name to be used for this key |
| `key_type` | string (rsa, ec, ed25519) (default: rsa) | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |
| `managed_key_id` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_name is required. Ignored for other types. |
| `managed_key_name` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_id is required. Ignored for other types. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_id` | string | no | ID assigned to this key. |
| `key_name` | string | no | Name assigned to this key. |
| `key_type` | string | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |
| `private_key` | string | no | The private key string |





### POST /{pki_mount_path}/keys/generate/kms

**Operation ID:** `pki-generate-kms-key`


Generate a new private key used for signing.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_bits` | integer (default: 0) | no | The number of bits to use. Allowed values are 0 (universal default); with rsa key_type: 2048 (default), 3072, or 4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with ed25519. |
| `key_name` | string | no | Optional name to be used for this key |
| `key_type` | string (rsa, ec, ed25519) (default: rsa) | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |
| `managed_key_id` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_name is required. Ignored for other types. |
| `managed_key_name` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_id is required. Ignored for other types. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_id` | string | no | ID assigned to this key. |
| `key_name` | string | no | Name assigned to this key. |
| `key_type` | string | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |
| `private_key` | string | no | The private key string |





### POST /{pki_mount_path}/keys/import

**Operation ID:** `pki-import-key`


Import the specified key.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_name` | string | no | Optional name to be used for this key |
| `pem_bundle` | string | no | PEM-format, unencrypted secret key |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_id` | string | no | ID assigned to this key. |
| `key_name` | string | no | Name assigned to this key. |
| `key_type` | string | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |





### POST /{pki_mount_path}/ocsp

**Operation ID:** `pki-query-ocsp`


Query a certificate's revocation status through OCSP'


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{pki_mount_path}/ocsp/{req}

**Operation ID:** `pki-query-ocsp-with-get-req`


Query a certificate's revocation status through OCSP'


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `req` | string | path | yes | base-64 encoded ocsp request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{pki_mount_path}/revoke

**Operation ID:** `pki-revoke`


Revoke a certificate by serial number or with explicit certificate. When calling /revoke-with-key, the private key corresponding to the certificate must be provided to authenticate the request.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `certificate` | string | no | Certificate to revoke in PEM format; must be signed by an issuer in this mount. |
| `serial_number` | string | no | Certificate serial number, in colon- or hyphen-separated octal |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `revocation_time` | integer | no | Revocation Time |
| `revocation_time_rfc3339` | string | no | Revocation Time |
| `state` | string | no | Revocation State |





### POST /{pki_mount_path}/revoke-with-key

**Operation ID:** `pki-revoke-with-key`


Revoke a certificate by serial number or with explicit certificate. When calling /revoke-with-key, the private key corresponding to the certificate must be provided to authenticate the request.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `certificate` | string | no | Certificate to revoke in PEM format; must be signed by an issuer in this mount. |
| `private_key` | string | no | Key to use to verify revocation permission; must be in PEM format. |
| `serial_number` | string | no | Certificate serial number, in colon- or hyphen-separated octal |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `revocation_time` | integer | no | Revocation Time |
| `revocation_time_rfc3339` | string | no | Revocation Time |
| `state` | string | no | Revocation State |





### GET /{pki_mount_path}/roles

**Operation ID:** `pki-list-roles`


List the existing roles in this backend


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no | List of roles |





### GET /{pki_mount_path}/roles/{name}

**Operation ID:** `pki-read-role`


Manage the roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allow_any_name` | boolean | no | If set, clients can request certificates for any domain, regardless of allowed_domains restrictions. See the documentation for more information. |
| `allow_bare_domains` | boolean | no | If set, clients can request certificates for the base domains themselves, e.g. "example.com" of domains listed in allowed_domains. This is a separate option as in some cases this can be considered a security threat. See the documentation for more information. |
| `allow_glob_domains` | boolean | no | If set, domains specified in allowed_domains can include shell-style glob patterns, e.g. "ftp*.example.com". See the documentation for more information. |
| `allow_ip_sans` | boolean | no | If set, IP Subject Alternative Names are allowed. Any valid IP is accepted and No authorization checking is performed. |
| `allow_localhost` | boolean | no | Whether to allow "localhost" and "localdomain" as a valid common name in a request, independent of allowed_domains value. |
| `allow_subdomains` | boolean | no | If set, clients can request certificates for subdomains of domains listed in allowed_domains, including wildcard subdomains. See the documentation for more information. |
| `allow_token_displayname` | boolean | no | Whether to allow "localhost" and "localdomain" as a valid common name in a request, independent of allowed_domains value. |
| `allow_wildcard_certificates` | boolean | no | If set, allows certificates with wildcards in the common name to be issued, conforming to RFC 6125's Section 6.4.3; e.g., "*.example.net" or "b*z.example.net". See the documentation for more information. |
| `allowed_domains` | array | no | Specifies the domains this role is allowed to issue certificates for. This is used with the allow_bare_domains, allow_subdomains, and allow_glob_domains to determine matches for the common name, DNS-typed SAN entries, and Email-typed SAN entries of certificates. See the documentation for more information. This parameter accepts a comma-separated string or list of domains. |
| `allowed_domains_template` | boolean | no | If set, Allowed domains can be specified using identity template policies. Non-templated domains are also permitted. |
| `allowed_other_sans` | array | no | If set, an array of allowed other names to put in SANs. These values support globbing and must be in the format <oid>;<type>:<value>. Currently only "utf8" is a valid type. All values, including globbing values, must use this syntax, with the exception being a single "*" which allows any OID and any value (but type must still be utf8). |
| `allowed_serial_numbers` | array | no | If set, an array of allowed serial numbers to put in Subject. These values support globbing. |
| `allowed_uri_sans` | array | no | If set, an array of allowed URIs for URI Subject Alternative Names. Any valid URI is accepted, these values support globbing. |
| `allowed_uri_sans_template` | boolean | no | If set, Allowed URI SANs can be specified using identity template policies. Non-templated URI SANs are also permitted. |
| `allowed_user_ids` | array | no | If set, an array of allowed user-ids to put in user system login name specified here: https://www.rfc-editor.org/rfc/rfc1274#section-9.3.1 |
| `basic_constraints_valid_for_non_ca` | boolean | no | Mark Basic Constraints valid when issuing non-CA certificates. |
| `client_flag` | boolean | no | If set, certificates are flagged for client auth use. Defaults to true. See also RFC 5280 Section 4.2.1.12. |
| `cn_validations` | array | no | List of allowed validations to run against the Common Name field. Values can include 'email' to validate the CN is a email address, 'hostname' to validate the CN is a valid hostname (potentially including wildcards). When multiple validations are specified, these take OR semantics (either email OR hostname are allowed). The special value 'disabled' allows disabling all CN name validations, allowing for arbitrary non-Hostname, non-Email address CNs. |
| `code_signing_flag` | boolean | no | If set, certificates are flagged for code signing use. Defaults to false. See also RFC 5280 Section 4.2.1.12. |
| `country` | array | no | If set, Country will be set to this value in certificates issued by this role. |
| `email_protection_flag` | boolean | no | If set, certificates are flagged for email protection use. Defaults to false. See also RFC 5280 Section 4.2.1.12. |
| `enforce_hostnames` | boolean | no | If set, only valid host names are allowed for CN and DNS SANs, and the host part of email addresses. Defaults to true. |
| `ext_key_usage` | array | no | A comma-separated string or list of extended key usages. Valid values can be found at https://golang.org/pkg/crypto/x509/#ExtKeyUsage -- simply drop the "ExtKeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. See also RFC 5280 Section 4.2.1.12. |
| `ext_key_usage_oids` | array | no | A comma-separated string or list of extended key usage oids. |
| `generate_lease` | boolean | no | If set, certificates issued/signed against this role will have Vault leases attached to them. Defaults to "false". Certificates can be added to the CRL by "vault revoke <lease_id>" when certificates are associated with leases. It can also be done using the "pki/revoke" endpoint. However, when lease generation is disabled, invoking "pki/revoke" would be the only way to add the certificates to the CRL. When large number of certificates are generated with long lifetimes, it is recommended that lease generation be disabled, as large amount of leases adversely affect the startup time of Vault. |
| `issuer_ref` | string | no | Reference to the issuer used to sign requests serviced by this role. |
| `key_bits` | integer | no | The number of bits to use. Allowed values are 0 (universal default); with rsa key_type: 2048 (default), 3072, or 4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with ed25519. |
| `key_type` | string | no | The type of key to use; defaults to RSA. "rsa" "ec", "ed25519" and "any" are the only valid values. |
| `key_usage` | array | no | A comma-separated string or list of key usages (not extended key usages). Valid values can be found at https://golang.org/pkg/crypto/x509/#KeyUsage -- simply drop the "KeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. See also RFC 5280 Section 4.2.1.3. |
| `locality` | array | no | If set, Locality will be set to this value in certificates issued by this role. |
| `max_ttl` | integer | no | The maximum allowed lease duration. If not set, defaults to the system maximum lease TTL. |
| `no_store` | boolean | no | If set, certificates issued/signed against this role will not be stored in the storage backend. This can improve performance when issuing large numbers of certificates. However, certificates issued in this way cannot be enumerated or revoked, so this option is recommended only for certificates that are non-sensitive, or extremely short-lived. This option implies a value of "false" for "generate_lease". |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ. |
| `not_before_duration` | integer | no | The duration in seconds before now which the certificate needs to be backdated by. |
| `organization` | array | no | If set, O (Organization) will be set to this value in certificates issued by this role. |
| `ou` | array | no | If set, OU (OrganizationalUnit) will be set to this value in certificates issued by this role. |
| `policy_identifiers` | array | no | A comma-separated string or list of policy OIDs, or a JSON list of qualified policy information, which must include an oid, and may include a notice and/or cps url, using the form [{"oid"="1.3.6.1.4.1.7.8","notice"="I am a user Notice"}, {"oid"="1.3.6.1.4.1.44947.1.2.4 ","cps"="https://example.com"}]. |
| `postal_code` | array | no | If set, Postal Code will be set to this value in certificates issued by this role. |
| `province` | array | no | If set, Province will be set to this value in certificates issued by this role. |
| `require_cn` | boolean | no | If set to false, makes the 'common_name' field optional while generating a certificate. |
| `server_flag` | boolean (default: True) | no | If set, certificates are flagged for server auth use. Defaults to true. See also RFC 5280 Section 4.2.1.12. |
| `signature_bits` | integer | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `street_address` | array | no | If set, Street Address will be set to this value in certificates issued by this role. |
| `ttl` | integer | no | The lease duration (validity period of the certificate) if no specific lease duration is requested. The lease duration controls the expiration of certificates issued by this backend. Defaults to the system default value or the value of max_ttl, whichever is shorter. |
| `use_csr_common_name` | boolean | no | If set, when used with a signing profile, the common name in the CSR will be used. This does *not* include any requested Subject Alternative Names; use use_csr_sans for that. Defaults to true. |
| `use_csr_sans` | boolean | no | If set, when used with a signing profile, the SANs in the CSR will be used. This does *not* include the Common Name (cn); use use_csr_common_name for that. Defaults to true. |
| `use_pss` | boolean | no | Whether or not to use PSS signatures when using a RSA key-type issuer. Defaults to false. |





### POST /{pki_mount_path}/roles/{name}

**Operation ID:** `pki-write-role`


Manage the roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allow_any_name` | boolean | no | If set, clients can request certificates for any domain, regardless of allowed_domains restrictions. See the documentation for more information. |
| `allow_bare_domains` | boolean | no | If set, clients can request certificates for the base domains themselves, e.g. "example.com" of domains listed in allowed_domains. This is a separate option as in some cases this can be considered a security threat. See the documentation for more information. |
| `allow_glob_domains` | boolean | no | If set, domains specified in allowed_domains can include shell-style glob patterns, e.g. "ftp*.example.com". See the documentation for more information. |
| `allow_ip_sans` | boolean (default: True) | no | If set, IP Subject Alternative Names are allowed. Any valid IP is accepted and No authorization checking is performed. |
| `allow_localhost` | boolean (default: True) | no | Whether to allow "localhost" and "localdomain" as a valid common name in a request, independent of allowed_domains value. |
| `allow_subdomains` | boolean | no | If set, clients can request certificates for subdomains of domains listed in allowed_domains, including wildcard subdomains. See the documentation for more information. |
| `allow_wildcard_certificates` | boolean (default: True) | no | If set, allows certificates with wildcards in the common name to be issued, conforming to RFC 6125's Section 6.4.3; e.g., "*.example.net" or "b*z.example.net". See the documentation for more information. |
| `allowed_domains` | array | no | Specifies the domains this role is allowed to issue certificates for. This is used with the allow_bare_domains, allow_subdomains, and allow_glob_domains to determine matches for the common name, DNS-typed SAN entries, and Email-typed SAN entries of certificates. See the documentation for more information. This parameter accepts a comma-separated string or list of domains. |
| `allowed_domains_template` | boolean (default: False) | no | If set, Allowed domains can be specified using identity template policies. Non-templated domains are also permitted. |
| `allowed_other_sans` | array | no | If set, an array of allowed other names to put in SANs. These values support globbing and must be in the format <oid>;<type>:<value>. Currently only "utf8" is a valid type. All values, including globbing values, must use this syntax, with the exception being a single "*" which allows any OID and any value (but type must still be utf8). |
| `allowed_serial_numbers` | array | no | If set, an array of allowed serial numbers to put in Subject. These values support globbing. |
| `allowed_uri_sans` | array | no | If set, an array of allowed URIs for URI Subject Alternative Names. Any valid URI is accepted, these values support globbing. |
| `allowed_uri_sans_template` | boolean (default: False) | no | If set, Allowed URI SANs can be specified using identity template policies. Non-templated URI SANs are also permitted. |
| `allowed_user_ids` | array | no | If set, an array of allowed user-ids to put in user system login name specified here: https://www.rfc-editor.org/rfc/rfc1274#section-9.3.1 |
| `backend` | string | no | Backend Type |
| `basic_constraints_valid_for_non_ca` | boolean | no | Mark Basic Constraints valid when issuing non-CA certificates. |
| `client_flag` | boolean (default: True) | no | If set, certificates are flagged for client auth use. Defaults to true. See also RFC 5280 Section 4.2.1.12. |
| `cn_validations` | array (default: ['email', 'hostname']) | no | List of allowed validations to run against the Common Name field. Values can include 'email' to validate the CN is a email address, 'hostname' to validate the CN is a valid hostname (potentially including wildcards). When multiple validations are specified, these take OR semantics (either email OR hostname are allowed). The special value 'disabled' allows disabling all CN name validations, allowing for arbitrary non-Hostname, non-Email address CNs. |
| `code_signing_flag` | boolean | no | If set, certificates are flagged for code signing use. Defaults to false. See also RFC 5280 Section 4.2.1.12. |
| `country` | array | no | If set, Country will be set to this value in certificates issued by this role. |
| `email_protection_flag` | boolean | no | If set, certificates are flagged for email protection use. Defaults to false. See also RFC 5280 Section 4.2.1.12. |
| `enforce_hostnames` | boolean (default: True) | no | If set, only valid host names are allowed for CN and DNS SANs, and the host part of email addresses. Defaults to true. |
| `ext_key_usage` | array (default: []) | no | A comma-separated string or list of extended key usages. Valid values can be found at https://golang.org/pkg/crypto/x509/#ExtKeyUsage -- simply drop the "ExtKeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. See also RFC 5280 Section 4.2.1.12. |
| `ext_key_usage_oids` | array | no | A comma-separated string or list of extended key usage oids. |
| `generate_lease` | boolean | no | If set, certificates issued/signed against this role will have Vault leases attached to them. Defaults to "false". Certificates can be added to the CRL by "vault revoke <lease_id>" when certificates are associated with leases. It can also be done using the "pki/revoke" endpoint. However, when lease generation is disabled, invoking "pki/revoke" would be the only way to add the certificates to the CRL. When large number of certificates are generated with long lifetimes, it is recommended that lease generation be disabled, as large amount of leases adversely affect the startup time of Vault. |
| `issuer_ref` | string (default: default) | no | Reference to the issuer used to sign requests serviced by this role. |
| `key_bits` | integer (default: 0) | no | The number of bits to use. Allowed values are 0 (universal default); with rsa key_type: 2048 (default), 3072, or 4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with ed25519. |
| `key_type` | string (rsa, ec, ed25519, any) (default: rsa) | no | The type of key to use; defaults to RSA. "rsa" "ec", "ed25519" and "any" are the only valid values. |
| `key_usage` | array (default: ['DigitalSignature', 'KeyAgreement', 'KeyEncipherment']) | no | A comma-separated string or list of key usages (not extended key usages). Valid values can be found at https://golang.org/pkg/crypto/x509/#KeyUsage -- simply drop the "KeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. See also RFC 5280 Section 4.2.1.3. |
| `locality` | array | no | If set, Locality will be set to this value in certificates issued by this role. |
| `max_ttl` | integer | no | The maximum allowed lease duration. If not set, defaults to the system maximum lease TTL. |
| `no_store` | boolean | no | If set, certificates issued/signed against this role will not be stored in the storage backend. This can improve performance when issuing large numbers of certificates. However, certificates issued in this way cannot be enumerated or revoked, so this option is recommended only for certificates that are non-sensitive, or extremely short-lived. This option implies a value of "false" for "generate_lease". |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ. |
| `not_before_duration` | integer (default: 30) | no | The duration before now which the certificate needs to be backdated by. |
| `organization` | array | no | If set, O (Organization) will be set to this value in certificates issued by this role. |
| `ou` | array | no | If set, OU (OrganizationalUnit) will be set to this value in certificates issued by this role. |
| `policy_identifiers` | array | no | A comma-separated string or list of policy OIDs, or a JSON list of qualified policy information, which must include an oid, and may include a notice and/or cps url, using the form [{"oid"="1.3.6.1.4.1.7.8","notice"="I am a user Notice"}, {"oid"="1.3.6.1.4.1.44947.1.2.4 ","cps"="https://example.com"}]. |
| `postal_code` | array | no | If set, Postal Code will be set to this value in certificates issued by this role. |
| `province` | array | no | If set, Province will be set to this value in certificates issued by this role. |
| `require_cn` | boolean (default: True) | no | If set to false, makes the 'common_name' field optional while generating a certificate. |
| `server_flag` | boolean (default: True) | no | If set, certificates are flagged for server auth use. Defaults to true. See also RFC 5280 Section 4.2.1.12. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `street_address` | array | no | If set, Street Address will be set to this value in certificates issued by this role. |
| `ttl` | integer | no | The lease duration (validity period of the certificate) if no specific lease duration is requested. The lease duration controls the expiration of certificates issued by this backend. Defaults to the system default value or the value of max_ttl, whichever is shorter. |
| `use_csr_common_name` | boolean (default: True) | no | If set, when used with a signing profile, the common name in the CSR will be used. This does *not* include any requested Subject Alternative Names; use use_csr_sans for that. Defaults to true. |
| `use_csr_sans` | boolean (default: True) | no | If set, when used with a signing profile, the SANs in the CSR will be used. This does *not* include the Common Name (cn); use use_csr_common_name for that. Defaults to true. |
| `use_pss` | boolean (default: False) | no | Whether or not to use PSS signatures when using a RSA key-type issuer. Defaults to false. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allow_any_name` | boolean | no | If set, clients can request certificates for any domain, regardless of allowed_domains restrictions. See the documentation for more information. |
| `allow_bare_domains` | boolean | no | If set, clients can request certificates for the base domains themselves, e.g. "example.com" of domains listed in allowed_domains. This is a separate option as in some cases this can be considered a security threat. See the documentation for more information. |
| `allow_glob_domains` | boolean | no | If set, domains specified in allowed_domains can include shell-style glob patterns, e.g. "ftp*.example.com". See the documentation for more information. |
| `allow_ip_sans` | boolean | no | If set, IP Subject Alternative Names are allowed. Any valid IP is accepted and No authorization checking is performed. |
| `allow_localhost` | boolean | no | Whether to allow "localhost" and "localdomain" as a valid common name in a request, independent of allowed_domains value. |
| `allow_subdomains` | boolean | no | If set, clients can request certificates for subdomains of domains listed in allowed_domains, including wildcard subdomains. See the documentation for more information. |
| `allow_token_displayname` | boolean | no | Whether to allow "localhost" and "localdomain" as a valid common name in a request, independent of allowed_domains value. |
| `allow_wildcard_certificates` | boolean | no | If set, allows certificates with wildcards in the common name to be issued, conforming to RFC 6125's Section 6.4.3; e.g., "*.example.net" or "b*z.example.net". See the documentation for more information. |
| `allowed_domains` | array | no | Specifies the domains this role is allowed to issue certificates for. This is used with the allow_bare_domains, allow_subdomains, and allow_glob_domains to determine matches for the common name, DNS-typed SAN entries, and Email-typed SAN entries of certificates. See the documentation for more information. This parameter accepts a comma-separated string or list of domains. |
| `allowed_domains_template` | boolean | no | If set, Allowed domains can be specified using identity template policies. Non-templated domains are also permitted. |
| `allowed_other_sans` | array | no | If set, an array of allowed other names to put in SANs. These values support globbing and must be in the format <oid>;<type>:<value>. Currently only "utf8" is a valid type. All values, including globbing values, must use this syntax, with the exception being a single "*" which allows any OID and any value (but type must still be utf8). |
| `allowed_serial_numbers` | array | no | If set, an array of allowed serial numbers to put in Subject. These values support globbing. |
| `allowed_uri_sans` | array | no | If set, an array of allowed URIs for URI Subject Alternative Names. Any valid URI is accepted, these values support globbing. |
| `allowed_uri_sans_template` | boolean | no | If set, Allowed URI SANs can be specified using identity template policies. Non-templated URI SANs are also permitted. |
| `allowed_user_ids` | array | no | If set, an array of allowed user-ids to put in user system login name specified here: https://www.rfc-editor.org/rfc/rfc1274#section-9.3.1 |
| `basic_constraints_valid_for_non_ca` | boolean | no | Mark Basic Constraints valid when issuing non-CA certificates. |
| `client_flag` | boolean | no | If set, certificates are flagged for client auth use. Defaults to true. See also RFC 5280 Section 4.2.1.12. |
| `cn_validations` | array | no | List of allowed validations to run against the Common Name field. Values can include 'email' to validate the CN is a email address, 'hostname' to validate the CN is a valid hostname (potentially including wildcards). When multiple validations are specified, these take OR semantics (either email OR hostname are allowed). The special value 'disabled' allows disabling all CN name validations, allowing for arbitrary non-Hostname, non-Email address CNs. |
| `code_signing_flag` | boolean | no | If set, certificates are flagged for code signing use. Defaults to false. See also RFC 5280 Section 4.2.1.12. |
| `country` | array | no | If set, Country will be set to this value in certificates issued by this role. |
| `email_protection_flag` | boolean | no | If set, certificates are flagged for email protection use. Defaults to false. See also RFC 5280 Section 4.2.1.12. |
| `enforce_hostnames` | boolean | no | If set, only valid host names are allowed for CN and DNS SANs, and the host part of email addresses. Defaults to true. |
| `ext_key_usage` | array | no | A comma-separated string or list of extended key usages. Valid values can be found at https://golang.org/pkg/crypto/x509/#ExtKeyUsage -- simply drop the "ExtKeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. See also RFC 5280 Section 4.2.1.12. |
| `ext_key_usage_oids` | array | no | A comma-separated string or list of extended key usage oids. |
| `generate_lease` | boolean | no | If set, certificates issued/signed against this role will have Vault leases attached to them. Defaults to "false". Certificates can be added to the CRL by "vault revoke <lease_id>" when certificates are associated with leases. It can also be done using the "pki/revoke" endpoint. However, when lease generation is disabled, invoking "pki/revoke" would be the only way to add the certificates to the CRL. When large number of certificates are generated with long lifetimes, it is recommended that lease generation be disabled, as large amount of leases adversely affect the startup time of Vault. |
| `issuer_ref` | string | no | Reference to the issuer used to sign requests serviced by this role. |
| `key_bits` | integer | no | The number of bits to use. Allowed values are 0 (universal default); with rsa key_type: 2048 (default), 3072, or 4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with ed25519. |
| `key_type` | string | no | The type of key to use; defaults to RSA. "rsa" "ec", "ed25519" and "any" are the only valid values. |
| `key_usage` | array | no | A comma-separated string or list of key usages (not extended key usages). Valid values can be found at https://golang.org/pkg/crypto/x509/#KeyUsage -- simply drop the "KeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. See also RFC 5280 Section 4.2.1.3. |
| `locality` | array | no | If set, Locality will be set to this value in certificates issued by this role. |
| `max_ttl` | integer | no | The maximum allowed lease duration. If not set, defaults to the system maximum lease TTL. |
| `no_store` | boolean | no | If set, certificates issued/signed against this role will not be stored in the storage backend. This can improve performance when issuing large numbers of certificates. However, certificates issued in this way cannot be enumerated or revoked, so this option is recommended only for certificates that are non-sensitive, or extremely short-lived. This option implies a value of "false" for "generate_lease". |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ. |
| `not_before_duration` | integer | no | The duration in seconds before now which the certificate needs to be backdated by. |
| `organization` | array | no | If set, O (Organization) will be set to this value in certificates issued by this role. |
| `ou` | array | no | If set, OU (OrganizationalUnit) will be set to this value in certificates issued by this role. |
| `policy_identifiers` | array | no | A comma-separated string or list of policy OIDs, or a JSON list of qualified policy information, which must include an oid, and may include a notice and/or cps url, using the form [{"oid"="1.3.6.1.4.1.7.8","notice"="I am a user Notice"}, {"oid"="1.3.6.1.4.1.44947.1.2.4 ","cps"="https://example.com"}]. |
| `postal_code` | array | no | If set, Postal Code will be set to this value in certificates issued by this role. |
| `province` | array | no | If set, Province will be set to this value in certificates issued by this role. |
| `require_cn` | boolean | no | If set to false, makes the 'common_name' field optional while generating a certificate. |
| `server_flag` | boolean (default: True) | no | If set, certificates are flagged for server auth use. Defaults to true. See also RFC 5280 Section 4.2.1.12. |
| `signature_bits` | integer | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `street_address` | array | no | If set, Street Address will be set to this value in certificates issued by this role. |
| `ttl` | integer | no | The lease duration (validity period of the certificate) if no specific lease duration is requested. The lease duration controls the expiration of certificates issued by this backend. Defaults to the system default value or the value of max_ttl, whichever is shorter. |
| `use_csr_common_name` | boolean | no | If set, when used with a signing profile, the common name in the CSR will be used. This does *not* include any requested Subject Alternative Names; use use_csr_sans for that. Defaults to true. |
| `use_csr_sans` | boolean | no | If set, when used with a signing profile, the SANs in the CSR will be used. This does *not* include the Common Name (cn); use use_csr_common_name for that. Defaults to true. |
| `use_pss` | boolean | no | Whether or not to use PSS signatures when using a RSA key-type issuer. Defaults to false. |





### DELETE /{pki_mount_path}/roles/{name}

**Operation ID:** `pki-delete-role`


Manage the roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: No Content



### POST /{pki_mount_path}/roles/{role}/acme/account/{kid}

**Operation ID:** `pki-write-roles-role-acme-account-kid`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `kid` | string | path | yes | The key identifier provided by the CA |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/roles/{role}/acme/authorization/{auth_id}

**Operation ID:** `pki-write-roles-role-acme-authorization-auth_id`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `auth_id` | string | path | yes | ACME authorization identifier value |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/roles/{role}/acme/challenge/{auth_id}/{challenge_type}

**Operation ID:** `pki-write-roles-role-acme-challenge-auth_id-challenge_type`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `auth_id` | string | path | yes | ACME authorization identifier value |
| `challenge_type` | string | path | yes | ACME challenge type |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### GET /{pki_mount_path}/roles/{role}/acme/directory

**Operation ID:** `pki-read-roles-role-acme-directory`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{pki_mount_path}/roles/{role}/acme/new-account

**Operation ID:** `pki-write-roles-role-acme-new-account`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/roles/{role}/acme/new-eab

**Operation ID:** `pki-generate-eab-key-for-role`


Generate external account bindings to be used for ACME


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `acme_directory` | string | no | The ACME directory to which the key belongs |
| `created_on` | string | no | An RFC3339 formatted date time when the EAB token was created |
| `id` | string | no | The EAB key identifier |
| `key` | string | no | The EAB hmac key |
| `key_type` | string | no | The EAB key type |





### GET /{pki_mount_path}/roles/{role}/acme/new-nonce

**Operation ID:** `pki-read-roles-role-acme-new-nonce`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{pki_mount_path}/roles/{role}/acme/new-order

**Operation ID:** `pki-write-roles-role-acme-new-order`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/roles/{role}/acme/order/{order_id}

**Operation ID:** `pki-write-roles-role-acme-order-order_id`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `order_id` | string | path | yes | The ACME order identifier to fetch |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/roles/{role}/acme/order/{order_id}/cert

**Operation ID:** `pki-write-roles-role-acme-order-order_id-cert`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `order_id` | string | path | yes | The ACME order identifier to fetch |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/roles/{role}/acme/order/{order_id}/finalize

**Operation ID:** `pki-write-roles-role-acme-order-order_id-finalize`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `order_id` | string | path | yes | The ACME order identifier to fetch |
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/roles/{role}/acme/orders

**Operation ID:** `pki-write-roles-role-acme-orders`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### POST /{pki_mount_path}/roles/{role}/acme/revoke-cert

**Operation ID:** `pki-write-roles-role-acme-revoke-cert`


An endpoint implementing the standard ACME protocol


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | The desired role for the acme request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `payload` | string | no | ACME request 'payload' value |
| `protected` | string | no | ACME request 'protected' value |
| `signature` | string | no | ACME request 'signature' value |




#### Responses


**200**: OK



### DELETE /{pki_mount_path}/root

**Operation ID:** `pki-delete-root`


Deletes the root CA key to allow a new one to be generated.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{pki_mount_path}/root/generate/{exported}

**Operation ID:** `pki-generate-root`


Generate a new CA certificate and private key used for signing.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `exported` | string (internal, external, kms) | path | yes | Must be "internal", "exported" or "kms". If set to "exported", the generated private key will be returned. This is your *only* chance to retrieve the private key! |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. May contain both DNS names and email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If not specified when signing, the common name will be taken from the CSR; other names must still be specified in alt_names or ip_sans. |
| `country` | array | no | If set, Country will be set to this value. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `issuer_name` | string | no | Provide a name to the generated or existing issuer, the name must be unique across all issuers and not be the reserved value 'default' |
| `key_bits` | integer (default: 0) | no | The number of bits to use. Allowed values are 0 (universal default); with rsa key_type: 2048 (default), 3072, or 4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with ed25519. |
| `key_name` | string | no | Provide a name to the generated or existing key, the name must be unique across all keys and not be the reserved value 'default' |
| `key_ref` | string (default: default) | no | Reference to a existing key; either "default" for the configured default key, an identifier or the name assigned to the key. |
| `key_type` | string (rsa, ec, ed25519) (default: rsa) | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |
| `locality` | array | no | If set, Locality will be set to this value. |
| `managed_key_id` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_name is required. Ignored for other types. |
| `managed_key_name` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_id is required. Ignored for other types. |
| `max_path_length` | integer (default: -1) | no | The maximum allowable path length |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `not_before_duration` | integer (default: 30) | no | The duration before now which the certificate needs to be backdated by. |
| `organization` | array | no | If set, O (Organization) will be set to this value. |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `ou` | array | no | If set, OU (OrganizationalUnit) will be set to this value. |
| `permitted_dns_domains` | array | no | Domains for which this certificate is allowed to sign or issue child certificates. If set, all DNS names (subject and alt) on child certs must be exact matches or subsets of the given domains (see https://tools.ietf.org/html/rfc5280#section-4.2.1.10). |
| `postal_code` | array | no | If set, Postal Code will be set to this value. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `province` | array | no | If set, Province will be set to this value. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `street_address` | array | no | If set, Street Address will be set to this value. |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the mount max TTL. Note: this only has an effect when generating a CA cert or signing a CA cert, not when generating a CSR for an intermediate CA. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `use_pss` | boolean (default: False) | no | Whether or not to use PSS signatures when using a RSA key-type issuer. Defaults to false. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `certificate` | string | no | The generated self-signed CA certificate. |
| `expiration` | integer | no | The expiration of the given issuer. |
| `issuer_id` | string | no | The ID of the issuer |
| `issuer_name` | string | no | The name of the issuer. |
| `issuing_ca` | string | no | The issuing certificate authority. |
| `key_id` | string | no | The ID of the key. |
| `key_name` | string | no | The key name if given. |
| `private_key` | string | no | The private key if exported was specified. |
| `serial_number` | string | no | The requested Subject's named serial number. |





### POST /{pki_mount_path}/root/replace

**Operation ID:** `pki-replace-root`


Read and set the default issuer certificate for signing.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `default` | string (default: next) | no | Reference (name or identifier) to the default issuer. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `default` | string | no | Reference (name or identifier) to the default issuer. |
| `default_follows_latest_issuer` | boolean | no | Whether the default issuer should automatically follow the latest generated or imported issuer. Defaults to false. |





### POST /{pki_mount_path}/root/rotate/{exported}

**Operation ID:** `pki-rotate-root`


Generate a new CA certificate and private key used for signing.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `exported` | string (internal, external, kms) | path | yes | Must be "internal", "exported" or "kms". If set to "exported", the generated private key will be returned. This is your *only* chance to retrieve the private key! |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. May contain both DNS names and email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If not specified when signing, the common name will be taken from the CSR; other names must still be specified in alt_names or ip_sans. |
| `country` | array | no | If set, Country will be set to this value. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `issuer_name` | string | no | Provide a name to the generated or existing issuer, the name must be unique across all issuers and not be the reserved value 'default' |
| `key_bits` | integer (default: 0) | no | The number of bits to use. Allowed values are 0 (universal default); with rsa key_type: 2048 (default), 3072, or 4096; with ec key_type: 224, 256 (default), 384, or 521; ignored with ed25519. |
| `key_name` | string | no | Provide a name to the generated or existing key, the name must be unique across all keys and not be the reserved value 'default' |
| `key_ref` | string (default: default) | no | Reference to a existing key; either "default" for the configured default key, an identifier or the name assigned to the key. |
| `key_type` | string (rsa, ec, ed25519) (default: rsa) | no | The type of key to use; defaults to RSA. "rsa" "ec" and "ed25519" are the only valid values. |
| `locality` | array | no | If set, Locality will be set to this value. |
| `managed_key_id` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_name is required. Ignored for other types. |
| `managed_key_name` | string | no | The name of the managed key to use when the exported type is kms. When kms type is the key type, this field or managed_key_id is required. Ignored for other types. |
| `max_path_length` | integer (default: -1) | no | The maximum allowable path length |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `not_before_duration` | integer (default: 30) | no | The duration before now which the certificate needs to be backdated by. |
| `organization` | array | no | If set, O (Organization) will be set to this value. |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `ou` | array | no | If set, OU (OrganizationalUnit) will be set to this value. |
| `permitted_dns_domains` | array | no | Domains for which this certificate is allowed to sign or issue child certificates. If set, all DNS names (subject and alt) on child certs must be exact matches or subsets of the given domains (see https://tools.ietf.org/html/rfc5280#section-4.2.1.10). |
| `postal_code` | array | no | If set, Postal Code will be set to this value. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `province` | array | no | If set, Province will be set to this value. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `street_address` | array | no | If set, Street Address will be set to this value. |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the mount max TTL. Note: this only has an effect when generating a CA cert or signing a CA cert, not when generating a CSR for an intermediate CA. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `use_pss` | boolean (default: False) | no | Whether or not to use PSS signatures when using a RSA key-type issuer. Defaults to false. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `certificate` | string | no | The generated self-signed CA certificate. |
| `expiration` | integer | no | The expiration of the given issuer. |
| `issuer_id` | string | no | The ID of the issuer |
| `issuer_name` | string | no | The name of the issuer. |
| `issuing_ca` | string | no | The issuing certificate authority. |
| `key_id` | string | no | The ID of the key. |
| `key_name` | string | no | The key name if given. |
| `private_key` | string | no | The private key if exported was specified. |
| `serial_number` | string | no | The requested Subject's named serial number. |





### POST /{pki_mount_path}/root/sign-intermediate

**Operation ID:** `pki-root-sign-intermediate`


Issue an intermediate CA certificate based on the provided CSR.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. May contain both DNS names and email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If not specified when signing, the common name will be taken from the CSR; other names must still be specified in alt_names or ip_sans. |
| `country` | array | no | If set, Country will be set to this value. |
| `csr` | string (default: ) | no | PEM-format CSR to be signed. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `issuer_name` | string | no | Provide a name to the generated or existing issuer, the name must be unique across all issuers and not be the reserved value 'default' |
| `issuer_ref` | string (default: default) | no | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `locality` | array | no | If set, Locality will be set to this value. |
| `max_path_length` | integer (default: -1) | no | The maximum allowable path length |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `not_before_duration` | integer (default: 30) | no | The duration before now which the certificate needs to be backdated by. |
| `organization` | array | no | If set, O (Organization) will be set to this value. |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `ou` | array | no | If set, OU (OrganizationalUnit) will be set to this value. |
| `permitted_dns_domains` | array | no | Domains for which this certificate is allowed to sign or issue child certificates. If set, all DNS names (subject and alt) on child certs must be exact matches or subsets of the given domains (see https://tools.ietf.org/html/rfc5280#section-4.2.1.10). |
| `postal_code` | array | no | If set, Postal Code will be set to this value. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `province` | array | no | If set, Province will be set to this value. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `skid` | string (default: ) | no | Value for the Subject Key Identifier field (RFC 5280 Section 4.2.1.2). This value should ONLY be used when cross-signing to mimic the existing certificate's SKID value; this is necessary to allow certain TLS implementations (such as OpenSSL) which use SKID/AKID matches in chain building to restrict possible valid chains. Specified as a string in hex format. Default is empty, allowing Vault to automatically calculate the SKID according to method one in the above RFC section. |
| `street_address` | array | no | If set, Street Address will be set to this value. |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the mount max TTL. Note: this only has an effect when generating a CA cert or signing a CA cert, not when generating a CSR for an intermediate CA. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `use_csr_values` | boolean (default: False) | no | If true, then: 1) Subject information, including names and alternate names, will be preserved from the CSR rather than using values provided in the other parameters to this path; 2) Any key usages requested in the CSR will be added to the basic set of key usages used for CA certs signed by this path; for instance, the non-repudiation flag; 3) Extensions requested in the CSR will be copied into the issued certificate. |
| `use_pss` | boolean (default: False) | no | Whether or not to use PSS signatures when using a RSA key-type issuer. Defaults to false. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | CA Chain |
| `certificate` | string | no | Certificate |
| `expiration` | integer | no | Expiration Time |
| `issuing_ca` | string | no | Issuing CA |
| `serial_number` | string | no | Serial Number |





### POST /{pki_mount_path}/root/sign-self-issued

**Operation ID:** `pki-root-sign-self-issued`


Re-issue a self-signed certificate based on the provided certificate.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `certificate` | string | no | PEM-format self-issued certificate to be signed. |
| `issuer_ref` | string (default: default) | no | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `require_matching_certificate_algorithms` | boolean (default: False) | no | If true, require the public key algorithm of the signer to match that of the self issued certificate. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `certificate` | string | no | Certificate |
| `issuing_ca` | string | no | Issuing CA |





### POST /{pki_mount_path}/sign-verbatim

**Operation ID:** `pki-sign-verbatim`


Issue a certificate directly based on the provided CSR.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. If email protection is enabled for the role, this may contain email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If email protection is enabled in the role, this may be an email address. |
| `csr` | string (default: ) | no | PEM-format CSR to be signed. Values will be taken verbatim from the CSR, except for basic constraints. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `ext_key_usage` | array (default: []) | no | A comma-separated string or list of extended key usages. Valid values can be found at https://golang.org/pkg/crypto/x509/#ExtKeyUsage -- simply drop the "ExtKeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. |
| `ext_key_usage_oids` | array | no | A comma-separated string or list of extended key usage oids. |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `issuer_ref` | string (default: default) | no | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `key_usage` | array (default: ['DigitalSignature', 'KeyAgreement', 'KeyEncipherment']) | no | A comma-separated string or list of key usages (not extended key usages). Valid values can be found at https://golang.org/pkg/crypto/x509/#KeyUsage -- simply drop the "KeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `remove_roots_from_chain` | boolean (default: False) | no | Whether or not to remove self-signed CA certificates in the output of the ca_chain field. |
| `role` | string | no | The desired role with configuration for this request |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the role max TTL. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `use_pss` | boolean (default: False) | no | Whether or not to use PSS signatures when using a RSA key-type issuer. Defaults to false. |
| `user_ids` | array | no | The requested user_ids value to place in the subject, if any, in a comma-delimited list. Restricted by allowed_user_ids. Any values are added with OID 0.9.2342.19200300.100.1.1. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | Certificate Chain |
| `certificate` | string | no | Certificate |
| `expiration` | integer | no | Time of expiration |
| `issuing_ca` | string | no | Issuing Certificate Authority |
| `private_key` | string | no | Private key |
| `private_key_type` | string | no | Private key type |
| `serial_number` | string | no | Serial Number |





### POST /{pki_mount_path}/sign-verbatim/{role}

**Operation ID:** `pki-sign-verbatim-with-role`


Issue a certificate directly based on the provided CSR.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | The desired role with configuration for this request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. If email protection is enabled for the role, this may contain email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If email protection is enabled in the role, this may be an email address. |
| `csr` | string (default: ) | no | PEM-format CSR to be signed. Values will be taken verbatim from the CSR, except for basic constraints. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `ext_key_usage` | array (default: []) | no | A comma-separated string or list of extended key usages. Valid values can be found at https://golang.org/pkg/crypto/x509/#ExtKeyUsage -- simply drop the "ExtKeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. |
| `ext_key_usage_oids` | array | no | A comma-separated string or list of extended key usage oids. |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `issuer_ref` | string (default: default) | no | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `key_usage` | array (default: ['DigitalSignature', 'KeyAgreement', 'KeyEncipherment']) | no | A comma-separated string or list of key usages (not extended key usages). Valid values can be found at https://golang.org/pkg/crypto/x509/#KeyUsage -- simply drop the "KeyUsage" part of the name. To remove all key usages from being set, set this value to an empty list. |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `remove_roots_from_chain` | boolean (default: False) | no | Whether or not to remove self-signed CA certificates in the output of the ca_chain field. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `signature_bits` | integer (default: 0) | no | The number of bits to use in the signature algorithm; accepts 256 for SHA-2-256, 384 for SHA-2-384, and 512 for SHA-2-512. Defaults to 0 to automatically detect based on key length (SHA-2-256 for RSA keys, and matching the curve size for NIST P-Curves). |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the role max TTL. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `use_pss` | boolean (default: False) | no | Whether or not to use PSS signatures when using a RSA key-type issuer. Defaults to false. |
| `user_ids` | array | no | The requested user_ids value to place in the subject, if any, in a comma-delimited list. Restricted by allowed_user_ids. Any values are added with OID 0.9.2342.19200300.100.1.1. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | Certificate Chain |
| `certificate` | string | no | Certificate |
| `expiration` | integer | no | Time of expiration |
| `issuing_ca` | string | no | Issuing Certificate Authority |
| `private_key` | string | no | Private key |
| `private_key_type` | string | no | Private key type |
| `serial_number` | string | no | Serial Number |





### POST /{pki_mount_path}/sign/{role}

**Operation ID:** `pki-sign-with-role`


Request certificates using a certain role with the provided details.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | The desired role with configuration for this request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `alt_names` | string | no | The requested Subject Alternative Names, if any, in a comma-delimited list. If email protection is enabled for the role, this may contain email addresses. |
| `common_name` | string | no | The requested common name; if you want more than one, specify the alternative names in the alt_names map. If email protection is enabled in the role, this may be an email address. |
| `csr` | string (default: ) | no | PEM-format CSR to be signed. |
| `exclude_cn_from_sans` | boolean (default: False) | no | If true, the Common Name will not be included in DNS or Email Subject Alternate Names. Defaults to false (CN is included). |
| `format` | string (pem, der, pem_bundle) (default: pem) | no | Format for returned data. Can be "pem", "der", or "pem_bundle". If "pem_bundle", any private key and issuing cert will be appended to the certificate pem. If "der", the value will be base64 encoded. Defaults to "pem". |
| `ip_sans` | array | no | The requested IP SANs, if any, in a comma-delimited list |
| `issuer_ref` | string (default: default) | no | Reference to a existing issuer; either "default" for the configured default issuer, an identifier or the name assigned to the issuer. |
| `not_after` | string | no | Set the not after field of the certificate with specified date value. The value format should be given in UTC format YYYY-MM-ddTHH:MM:SSZ |
| `other_sans` | array | no | Requested other SANs, in an array with the format <oid>;UTF8:<utf8 string value> for each entry. |
| `private_key_format` | string (, der, pem, pkcs8) (default: der) | no | Format for the returned private key. Generally the default will be controlled by the "format" parameter as either base64-encoded DER or PEM-encoded DER. However, this can be set to "pkcs8" to have the returned private key contain base64-encoded pkcs8 or PEM-encoded pkcs8 instead. Defaults to "der". |
| `remove_roots_from_chain` | boolean (default: False) | no | Whether or not to remove self-signed CA certificates in the output of the ca_chain field. |
| `serial_number` | string | no | The Subject's requested serial number, if any. See RFC 4519 Section 2.31 'serialNumber' for a description of this field. If you want more than one, specify alternative names in the alt_names map using OID 2.5.4.5. This has no impact on the final certificate's Serial Number field. |
| `ttl` | integer | no | The requested Time To Live for the certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be larger than the role max TTL. |
| `uri_sans` | array | no | The requested URI SANs, if any, in a comma-delimited list. |
| `user_ids` | array | no | The requested user_ids value to place in the subject, if any, in a comma-delimited list. Restricted by allowed_user_ids. Any values are added with OID 0.9.2342.19200300.100.1.1. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ca_chain` | array | no | Certificate Chain |
| `certificate` | string | no | Certificate |
| `expiration` | integer | no | Time of expiration |
| `issuing_ca` | string | no | Issuing Certificate Authority |
| `private_key` | string | no | Private key |
| `private_key_type` | string | no | Private key type |
| `serial_number` | string | no | Serial Number |





### POST /{pki_mount_path}/tidy

**Operation ID:** `pki-tidy`


Tidy up the backend by removing expired certificates, revocation information, or both.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `acme_account_safety_buffer` | integer (default: 2592000) | no | The amount of time that must pass after creation that an account with no orders is marked revoked, and the amount of time after being marked revoked or deactivated. |
| `issuer_safety_buffer` | integer (default: 31536000) | no | The amount of extra time that must have passed beyond issuer's expiration before it is removed from the backend storage. Defaults to 8760 hours (1 year). |
| `pause_duration` | string (default: 0s) | no | The amount of time to wait between processing certificates. This allows operators to change the execution profile of tidy to take consume less resources by slowing down how long it takes to run. Note that the entire list of certificates will be stored in memory during the entire tidy operation, but resources to read/process/update existing entries will be spread out over a greater period of time. By default this is zero seconds. |
| `revocation_queue_safety_buffer` | integer (default: 172800) | no | The amount of time that must pass from the cross-cluster revocation request being initiated to when it will be slated for removal. Setting this too low may remove valid revocation requests before the owning cluster has a chance to process them, especially if the cluster is offline. |
| `safety_buffer` | integer (default: 259200) | no | The amount of extra time that must have passed beyond certificate expiration before it is removed from the backend storage and/or revocation list. Defaults to 72 hours. |
| `tidy_acme` | boolean (default: False) | no | Set to true to enable tidying ACME accounts, orders and authorizations. ACME orders are tidied (deleted) safety_buffer after the certificate associated with them expires, or after the order and relevant authorizations have expired if no certificate was produced. Authorizations are tidied with the corresponding order. When a valid ACME Account is at least acme_account_safety_buffer old, and has no remaining orders associated with it, the account is marked as revoked. After another acme_account_safety_buffer has passed from the revocation or deactivation date, a revoked or deactivated ACME account is deleted. |
| `tidy_cert_store` | boolean | no | Set to true to enable tidying up the certificate store |
| `tidy_cross_cluster_revoked_certs` | boolean | no | Set to true to enable tidying up the cross-cluster revoked certificate store. Only runs on the active primary node. |
| `tidy_expired_issuers` | boolean | no | Set to true to automatically remove expired issuers past the issuer_safety_buffer. No keys will be removed as part of this operation. |
| `tidy_move_legacy_ca_bundle` | boolean | no | Set to true to move the legacy ca_bundle from /config/ca_bundle to /config/ca_bundle.bak. This prevents downgrades to pre-Vault 1.11 versions (as older PKI engines do not know about the new multi-issuer storage layout), but improves the performance on seal wrapped PKI mounts. This will only occur if at least issuer_safety_buffer time has occurred after the initial storage migration. This backup is saved in case of an issue in future migrations. Operators may consider removing it via sys/raw if they desire. The backup will be removed via a DELETE /root call, but note that this removes ALL issuers within the mount (and is thus not desirable in most operational scenarios). |
| `tidy_revocation_list` | boolean | no | Deprecated; synonym for 'tidy_revoked_certs |
| `tidy_revocation_queue` | boolean (default: False) | no | Set to true to remove stale revocation queue entries that haven't been confirmed by any active cluster. Only runs on the active primary node |
| `tidy_revoked_cert_issuer_associations` | boolean | no | Set to true to validate issuer associations on revocation entries. This helps increase the performance of CRL building and OCSP responses. |
| `tidy_revoked_certs` | boolean | no | Set to true to expire all revoked and expired certificates, removing them both from the CRL and from storage. The CRL will be rotated if this causes any values to be removed. |




#### Responses


**202**: Accepted



### POST /{pki_mount_path}/tidy-cancel

**Operation ID:** `pki-tidy-cancel`


Cancels a currently running tidy operation.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `acme_account_deleted_count` | integer | no | The number of revoked acme accounts removed |
| `acme_account_revoked_count` | integer | no | The number of unused acme accounts revoked |
| `acme_account_safety_buffer` | integer | no | Safety buffer after creation after which accounts lacking orders are revoked |
| `acme_orders_deleted_count` | integer | no | The number of expired, unused acme orders removed |
| `cert_store_deleted_count` | integer | no | The number of certificate storage entries deleted |
| `cross_revoked_cert_deleted_count` | integer | no |  |
| `current_cert_store_count` | integer | no | The number of revoked certificate entries deleted |
| `current_revoked_cert_count` | integer | no | The number of revoked certificate entries deleted |
| `error` | string | no | The error message |
| `internal_backend_uuid` | string | no |  |
| `issuer_safety_buffer` | integer | no | Issuer safety buffer |
| `last_auto_tidy_finished` | string | no | Time the last auto-tidy operation finished |
| `message` | string | no | Message of the operation |
| `missing_issuer_cert_count` | integer | no |  |
| `pause_duration` | string | no | Duration to pause between tidying certificates |
| `revocation_queue_deleted_count` | integer | no |  |
| `revocation_queue_safety_buffer` | integer | no | Revocation queue safety buffer |
| `revoked_cert_deleted_count` | integer | no | The number of revoked certificate entries deleted |
| `safety_buffer` | integer | no | Safety buffer time duration |
| `state` | string | no | One of Inactive, Running, Finished, or Error |
| `tidy_acme` | boolean | no | Tidy Unused Acme Accounts, and Orders |
| `tidy_cert_store` | boolean | no | Tidy certificate store |
| `tidy_cross_cluster_revoked_certs` | boolean | no | Tidy the cross-cluster revoked certificate store |
| `tidy_expired_issuers` | boolean | no | Tidy expired issuers |
| `tidy_move_legacy_ca_bundle` | boolean | no |  |
| `tidy_revocation_queue` | boolean | no |  |
| `tidy_revoked_cert_issuer_associations` | boolean | no | Tidy revoked certificate issuer associations |
| `tidy_revoked_certs` | boolean | no | Tidy revoked certificates |
| `time_finished` | string | no | Time the operation finished |
| `time_started` | string | no | Time the operation started |
| `total_acme_account_count` | integer | no | Total number of acme accounts iterated over |





### GET /{pki_mount_path}/tidy-status

**Operation ID:** `pki-tidy-status`


Returns the status of the tidy operation.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `acme_account_deleted_count` | integer | no | The number of revoked acme accounts removed |
| `acme_account_revoked_count` | integer | no | The number of unused acme accounts revoked |
| `acme_account_safety_buffer` | integer | no | Safety buffer after creation after which accounts lacking orders are revoked |
| `acme_orders_deleted_count` | integer | no | The number of expired, unused acme orders removed |
| `cert_store_deleted_count` | integer | no | The number of certificate storage entries deleted |
| `cross_revoked_cert_deleted_count` | integer | no |  |
| `current_cert_store_count` | integer | no | The number of revoked certificate entries deleted |
| `current_revoked_cert_count` | integer | no | The number of revoked certificate entries deleted |
| `error` | string | no | The error message |
| `internal_backend_uuid` | string | no |  |
| `issuer_safety_buffer` | integer | no | Issuer safety buffer |
| `last_auto_tidy_finished` | string | no | Time the last auto-tidy operation finished |
| `message` | string | no | Message of the operation |
| `missing_issuer_cert_count` | integer | no |  |
| `pause_duration` | string | no | Duration to pause between tidying certificates |
| `revocation_queue_deleted_count` | integer | no |  |
| `revocation_queue_safety_buffer` | integer | no | Revocation queue safety buffer |
| `revoked_cert_deleted_count` | integer | no | The number of revoked certificate entries deleted |
| `safety_buffer` | integer | no | Safety buffer time duration |
| `state` | string | no | One of Inactive, Running, Finished, or Error |
| `tidy_acme` | boolean | no | Tidy Unused Acme Accounts, and Orders |
| `tidy_cert_store` | boolean | no | Tidy certificate store |
| `tidy_cross_cluster_revoked_certs` | boolean | no | Tidy the cross-cluster revoked certificate store |
| `tidy_expired_issuers` | boolean | no | Tidy expired issuers |
| `tidy_move_legacy_ca_bundle` | boolean | no |  |
| `tidy_revocation_queue` | boolean | no |  |
| `tidy_revoked_cert_issuer_associations` | boolean | no | Tidy revoked certificate issuer associations |
| `tidy_revoked_certs` | boolean | no | Tidy revoked certificates |
| `time_finished` | string | no | Time the operation finished |
| `time_started` | string | no | Time the operation started |
| `total_acme_account_count` | integer | no | Total number of acme accounts iterated over |





### GET /{pki_mount_path}/unified-crl

**Operation ID:** `pki-read-unified-crl-der`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{pki_mount_path}/unified-crl/delta

**Operation ID:** `pki-read-unified-crl-delta`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{pki_mount_path}/unified-crl/delta/pem

**Operation ID:** `pki-read-unified-crl-delta-pem`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{pki_mount_path}/unified-crl/pem

**Operation ID:** `pki-read-unified-crl-pem`


Fetch a CA, CRL, CA Chain, or non-revoked certificate.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{pki_mount_path}/unified-ocsp

**Operation ID:** `pki-query-unified-ocsp`


Query a certificate's revocation status through OCSP'


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{pki_mount_path}/unified-ocsp/{req}

**Operation ID:** `pki-query-unified-ocsp-with-get-req`


Query a certificate's revocation status through OCSP'


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `req` | string | path | yes | base-64 encoded ocsp request |
| `pki_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{rabbitmq_mount_path}/config/connection

**Operation ID:** `rabbit-mq-configure-connection`


Configure the connection URI, username, and password to talk to RabbitMQ management HTTP API.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `rabbitmq_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `connection_uri` | string | no | RabbitMQ Management URI |
| `password` | string | no | Password of the provided RabbitMQ management user |
| `password_policy` | string | no | Name of the password policy to use to generate passwords for dynamic credentials. |
| `username` | string | no | Username of a RabbitMQ management administrator |
| `username_template` | string | no | Template describing how dynamic usernames are generated. |
| `verify_connection` | boolean (default: True) | no | If set, connection_uri is verified by actually connecting to the RabbitMQ management API |




#### Responses


**200**: OK



### GET /{rabbitmq_mount_path}/config/lease

**Operation ID:** `rabbit-mq-read-lease-configuration`


Configure the lease parameters for generated credentials


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `rabbitmq_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{rabbitmq_mount_path}/config/lease

**Operation ID:** `rabbit-mq-configure-lease`


Configure the lease parameters for generated credentials


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `rabbitmq_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `max_ttl` | integer (default: 0) | no | Duration after which the issued credentials should not be allowed to be renewed |
| `ttl` | integer (default: 0) | no | Duration before which the issued credentials needs renewal |




#### Responses


**200**: OK



### GET /{rabbitmq_mount_path}/creds/{name}

**Operation ID:** `rabbit-mq-request-credentials`


Request RabbitMQ credentials for a certain role.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `rabbitmq_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{rabbitmq_mount_path}/roles

**Operation ID:** `rabbit-mq-list-roles`


Manage the roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `rabbitmq_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /{rabbitmq_mount_path}/roles/{name}

**Operation ID:** `rabbit-mq-read-role`


Manage the roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `rabbitmq_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{rabbitmq_mount_path}/roles/{name}

**Operation ID:** `rabbit-mq-write-role`


Manage the roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `rabbitmq_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `tags` | string | no | Comma-separated list of tags for this role. |
| `vhost_topics` | string | no | A nested map of virtual hosts and exchanges to topic permissions. |
| `vhosts` | string | no | A map of virtual hosts to permissions. |




#### Responses


**200**: OK



### DELETE /{rabbitmq_mount_path}/roles/{name}

**Operation ID:** `rabbit-mq-delete-role`


Manage the roles that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the role. |
| `rabbitmq_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### GET /{ssh_mount_path}/config/ca

**Operation ID:** `ssh-read-ca-configuration`


Set the SSH private key used for signing certificates.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{ssh_mount_path}/config/ca

**Operation ID:** `ssh-configure-ca`


Set the SSH private key used for signing certificates.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `generate_signing_key` | boolean (default: True) | no | Generate SSH key pair internally rather than use the private_key and public_key fields. |
| `key_bits` | integer (default: 0) | no | Specifies the desired key bits when generating variable-length keys (such as when key_type="ssh-rsa") or which NIST P-curve to use when key_type="ec" (256, 384, or 521). |
| `key_type` | string (default: ssh-rsa) | no | Specifies the desired key type when generating; could be a OpenSSH key type identifier (ssh-rsa, ecdsa-sha2-nistp256, ecdsa-sha2-nistp384, ecdsa-sha2-nistp521, or ssh-ed25519) or an algorithm (rsa, ec, ed25519). |
| `private_key` | string | no | Private half of the SSH key that will be used to sign certificates. |
| `public_key` | string | no | Public half of the SSH key that will be used to sign certificates. |




#### Responses


**200**: OK



### DELETE /{ssh_mount_path}/config/ca

**Operation ID:** `ssh-delete-ca-configuration`


Set the SSH private key used for signing certificates.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### GET /{ssh_mount_path}/config/zeroaddress

**Operation ID:** `ssh-read-zero-address-configuration`


Assign zero address as default CIDR block for select roles.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{ssh_mount_path}/config/zeroaddress

**Operation ID:** `ssh-configure-zero-address`


Assign zero address as default CIDR block for select roles.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `roles` | array | no | [Required] Comma separated list of role names which allows credentials to be requested for any IP address. CIDR blocks previously registered under these roles will be ignored. |




#### Responses


**200**: OK



### DELETE /{ssh_mount_path}/config/zeroaddress

**Operation ID:** `ssh-delete-zero-address-configuration`


Assign zero address as default CIDR block for select roles.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /{ssh_mount_path}/creds/{role}

**Operation ID:** `ssh-generate-credentials`


Creates a credential for establishing SSH connection with the remote host.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | [Required] Name of the role |
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ip` | string | no | [Required] IP of the remote host |
| `username` | string | no | [Optional] Username in remote host |




#### Responses


**200**: OK



### POST /{ssh_mount_path}/issue/{role}

**Operation ID:** `ssh-issue-certificate`


Request a certificate using a certain role with the provided details.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | The desired role with configuration for this request. |
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `cert_type` | string (default: user) | no | Type of certificate to be created; either "user" or "host". |
| `critical_options` | object | no | Critical options that the certificate should be signed for. |
| `extensions` | object | no | Extensions that the certificate should be signed for. |
| `key_bits` | integer (default: 0) | no | Specifies the number of bits to use for the generated keys. |
| `key_id` | string | no | Key id that the created certificate should have. If not specified, the display name of the token will be used. |
| `key_type` | string (default: rsa) | no | Specifies the desired key type; must be `rsa`, `ed25519` or `ec` |
| `ttl` | integer | no | The requested Time To Live for the SSH certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be later than the role max TTL. |
| `valid_principals` | string | no | Valid principals, either usernames or hostnames, that the certificate should be signed for. |




#### Responses


**200**: OK



### POST /{ssh_mount_path}/lookup

**Operation ID:** `ssh-list-roles-by-ip`


List all the roles associated with the given IP address.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ip` | string | no | [Required] IP address of remote host |




#### Responses


**200**: OK



### GET /{ssh_mount_path}/public_key

**Operation ID:** `ssh-read-public-key`


Retrieve the public key.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{ssh_mount_path}/roles

**Operation ID:** `ssh-list-roles`


Manage the 'roles' that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /{ssh_mount_path}/roles/{role}

**Operation ID:** `ssh-read-role`


Manage the 'roles' that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | [Required for all types] Name of the role being created. |
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{ssh_mount_path}/roles/{role}

**Operation ID:** `ssh-write-role`


Manage the 'roles' that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | [Required for all types] Name of the role being created. |
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm_signer` | string (, default, ssh-rsa, rsa-sha2-256, rsa-sha2-512) | no | [Not applicable for OTP type] [Optional for CA type] When supplied, this value specifies a signing algorithm for the key. Possible values: ssh-rsa, rsa-sha2-256, rsa-sha2-512, default, or the empty string. |
| `allow_bare_domains` | boolean | no | [Not applicable for OTP type] [Optional for CA type] If set, host certificates that are requested are allowed to use the base domains listed in "allowed_domains", e.g. "example.com". This is a separate option as in some cases this can be considered a security threat. |
| `allow_host_certificates` | boolean (default: False) | no | [Not applicable for OTP type] [Optional for CA type] If set, certificates are allowed to be signed for use as a 'host'. |
| `allow_subdomains` | boolean | no | [Not applicable for OTP type] [Optional for CA type] If set, host certificates that are requested are allowed to use subdomains of those listed in "allowed_domains". |
| `allow_user_certificates` | boolean (default: False) | no | [Not applicable for OTP type] [Optional for CA type] If set, certificates are allowed to be signed for use as a 'user'. |
| `allow_user_key_ids` | boolean | no | [Not applicable for OTP type] [Optional for CA type] If true, users can override the key ID for a signed certificate with the "key_id" field. When false, the key ID will always be the token display name. The key ID is logged by the SSH server and can be useful for auditing. |
| `allowed_critical_options` | string | no | [Not applicable for OTP type] [Optional for CA type] A comma-separated list of critical options that certificates can have when signed. To allow any critical options, set this to an empty string. |
| `allowed_domains` | string | no | [Not applicable for OTP type] [Optional for CA type] If this option is not specified, client can request for a signed certificate for any valid host. If only certain domains are allowed, then this list enforces it. |
| `allowed_domains_template` | boolean (default: False) | no | [Not applicable for OTP type] [Optional for CA type] If set, Allowed domains can be specified using identity template policies. Non-templated domains are also permitted. |
| `allowed_extensions` | string | no | [Not applicable for OTP type] [Optional for CA type] A comma-separated list of extensions that certificates can have when signed. An empty list means that no extension overrides are allowed by an end-user; explicitly specify '*' to allow any extensions to be set. |
| `allowed_user_key_lengths` | object | no | [Not applicable for OTP type] [Optional for CA type] If set, allows the enforcement of key types and minimum key sizes to be signed. |
| `allowed_users` | string | no | [Optional for all types] [Works differently for CA type] If this option is not specified, or is '*', client can request a credential for any valid user at the remote host, including the admin user. If only certain usernames are to be allowed, then this list enforces it. If this field is set, then credentials can only be created for default_user and usernames present in this list. Setting this option will enable all the users with access to this role to fetch credentials for all other usernames in this list. Use with caution. N.B.: with the CA type, an empty list means that no users are allowed; explicitly specify '*' to allow any user. |
| `allowed_users_template` | boolean (default: False) | no | [Not applicable for OTP type] [Optional for CA type] If set, Allowed users can be specified using identity template policies. Non-templated users are also permitted. |
| `cidr_list` | string | no | [Optional for OTP type] [Not applicable for CA type] Comma separated list of CIDR blocks for which the role is applicable for. CIDR blocks can belong to more than one role. |
| `default_critical_options` | object | no | [Not applicable for OTP type] [Optional for CA type] Critical options certificates should have if none are provided when signing. This field takes in key value pairs in JSON format. Note that these are not restricted by "allowed_critical_options". Defaults to none. |
| `default_extensions` | object | no | [Not applicable for OTP type] [Optional for CA type] Extensions certificates should have if none are provided when signing. This field takes in key value pairs in JSON format. Note that these are not restricted by "allowed_extensions". Defaults to none. |
| `default_extensions_template` | boolean (default: False) | no | [Not applicable for OTP type] [Optional for CA type] If set, Default extension values can be specified using identity template policies. Non-templated extension values are also permitted. |
| `default_user` | string | no | [Required for OTP type] [Optional for CA type] Default username for which a credential will be generated. When the endpoint 'creds/' is used without a username, this value will be used as default username. |
| `default_user_template` | boolean (default: False) | no | [Not applicable for OTP type] [Optional for CA type] If set, Default user can be specified using identity template policies. Non-templated users are also permitted. |
| `exclude_cidr_list` | string | no | [Optional for OTP type] [Not applicable for CA type] Comma separated list of CIDR blocks. IP addresses belonging to these blocks are not accepted by the role. This is particularly useful when big CIDR blocks are being used by the role and certain parts of it needs to be kept out. |
| `key_id_format` | string | no | [Not applicable for OTP type] [Optional for CA type] When supplied, this value specifies a custom format for the key id of a signed certificate. The following variables are available for use: '{{token_display_name}}' - The display name of the token used to make the request. '{{role_name}}' - The name of the role signing the request. '{{public_key_hash}}' - A SHA256 checksum of the public key that is being signed. |
| `key_type` | string (otp, ca) | no | [Required for all types] Type of key used to login to hosts. It can be either 'otp' or 'ca'. 'otp' type requires agent to be installed in remote hosts. |
| `max_ttl` | integer | no | [Not applicable for OTP type] [Optional for CA type] The maximum allowed lease duration |
| `not_before_duration` | integer (default: 30) | no | [Not applicable for OTP type] [Optional for CA type] The duration that the SSH certificate should be backdated by at issuance. |
| `port` | integer | no | [Optional for OTP type] [Not applicable for CA type] Port number for SSH connection. Default is '22'. Port number does not play any role in creation of OTP. For 'otp' type, this is just a way to inform client about the port number to use. Port number will be returned to client by Vault server along with OTP. |
| `ttl` | integer | no | [Not applicable for OTP type] [Optional for CA type] The lease duration if no specific lease duration is requested. The lease duration controls the expiration of certificates issued by this backend. Defaults to the value of max_ttl. |




#### Responses


**200**: OK



### DELETE /{ssh_mount_path}/roles/{role}

**Operation ID:** `ssh-delete-role`


Manage the 'roles' that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | [Required for all types] Name of the role being created. |
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /{ssh_mount_path}/sign/{role}

**Operation ID:** `ssh-sign-certificate`


Request signing an SSH key using a certain role with the provided details.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `role` | string | path | yes | The desired role with configuration for this request. |
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `cert_type` | string (default: user) | no | Type of certificate to be created; either "user" or "host". |
| `critical_options` | object | no | Critical options that the certificate should be signed for. |
| `extensions` | object | no | Extensions that the certificate should be signed for. |
| `key_id` | string | no | Key id that the created certificate should have. If not specified, the display name of the token will be used. |
| `public_key` | string | no | SSH public key that should be signed. |
| `ttl` | integer | no | The requested Time To Live for the SSH certificate; sets the expiration date. If not specified the role default, backend default, or system default TTL is used, in that order. Cannot be later than the role max TTL. |
| `valid_principals` | string | no | Valid principals, either usernames or hostnames, that the certificate should be signed for. |




#### Responses


**200**: OK



### DELETE /{ssh_mount_path}/tidy/dynamic-keys

**Operation ID:** `ssh-tidy-dynamic-host-keys`


This endpoint removes the stored host keys used for the removed Dynamic Key feature, if present.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /{ssh_mount_path}/verify

**Operation ID:** `ssh-verify-otp`


Validate the OTP provided by Vault SSH Agent.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `ssh_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `otp` | string | no | [Required] One-Time-Key that needs to be validated |




#### Responses


**200**: OK



### GET /{totp_mount_path}/code/{name}

**Operation ID:** `totp-generate-code`


Request time-based one-time use password or validate a password for a certain key .


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key. |
| `totp_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{totp_mount_path}/code/{name}

**Operation ID:** `totp-validate-code`


Request time-based one-time use password or validate a password for a certain key .


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key. |
| `totp_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `code` | string | no | TOTP code to be validated. |




#### Responses


**200**: OK



### GET /{totp_mount_path}/keys

**Operation ID:** `totp-list-keys`


Manage the keys that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `totp_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /{totp_mount_path}/keys/{name}

**Operation ID:** `totp-read-key`


Manage the keys that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key. |
| `totp_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string | query | no | Return a list if `true` |




#### Responses


**200**: OK



### POST /{totp_mount_path}/keys/{name}

**Operation ID:** `totp-create-key`


Manage the keys that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key. |
| `totp_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `account_name` | string | no | The name of the account associated with the key. Required if generate is true. |
| `algorithm` | string (default: SHA1) | no | The hashing algorithm used to generate the TOTP token. Options include SHA1, SHA256 and SHA512. |
| `digits` | integer (default: 6) | no | The number of digits in the generated TOTP token. This value can either be 6 or 8. |
| `exported` | boolean (default: True) | no | Determines if a QR code and url are returned upon generating a key. Only used if generate is true. |
| `generate` | boolean (default: False) | no | Determines if a key should be generated by Vault or if a key is being passed from another service. |
| `issuer` | string | no | The name of the key's issuing organization. Required if generate is true. |
| `key` | string | no | The shared master key used to generate a TOTP token. Only used if generate is false. |
| `key_size` | integer (default: 20) | no | Determines the size in bytes of the generated key. Only used if generate is true. |
| `period` | integer (default: 30) | no | The length of time used to generate a counter for the TOTP token calculation. |
| `qr_size` | integer (default: 200) | no | The pixel size of the generated square QR code. Only used if generate is true and exported is true. If this value is 0, a QR code will not be returned. |
| `skew` | integer (default: 1) | no | The number of delay periods that are allowed when validating a TOTP token. This value can either be 0 or 1. Only used if generate is true. |
| `url` | string | no | A TOTP url string containing all of the parameters for key setup. Only used if generate is false. |




#### Responses


**200**: OK



### DELETE /{totp_mount_path}/keys/{name}

**Operation ID:** `totp-delete-key`


Manage the keys that can be created with this backend.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key. |
| `totp_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### GET /{transit_mount_path}/backup/{name}

**Operation ID:** `transit-back-up-key`


Backup the named key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{transit_mount_path}/byok-export/{destination}/{source}

**Operation ID:** `transit-byok-key`


Securely export named encryption or signing key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `destination` | string | path | yes | Destination key to export to; usually the public wrapping key of another Transit instance. |
| `source` | string | path | yes | Source key to export; could be any present key within Transit. |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{transit_mount_path}/byok-export/{destination}/{source}/{version}

**Operation ID:** `transit-byok-key-version`


Securely export named encryption or signing key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `destination` | string | path | yes | Destination key to export to; usually the public wrapping key of another Transit instance. |
| `source` | string | path | yes | Source key to export; could be any present key within Transit. |
| `version` | string | path | yes | Optional version of the key to export, else all key versions are exported. |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{transit_mount_path}/cache-config

**Operation ID:** `transit-read-cache-configuration`


Returns the size of the active cache


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{transit_mount_path}/cache-config

**Operation ID:** `transit-configure-cache`


Configures a new cache of the specified size


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `size` | integer (default: 0) | no | Size of cache, use 0 for an unlimited cache size, defaults to 0 |




#### Responses


**200**: OK



### GET /{transit_mount_path}/config/keys

**Operation ID:** `transit-read-keys-configuration`


Configuration common across all keys


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{transit_mount_path}/config/keys

**Operation ID:** `transit-configure-keys`


Configuration common across all keys


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `disable_upsert` | boolean | no | Whether to allow automatic upserting (creation) of keys on the encrypt endpoint. |




#### Responses


**200**: OK



### POST /{transit_mount_path}/datakey/{plaintext}/{name}

**Operation ID:** `transit-generate-data-key`


Generate a data key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The backend key used for encrypting the data key |
| `plaintext` | string | path | yes | "plaintext" will return the key in both plaintext and ciphertext; "wrapped" will return the ciphertext only. |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bits` | integer (default: 256) | no | Number of bits for the key; currently 128, 256, and 512 bits are supported. Defaults to 256. |
| `context` | string | no | Context for key derivation. Required for derived keys. |
| `key_version` | integer | no | The version of the Vault key to use for encryption of the data key. Must be 0 (for latest) or a value greater than or equal to the min_encryption_version configured on the key. |
| `nonce` | string | no | Nonce for when convergent encryption v1 is used (only in Vault 0.6.1) |




#### Responses


**200**: OK



### POST /{transit_mount_path}/decrypt/{name}

**Operation ID:** `transit-decrypt`


Decrypt a ciphertext value using a named key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `associated_data` | string | no | When using an AEAD cipher mode, such as AES-GCM, this parameter allows passing associated data (AD/AAD) into the encryption function; this data must be passed on subsequent decryption requests but can be transited in plaintext. On successful decryption, both the ciphertext and the associated data are attested not to have been tampered with. |
| `batch_input` | array | no | Specifies a list of items to be decrypted in a single batch. When this parameter is set, if the parameters 'ciphertext', 'context' and 'nonce' are also set, they will be ignored. Any batch output will preserve the order of the batch input. |
| `ciphertext` | string | no | The ciphertext to decrypt, provided as returned by encrypt. |
| `context` | string | no | Base64 encoded context for key derivation. Required if key derivation is enabled. |
| `nonce` | string | no | Base64 encoded nonce value used during encryption. Must be provided if convergent encryption is enabled for this key and the key was generated with Vault 0.6.1. Not required for keys created in 0.6.2+. |
| `partial_failure_response_code` | integer | no | Ordinarily, if a batch item fails to decrypt due to a bad input, but other batch items succeed, the HTTP response code is 400 (Bad Request). Some applications may want to treat partial failures differently. Providing the parameter returns the given response code integer instead of a 400 in this case. If all values fail HTTP 400 is still returned. |




#### Responses


**200**: OK



### POST /{transit_mount_path}/encrypt/{name}

**Operation ID:** `transit-encrypt`


Encrypt a plaintext value or a batch of plaintext
blocks using a named key


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `associated_data` | string | no | When using an AEAD cipher mode, such as AES-GCM, this parameter allows passing associated data (AD/AAD) into the encryption function; this data must be passed on subsequent decryption requests but can be transited in plaintext. On successful decryption, both the ciphertext and the associated data are attested not to have been tampered with. |
| `batch_input` | array | no | Specifies a list of items to be encrypted in a single batch. When this parameter is set, if the parameters 'plaintext', 'context' and 'nonce' are also set, they will be ignored. Any batch output will preserve the order of the batch input. |
| `context` | string | no | Base64 encoded context for key derivation. Required if key derivation is enabled |
| `convergent_encryption` | boolean | no | This parameter will only be used when a key is expected to be created. Whether to support convergent encryption. This is only supported when using a key with key derivation enabled and will require all requests to carry both a context and 96-bit (12-byte) nonce. The given nonce will be used in place of a randomly generated nonce. As a result, when the same context and nonce are supplied, the same ciphertext is generated. It is *very important* when using this mode that you ensure that all nonces are unique for a given context. Failing to do so will severely impact the ciphertext's security. |
| `key_version` | integer | no | The version of the key to use for encryption. Must be 0 (for latest) or a value greater than or equal to the min_encryption_version configured on the key. |
| `nonce` | string | no | Base64 encoded nonce value. Must be provided if convergent encryption is enabled for this key and the key was generated with Vault 0.6.1. Not required for keys created in 0.6.2+. The value must be exactly 96 bits (12 bytes) long and the user must ensure that for any given context (and thus, any given encryption key) this nonce value is **never reused**. |
| `partial_failure_response_code` | integer | no | Ordinarily, if a batch item fails to encrypt due to a bad input, but other batch items succeed, the HTTP response code is 400 (Bad Request). Some applications may want to treat partial failures differently. Providing the parameter returns the given response code integer instead of a 400 in this case. If all values fail HTTP 400 is still returned. |
| `plaintext` | string | no | Base64 encoded plaintext value to be encrypted |
| `type` | string (default: aes256-gcm96) | no | This parameter is required when encryption key is expected to be created. When performing an upsert operation, the type of key to create. Currently, "aes128-gcm96" (symmetric) and "aes256-gcm96" (symmetric) are the only types supported. Defaults to "aes256-gcm96". |




#### Responses


**200**: OK



### GET /{transit_mount_path}/export/{type}/{name}

**Operation ID:** `transit-export-key`


Export named encryption or signing key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |
| `type` | string | path | yes | Type of key to export (encryption-key, signing-key, hmac-key, public-key) |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### GET /{transit_mount_path}/export/{type}/{name}/{version}

**Operation ID:** `transit-export-key-version`


Export named encryption or signing key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |
| `type` | string | path | yes | Type of key to export (encryption-key, signing-key, hmac-key, public-key) |
| `version` | string | path | yes | Version of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{transit_mount_path}/hash

**Operation ID:** `transit-hash`


Generate a hash sum for input data


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm` | string (default: sha2-256) | no | Algorithm to use (POST body parameter). Valid values are: * sha2-224 * sha2-256 * sha2-384 * sha2-512 * sha3-224 * sha3-256 * sha3-384 * sha3-512 Defaults to "sha2-256". |
| `format` | string (default: hex) | no | Encoding format to use. Can be "hex" or "base64". Defaults to "hex". |
| `input` | string | no | The base64-encoded input data |
| `urlalgorithm` | string | no | Algorithm to use (POST URL parameter) |




#### Responses


**200**: OK



### POST /{transit_mount_path}/hash/{urlalgorithm}

**Operation ID:** `transit-hash-with-algorithm`


Generate a hash sum for input data


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `urlalgorithm` | string | path | yes | Algorithm to use (POST URL parameter) |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm` | string (default: sha2-256) | no | Algorithm to use (POST body parameter). Valid values are: * sha2-224 * sha2-256 * sha2-384 * sha2-512 * sha3-224 * sha3-256 * sha3-384 * sha3-512 Defaults to "sha2-256". |
| `format` | string (default: hex) | no | Encoding format to use. Can be "hex" or "base64". Defaults to "hex". |
| `input` | string | no | The base64-encoded input data |




#### Responses


**200**: OK



### POST /{transit_mount_path}/hmac/{name}

**Operation ID:** `transit-generate-hmac`


Generate an HMAC for input data using the named key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The key to use for the HMAC function |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm` | string (default: sha2-256) | no | Algorithm to use (POST body parameter). Valid values are: * sha2-224 * sha2-256 * sha2-384 * sha2-512 * sha3-224 * sha3-256 * sha3-384 * sha3-512 Defaults to "sha2-256". |
| `batch_input` | array | no | Specifies a list of items to be processed in a single batch. When this parameter is set, if the parameter 'input' is also set, it will be ignored. Any batch output will preserve the order of the batch input. |
| `input` | string | no | The base64-encoded input data |
| `key_version` | integer | no | The version of the key to use for generating the HMAC. Must be 0 (for latest) or a value greater than or equal to the min_encryption_version configured on the key. |
| `urlalgorithm` | string | no | Algorithm to use (POST URL parameter) |




#### Responses


**200**: OK



### POST /{transit_mount_path}/hmac/{name}/{urlalgorithm}

**Operation ID:** `transit-generate-hmac-with-algorithm`


Generate an HMAC for input data using the named key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The key to use for the HMAC function |
| `urlalgorithm` | string | path | yes | Algorithm to use (POST URL parameter) |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm` | string (default: sha2-256) | no | Algorithm to use (POST body parameter). Valid values are: * sha2-224 * sha2-256 * sha2-384 * sha2-512 * sha3-224 * sha3-256 * sha3-384 * sha3-512 Defaults to "sha2-256". |
| `batch_input` | array | no | Specifies a list of items to be processed in a single batch. When this parameter is set, if the parameter 'input' is also set, it will be ignored. Any batch output will preserve the order of the batch input. |
| `input` | string | no | The base64-encoded input data |
| `key_version` | integer | no | The version of the key to use for generating the HMAC. Must be 0 (for latest) or a value greater than or equal to the min_encryption_version configured on the key. |




#### Responses


**200**: OK



### GET /{transit_mount_path}/keys

**Operation ID:** `transit-list-keys`


Managed named encryption keys


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /{transit_mount_path}/keys/{name}

**Operation ID:** `transit-read-key`


Managed named encryption keys


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK



### POST /{transit_mount_path}/keys/{name}

**Operation ID:** `transit-create-key`


Managed named encryption keys


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allow_plaintext_backup` | boolean | no | Enables taking a backup of the named key in plaintext format. Once set, this cannot be disabled. |
| `auto_rotate_period` | integer (default: 0) | no | Amount of time the key should live before being automatically rotated. A value of 0 (default) disables automatic rotation for the key. |
| `context` | string | no | Base64 encoded context for key derivation. When reading a key with key derivation enabled, if the key type supports public keys, this will return the public key for the given context. |
| `convergent_encryption` | boolean | no | Whether to support convergent encryption. This is only supported when using a key with key derivation enabled and will require all requests to carry both a context and 96-bit (12-byte) nonce. The given nonce will be used in place of a randomly generated nonce. As a result, when the same context and nonce are supplied, the same ciphertext is generated. It is *very important* when using this mode that you ensure that all nonces are unique for a given context. Failing to do so will severely impact the ciphertext's security. |
| `derived` | boolean | no | Enables key derivation mode. This allows for per-transaction unique keys for encryption operations. |
| `exportable` | boolean | no | Enables keys to be exportable. This allows for all the valid keys in the key ring to be exported. |
| `key_size` | integer (default: 0) | no | The key size in bytes for the algorithm. Only applies to HMAC and must be no fewer than 32 bytes and no more than 512 |
| `managed_key_id` | string | no | The UUID of the managed key to use for this transit key |
| `managed_key_name` | string | no | The name of the managed key to use for this transit key |
| `type` | string (default: aes256-gcm96) | no | The type of key to create. Currently, "aes128-gcm96" (symmetric), "aes256-gcm96" (symmetric), "ecdsa-p256" (asymmetric), "ecdsa-p384" (asymmetric), "ecdsa-p521" (asymmetric), "ed25519" (asymmetric), "rsa-2048" (asymmetric), "rsa-3072" (asymmetric), "rsa-4096" (asymmetric) are supported. Defaults to "aes256-gcm96". |




#### Responses


**200**: OK



### DELETE /{transit_mount_path}/keys/{name}

**Operation ID:** `transit-delete-key`


Managed named encryption keys


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**204**: empty body



### POST /{transit_mount_path}/keys/{name}/config

**Operation ID:** `transit-configure-key`


Configure a named encryption key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allow_plaintext_backup` | boolean | no | Enables taking a backup of the named key in plaintext format. Once set, this cannot be disabled. |
| `auto_rotate_period` | integer | no | Amount of time the key should live before being automatically rotated. A value of 0 disables automatic rotation for the key. |
| `deletion_allowed` | boolean | no | Whether to allow deletion of the key |
| `exportable` | boolean | no | Enables export of the key. Once set, this cannot be disabled. |
| `min_decryption_version` | integer | no | If set, the minimum version of the key allowed to be decrypted. For signing keys, the minimum version allowed to be used for verification. |
| `min_encryption_version` | integer | no | If set, the minimum version of the key allowed to be used for encryption; or for signing keys, to be used for signing. If set to zero, only the latest version of the key is allowed. |




#### Responses


**200**: OK



### POST /{transit_mount_path}/keys/{name}/import

**Operation ID:** `transit-import-key`


Imports an externally-generated key into a new transit key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allow_plaintext_backup` | boolean | no | Enables taking a backup of the named key in plaintext format. Once set, this cannot be disabled. |
| `allow_rotation` | boolean | no | True if the imported key may be rotated within Vault; false otherwise. |
| `auto_rotate_period` | integer (default: 0) | no | Amount of time the key should live before being automatically rotated. A value of 0 (default) disables automatic rotation for the key. |
| `ciphertext` | string | no | The base64-encoded ciphertext of the keys. The AES key should be encrypted using OAEP with the wrapping key and then concatenated with the import key, wrapped by the AES key. |
| `context` | string | no | Base64 encoded context for key derivation. When reading a key with key derivation enabled, if the key type supports public keys, this will return the public key for the given context. |
| `derived` | boolean | no | Enables key derivation mode. This allows for per-transaction unique keys for encryption operations. |
| `exportable` | boolean | no | Enables keys to be exportable. This allows for all the valid keys in the key ring to be exported. |
| `hash_function` | string (default: SHA256) | no | The hash function used as a random oracle in the OAEP wrapping of the user-generated, ephemeral AES key. Can be one of "SHA1", "SHA224", "SHA256" (default), "SHA384", or "SHA512" |
| `public_key` | string | no | The plaintext PEM public key to be imported. If "ciphertext" is set, this field is ignored. |
| `type` | string (default: aes256-gcm96) | no | The type of key being imported. Currently, "aes128-gcm96" (symmetric), "aes256-gcm96" (symmetric), "ecdsa-p256" (asymmetric), "ecdsa-p384" (asymmetric), "ecdsa-p521" (asymmetric), "ed25519" (asymmetric), "rsa-2048" (asymmetric), "rsa-3072" (asymmetric), "rsa-4096" (asymmetric) are supported. Defaults to "aes256-gcm96". |




#### Responses


**200**: OK



### POST /{transit_mount_path}/keys/{name}/import_version

**Operation ID:** `transit-import-key-version`


Imports an externally-generated key into an existing imported key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `ciphertext` | string | no | The base64-encoded ciphertext of the keys. The AES key should be encrypted using OAEP with the wrapping key and then concatenated with the import key, wrapped by the AES key. |
| `hash_function` | string (default: SHA256) | no | The hash function used as a random oracle in the OAEP wrapping of the user-generated, ephemeral AES key. Can be one of "SHA1", "SHA224", "SHA256" (default), "SHA384", or "SHA512" |
| `public_key` | string | no | The plaintext public key to be imported. If "ciphertext" is set, this field is ignored. |
| `version` | integer | no | Key version to be updated, if left empty, a new version will be created unless a private key is specified and the 'Latest' key is missing a private key. |




#### Responses


**200**: OK



### POST /{transit_mount_path}/keys/{name}/rotate

**Operation ID:** `transit-rotate-key`


Rotate named encryption key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `managed_key_id` | string | no | The UUID of the managed key to use for the new version of this transit key |
| `managed_key_name` | string | no | The name of the managed key to use for the new version of this transit key |




#### Responses


**200**: OK



### POST /{transit_mount_path}/keys/{name}/trim

**Operation ID:** `transit-trim-key`


Trim key versions of a named key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `min_available_version` | integer | no | The minimum available version for the key ring. All versions before this version will be permanently deleted. This value can at most be equal to the lesser of 'min_decryption_version' and 'min_encryption_version'. This is not allowed to be set when either 'min_encryption_version' or 'min_decryption_version' is set to zero. |




#### Responses


**200**: OK



### POST /{transit_mount_path}/random

**Operation ID:** `transit-generate-random`


Generate random bytes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bytes` | integer (default: 32) | no | The number of bytes to generate (POST body parameter). Defaults to 32 (256 bits). |
| `format` | string (default: base64) | no | Encoding format to use. Can be "hex" or "base64". Defaults to "base64". |
| `source` | string (default: platform) | no | Which system to source random data from, ether "platform", "seal", or "all". |
| `urlbytes` | string | no | The number of bytes to generate (POST URL parameter) |




#### Responses


**200**: OK



### POST /{transit_mount_path}/random/{source}

**Operation ID:** `transit-generate-random-with-source`


Generate random bytes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `source` | string | path | yes | Which system to source random data from, ether "platform", "seal", or "all". |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bytes` | integer (default: 32) | no | The number of bytes to generate (POST body parameter). Defaults to 32 (256 bits). |
| `format` | string (default: base64) | no | Encoding format to use. Can be "hex" or "base64". Defaults to "base64". |
| `urlbytes` | string | no | The number of bytes to generate (POST URL parameter) |




#### Responses


**200**: OK



### POST /{transit_mount_path}/random/{source}/{urlbytes}

**Operation ID:** `transit-generate-random-with-source-and-bytes`


Generate random bytes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `source` | string | path | yes | Which system to source random data from, ether "platform", "seal", or "all". |
| `urlbytes` | string | path | yes | The number of bytes to generate (POST URL parameter) |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bytes` | integer (default: 32) | no | The number of bytes to generate (POST body parameter). Defaults to 32 (256 bits). |
| `format` | string (default: base64) | no | Encoding format to use. Can be "hex" or "base64". Defaults to "base64". |




#### Responses


**200**: OK



### POST /{transit_mount_path}/random/{urlbytes}

**Operation ID:** `transit-generate-random-with-bytes`


Generate random bytes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `urlbytes` | string | path | yes | The number of bytes to generate (POST URL parameter) |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bytes` | integer (default: 32) | no | The number of bytes to generate (POST body parameter). Defaults to 32 (256 bits). |
| `format` | string (default: base64) | no | Encoding format to use. Can be "hex" or "base64". Defaults to "base64". |
| `source` | string (default: platform) | no | Which system to source random data from, ether "platform", "seal", or "all". |




#### Responses


**200**: OK



### POST /{transit_mount_path}/restore

**Operation ID:** `transit-restore-key`


Restore the named key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `backup` | string | no | Backed up key data to be restored. This should be the output from the 'backup/' endpoint. |
| `force` | boolean (default: False) | no | If set and a key by the given name exists, force the restore operation and override the key. |
| `name` | string | no | If set, this will be the name of the restored key. |




#### Responses


**200**: OK



### POST /{transit_mount_path}/restore/{name}

**Operation ID:** `transit-restore-and-rename-key`


Restore the named key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | If set, this will be the name of the restored key. |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `backup` | string | no | Backed up key data to be restored. This should be the output from the 'backup/' endpoint. |
| `force` | boolean (default: False) | no | If set and a key by the given name exists, force the restore operation and override the key. |




#### Responses


**200**: OK



### POST /{transit_mount_path}/rewrap/{name}

**Operation ID:** `transit-rewrap`


Rewrap ciphertext


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the key |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `batch_input` | array | no | Specifies a list of items to be re-encrypted in a single batch. When this parameter is set, if the parameters 'ciphertext', 'context' and 'nonce' are also set, they will be ignored. Any batch output will preserve the order of the batch input. |
| `ciphertext` | string | no | Ciphertext value to rewrap |
| `context` | string | no | Base64 encoded context for key derivation. Required for derived keys. |
| `key_version` | integer | no | The version of the key to use for encryption. Must be 0 (for latest) or a value greater than or equal to the min_encryption_version configured on the key. |
| `nonce` | string | no | Nonce for when convergent encryption is used |




#### Responses


**200**: OK



### POST /{transit_mount_path}/sign/{name}

**Operation ID:** `transit-sign`


Generate a signature for input data using the named key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The key to use |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm` | string (default: sha2-256) | no | Deprecated: use "hash_algorithm" instead. |
| `batch_input` | array | no | Specifies a list of items for processing. When this parameter is set, any supplied 'input' or 'context' parameters will be ignored. Responses are returned in the 'batch_results' array component of the 'data' element of the response. Any batch output will preserve the order of the batch input |
| `context` | string | no | Base64 encoded context for key derivation. Required if key derivation is enabled; currently only available with ed25519 keys. |
| `hash_algorithm` | string (default: sha2-256) | no | Hash algorithm to use (POST body parameter). Valid values are: * sha1 * sha2-224 * sha2-256 * sha2-384 * sha2-512 * sha3-224 * sha3-256 * sha3-384 * sha3-512 * none Defaults to "sha2-256". Not valid for all key types, including ed25519. Using none requires setting prehashed=true and signature_algorithm=pkcs1v15, yielding a PKCSv1_5_NoOID instead of the usual PKCSv1_5_DERnull signature. |
| `input` | string | no | The base64-encoded input data |
| `key_version` | integer | no | The version of the key to use for signing. Must be 0 (for latest) or a value greater than or equal to the min_encryption_version configured on the key. |
| `marshaling_algorithm` | string (default: asn1) | no | The method by which to marshal the signature. The default is 'asn1' which is used by openssl and X.509. It can also be set to 'jws' which is used for JWT signatures; setting it to this will also cause the encoding of the signature to be url-safe base64 instead of using standard base64 encoding. Currently only valid for ECDSA P-256 key types". |
| `prehashed` | boolean | no | Set to 'true' when the input is already hashed. If the key type is 'rsa-2048', 'rsa-3072' or 'rsa-4096', then the algorithm used to hash the input should be indicated by the 'algorithm' parameter. |
| `salt_length` | string (default: auto) | no | The salt length used to sign. Currently only applies to the RSA PSS signature scheme. Options are 'auto' (the default used by Golang, causing the salt to be as large as possible when signing), 'hash' (causes the salt length to equal the length of the hash used in the signature), or an integer between the minimum and the maximum permissible salt lengths for the given RSA key size. Defaults to 'auto'. |
| `signature_algorithm` | string | no | The signature algorithm to use for signing. Currently only applies to RSA key types. Options are 'pss' or 'pkcs1v15'. Defaults to 'pss' |
| `urlalgorithm` | string | no | Hash algorithm to use (POST URL parameter) |




#### Responses


**200**: OK



### POST /{transit_mount_path}/sign/{name}/{urlalgorithm}

**Operation ID:** `transit-sign-with-algorithm`


Generate a signature for input data using the named key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The key to use |
| `urlalgorithm` | string | path | yes | Hash algorithm to use (POST URL parameter) |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm` | string (default: sha2-256) | no | Deprecated: use "hash_algorithm" instead. |
| `batch_input` | array | no | Specifies a list of items for processing. When this parameter is set, any supplied 'input' or 'context' parameters will be ignored. Responses are returned in the 'batch_results' array component of the 'data' element of the response. Any batch output will preserve the order of the batch input |
| `context` | string | no | Base64 encoded context for key derivation. Required if key derivation is enabled; currently only available with ed25519 keys. |
| `hash_algorithm` | string (default: sha2-256) | no | Hash algorithm to use (POST body parameter). Valid values are: * sha1 * sha2-224 * sha2-256 * sha2-384 * sha2-512 * sha3-224 * sha3-256 * sha3-384 * sha3-512 * none Defaults to "sha2-256". Not valid for all key types, including ed25519. Using none requires setting prehashed=true and signature_algorithm=pkcs1v15, yielding a PKCSv1_5_NoOID instead of the usual PKCSv1_5_DERnull signature. |
| `input` | string | no | The base64-encoded input data |
| `key_version` | integer | no | The version of the key to use for signing. Must be 0 (for latest) or a value greater than or equal to the min_encryption_version configured on the key. |
| `marshaling_algorithm` | string (default: asn1) | no | The method by which to marshal the signature. The default is 'asn1' which is used by openssl and X.509. It can also be set to 'jws' which is used for JWT signatures; setting it to this will also cause the encoding of the signature to be url-safe base64 instead of using standard base64 encoding. Currently only valid for ECDSA P-256 key types". |
| `prehashed` | boolean | no | Set to 'true' when the input is already hashed. If the key type is 'rsa-2048', 'rsa-3072' or 'rsa-4096', then the algorithm used to hash the input should be indicated by the 'algorithm' parameter. |
| `salt_length` | string (default: auto) | no | The salt length used to sign. Currently only applies to the RSA PSS signature scheme. Options are 'auto' (the default used by Golang, causing the salt to be as large as possible when signing), 'hash' (causes the salt length to equal the length of the hash used in the signature), or an integer between the minimum and the maximum permissible salt lengths for the given RSA key size. Defaults to 'auto'. |
| `signature_algorithm` | string | no | The signature algorithm to use for signing. Currently only applies to RSA key types. Options are 'pss' or 'pkcs1v15'. Defaults to 'pss' |




#### Responses


**200**: OK



### POST /{transit_mount_path}/verify/{name}

**Operation ID:** `transit-verify`


Verify a signature or HMAC for input data created using the named key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The key to use |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm` | string (default: sha2-256) | no | Deprecated: use "hash_algorithm" instead. |
| `batch_input` | array | no | Specifies a list of items for processing. When this parameter is set, any supplied 'input', 'hmac' or 'signature' parameters will be ignored. Responses are returned in the 'batch_results' array component of the 'data' element of the response. Any batch output will preserve the order of the batch input |
| `context` | string | no | Base64 encoded context for key derivation. Required if key derivation is enabled; currently only available with ed25519 keys. |
| `hash_algorithm` | string (default: sha2-256) | no | Hash algorithm to use (POST body parameter). Valid values are: * sha1 * sha2-224 * sha2-256 * sha2-384 * sha2-512 * sha3-224 * sha3-256 * sha3-384 * sha3-512 * none Defaults to "sha2-256". Not valid for all key types. See note about none on signing path. |
| `hmac` | string | no | The HMAC, including vault header/key version |
| `input` | string | no | The base64-encoded input data to verify |
| `marshaling_algorithm` | string (default: asn1) | no | The method by which to unmarshal the signature when verifying. The default is 'asn1' which is used by openssl and X.509; can also be set to 'jws' which is used for JWT signatures in which case the signature is also expected to be url-safe base64 encoding instead of standard base64 encoding. Currently only valid for ECDSA P-256 key types". |
| `prehashed` | boolean | no | Set to 'true' when the input is already hashed. If the key type is 'rsa-2048', 'rsa-3072' or 'rsa-4096', then the algorithm used to hash the input should be indicated by the 'algorithm' parameter. |
| `salt_length` | string (default: auto) | no | The salt length used to sign. Currently only applies to the RSA PSS signature scheme. Options are 'auto' (the default used by Golang, causing the salt to be as large as possible when signing), 'hash' (causes the salt length to equal the length of the hash used in the signature), or an integer between the minimum and the maximum permissible salt lengths for the given RSA key size. Defaults to 'auto'. |
| `signature` | string | no | The signature, including vault header/key version |
| `signature_algorithm` | string | no | The signature algorithm to use for signature verification. Currently only applies to RSA key types. Options are 'pss' or 'pkcs1v15'. Defaults to 'pss' |
| `urlalgorithm` | string | no | Hash algorithm to use (POST URL parameter) |




#### Responses


**200**: OK



### POST /{transit_mount_path}/verify/{name}/{urlalgorithm}

**Operation ID:** `transit-verify-with-algorithm`


Verify a signature or HMAC for input data created using the named key


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The key to use |
| `urlalgorithm` | string | path | yes | Hash algorithm to use (POST URL parameter) |
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm` | string (default: sha2-256) | no | Deprecated: use "hash_algorithm" instead. |
| `batch_input` | array | no | Specifies a list of items for processing. When this parameter is set, any supplied 'input', 'hmac' or 'signature' parameters will be ignored. Responses are returned in the 'batch_results' array component of the 'data' element of the response. Any batch output will preserve the order of the batch input |
| `context` | string | no | Base64 encoded context for key derivation. Required if key derivation is enabled; currently only available with ed25519 keys. |
| `hash_algorithm` | string (default: sha2-256) | no | Hash algorithm to use (POST body parameter). Valid values are: * sha1 * sha2-224 * sha2-256 * sha2-384 * sha2-512 * sha3-224 * sha3-256 * sha3-384 * sha3-512 * none Defaults to "sha2-256". Not valid for all key types. See note about none on signing path. |
| `hmac` | string | no | The HMAC, including vault header/key version |
| `input` | string | no | The base64-encoded input data to verify |
| `marshaling_algorithm` | string (default: asn1) | no | The method by which to unmarshal the signature when verifying. The default is 'asn1' which is used by openssl and X.509; can also be set to 'jws' which is used for JWT signatures in which case the signature is also expected to be url-safe base64 encoding instead of standard base64 encoding. Currently only valid for ECDSA P-256 key types". |
| `prehashed` | boolean | no | Set to 'true' when the input is already hashed. If the key type is 'rsa-2048', 'rsa-3072' or 'rsa-4096', then the algorithm used to hash the input should be indicated by the 'algorithm' parameter. |
| `salt_length` | string (default: auto) | no | The salt length used to sign. Currently only applies to the RSA PSS signature scheme. Options are 'auto' (the default used by Golang, causing the salt to be as large as possible when signing), 'hash' (causes the salt length to equal the length of the hash used in the signature), or an integer between the minimum and the maximum permissible salt lengths for the given RSA key size. Defaults to 'auto'. |
| `signature` | string | no | The signature, including vault header/key version |
| `signature_algorithm` | string | no | The signature algorithm to use for signature verification. Currently only applies to RSA key types. Options are 'pss' or 'pkcs1v15'. Defaults to 'pss' |




#### Responses


**200**: OK



### GET /{transit_mount_path}/wrapping_key

**Operation ID:** `transit-read-wrapping-key`


Returns the public key to use for wrapping imported keys


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `transit_mount_path` | string | path | yes | Path that the backend was mounted at |




#### Responses


**200**: OK




---


## system


### GET /sys/audit

**Operation ID:** `auditing-list-enabled-devices`


List the enabled audit devices.


**Required sudo:** yes


#### Responses


**200**: OK



### POST /sys/audit-hash/{path}

**Operation ID:** `auditing-calculate-hash`


The hash of the given string via the given audit backend


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The name of the backend. Cannot be delimited. Example: "mysql" |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `input` | string | no |  |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `hash` | string | no |  |





### POST /sys/audit/{path}

**Operation ID:** `auditing-enable-device`


Enable a new audit device at the supplied path.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The name of the backend. Cannot be delimited. Example: "mysql" |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `description` | string | no | User-friendly description for this audit backend. |
| `local` | boolean (default: False) | no | Mark the mount as a local mount, which is not replicated and is unaffected by replication. |
| `options` | object | no | Configuration options for the audit backend. |
| `type` | string | no | The type of the backend. Example: "mysql" |




#### Responses


**204**: OK



### DELETE /sys/audit/{path}

**Operation ID:** `auditing-disable-device`


Disable the audit device at the given path.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The name of the backend. Cannot be delimited. Example: "mysql" |




#### Responses


**204**: OK



### GET /sys/auth

**Operation ID:** `auth-list-enabled-methods`


List the currently enabled credential backends.


#### Responses


**200**: OK



### GET /sys/auth/{path}

**Operation ID:** `auth-read-configuration`


Read the configuration of the auth engine at the given path.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The path to mount to. Cannot be delimited. Example: "user" |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `accessor` | string | no |  |
| `config` | object | no |  |
| `deprecation_status` | string | no |  |
| `description` | string | no |  |
| `external_entropy_access` | boolean | no |  |
| `local` | boolean | no |  |
| `options` | object | no |  |
| `plugin_version` | string | no |  |
| `running_plugin_version` | string | no |  |
| `running_sha256` | string | no |  |
| `seal_wrap` | boolean | no |  |
| `type` | string | no |  |
| `uuid` | string | no |  |





### POST /sys/auth/{path}

**Operation ID:** `auth-enable-method`


Enables a new auth method.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The path to mount to. Cannot be delimited. Example: "user" |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `config` | object | no | Configuration for this mount, such as plugin_name. |
| `description` | string | no | User-friendly description for this credential backend. |
| `external_entropy_access` | boolean (default: False) | no | Whether to give the mount access to Stronghold's external entropy. |
| `local` | boolean (default: False) | no | Mark the mount as a local mount, which is not replicated and is unaffected by replication. |
| `options` | object | no | The options to pass into the backend. Should be a json object with string keys and values. |
| `plugin_name` | string | no | Name of the auth plugin to use based from the name in the plugin catalog. |
| `plugin_version` | string | no | The semantic version of the plugin to use. |
| `seal_wrap` | boolean (default: False) | no | Whether to turn on seal wrapping for the mount. |
| `type` | string | no | The type of the backend. Example: "userpass" |




#### Responses


**204**: OK



### DELETE /sys/auth/{path}

**Operation ID:** `auth-disable-method`


Disable the auth method at the given auth path


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The path to mount to. Cannot be delimited. Example: "user" |




#### Responses


**204**: OK



### GET /sys/auth/{path}/tune

**Operation ID:** `auth-read-tuning-information`


Reads the given auth path's configuration.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Tune the configuration parameters for an auth path. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allowed_managed_keys` | array | no |  |
| `allowed_response_headers` | array | no |  |
| `audit_non_hmac_request_keys` | array | no |  |
| `audit_non_hmac_response_keys` | array | no |  |
| `default_lease_ttl` | integer | no |  |
| `description` | string | no |  |
| `external_entropy_access` | boolean | no |  |
| `force_no_cache` | boolean | no |  |
| `listing_visibility` | string | no |  |
| `max_lease_ttl` | integer | no |  |
| `options` | object | no |  |
| `passthrough_request_headers` | array | no |  |
| `plugin_version` | string | no |  |
| `token_type` | string | no |  |
| `user_lockout_counter_reset_duration` | integer | no |  |
| `user_lockout_disable` | boolean | no |  |
| `user_lockout_duration` | integer | no |  |
| `user_lockout_threshold` | integer | no |  |





### POST /sys/auth/{path}/tune

**Operation ID:** `auth-tune-configuration-parameters`


Tune configuration parameters for a given auth path.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | Tune the configuration parameters for an auth path. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allowed_response_headers` | array | no | A list of headers to whitelist and allow a plugin to set on responses. |
| `audit_non_hmac_request_keys` | array | no | The list of keys in the request data object that will not be HMAC'ed by audit devices. |
| `audit_non_hmac_response_keys` | array | no | The list of keys in the response data object that will not be HMAC'ed by audit devices. |
| `default_lease_ttl` | string | no | The default lease TTL for this mount. |
| `description` | string | no | User-friendly description for this credential backend. |
| `listing_visibility` | string | no | Determines the visibility of the mount in the UI-specific listing endpoint. Accepted value are 'unauth' and 'hidden', with the empty default ('') behaving like 'hidden'. |
| `max_lease_ttl` | string | no | The max lease TTL for this mount. |
| `options` | object | no | The options to pass into the backend. Should be a json object with string keys and values. |
| `passthrough_request_headers` | array | no | A list of headers to whitelist and pass from the request to the plugin. |
| `plugin_version` | string | no | The semantic version of the plugin to use. |
| `token_type` | string | no | The type of token to issue (service or batch). |
| `user_lockout_config` | object | no | The user lockout configuration to pass into the backend. Should be a json object with string keys and values. |




#### Responses


**204**: OK



### POST /sys/capabilities

**Operation ID:** `query-token-capabilities`


Fetches the capabilities of the given token on the given path.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `path` | array | no | ⚠️ Deprecated. Use 'paths' instead. |
| `paths` | array | no | Paths on which capabilities are being queried. |
| `token` | string | no | Token for which capabilities are being queried. |




#### Responses


**200**: OK



### POST /sys/capabilities-accessor

**Operation ID:** `query-token-accessor-capabilities`


Fetches the capabilities of the token associated with the given token, on the given path.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `accessor` | string | no | Accessor of the token for which capabilities are being queried. |
| `path` | array | no | ⚠️ Deprecated. Use 'paths' instead. |
| `paths` | array | no | Paths on which capabilities are being queried. |




#### Responses


**200**: OK



### POST /sys/capabilities-self

**Operation ID:** `query-token-self-capabilities`


Fetches the capabilities of the given token on the given path.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `path` | array | no | ⚠️ Deprecated. Use 'paths' instead. |
| `paths` | array | no | Paths on which capabilities are being queried. |
| `token` | string | no | Token for which capabilities are being queried. |




#### Responses


**200**: OK



### GET /sys/config/auditing/request-headers

**Operation ID:** `auditing-list-request-headers`


List the request headers that are configured to be audited.


**Required sudo:** yes


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `headers` | object | no |  |





### GET /sys/config/auditing/request-headers/{header}

**Operation ID:** `auditing-read-request-header-information`


List the information for the given request header.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `header` | string | path | yes |  |




#### Responses


**200**: OK



### POST /sys/config/auditing/request-headers/{header}

**Operation ID:** `auditing-enable-request-header`


Enable auditing of a header.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `header` | string | path | yes |  |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `hmac` | boolean | no |  |




#### Responses


**204**: OK



### DELETE /sys/config/auditing/request-headers/{header}

**Operation ID:** `auditing-disable-request-header`


Disable auditing of the given request header.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `header` | string | path | yes |  |




#### Responses


**204**: OK



### GET /sys/config/control-group

**Operation ID:** `enterprise-stub-read-config-control-group`



#### Responses


**200**: OK



### POST /sys/config/control-group

**Operation ID:** `enterprise-stub-write-config-control-group`



#### Responses


**200**: OK



### DELETE /sys/config/control-group

**Operation ID:** `enterprise-stub-delete-config-control-group`



#### Responses


**204**: empty body



### GET /sys/config/cors

**Operation ID:** `cors-read-configuration`


Return the current CORS settings.


**Required sudo:** yes


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allowed_headers` | array | no |  |
| `allowed_origins` | array | no |  |
| `enabled` | boolean | no |  |





### POST /sys/config/cors

**Operation ID:** `cors-configure`


Configure the CORS settings.


**Required sudo:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allowed_headers` | array | no | A comma-separated string or array of strings indicating headers that are allowed on cross-origin requests. |
| `allowed_origins` | array | no | A comma-separated string or array of strings indicating origins that may make cross-origin requests. |
| `enable` | boolean | no | Enables or disables CORS headers on requests. |




#### Responses


**204**: OK



### DELETE /sys/config/cors

**Operation ID:** `cors-delete-configuration`


Remove any CORS settings.


**Required sudo:** yes


#### Responses


**204**: OK



### GET /sys/config/group-policy-application

**Operation ID:** `enterprise-stub-read-config-group-policy-application`



#### Responses


**200**: OK



### POST /sys/config/group-policy-application

**Operation ID:** `enterprise-stub-write-config-group-policy-application`



#### Responses


**200**: OK



### POST /sys/config/reload/{subsystem}

**Operation ID:** `reload-subsystem`


Reload the given subsystem


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `subsystem` | string | path | yes |  |




#### Responses


**204**: OK



### GET /sys/config/state/sanitized

**Operation ID:** `read-sanitized-configuration-state`


Return a sanitized version of the Stronghold server configuration.


#### Responses


**200**: OK



### GET /sys/config/ui/headers

**Operation ID:** `ui-headers-list`


Return a list of configured UI headers.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: 



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no | Lists of configured UI headers. Omitted if list is empty |





### GET /sys/config/ui/headers/{header}

**Operation ID:** `ui-headers-read-configuration`


Return the given UI header's configuration


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `header` | string | path | yes | The name of the header. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `value` | string | no | returns the first header value when `multivalue` request parameter is false |
| `values` | array | no | returns all header values when `multivalue` request parameter is true |





### POST /sys/config/ui/headers/{header}

**Operation ID:** `ui-headers-configure`


Configure the values to be returned for the UI header.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `header` | string | path | yes | The name of the header. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `multivalue` | boolean | no | Returns multiple values if true |
| `values` | array | no | The values to set the header. |




#### Responses


**200**: OK



### DELETE /sys/config/ui/headers/{header}

**Operation ID:** `ui-headers-delete-configuration`


Remove a UI header.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `header` | string | path | yes | The name of the header. |




#### Responses


**204**: OK



### POST /sys/control-group/authorize

**Operation ID:** `enterprise-stub-write-control-group-authorize`



#### Responses


**200**: OK



### POST /sys/control-group/request

**Operation ID:** `enterprise-stub-write-control-group-request`



#### Responses


**200**: OK



### POST /sys/decode-token

**Operation ID:** `decode`


Decodes the encoded token with the otp.


**Available without authentication:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `encoded_token` | string | no | Specifies the encoded token (result from generate-root). |
| `otp` | string | no | Specifies the otp code for decode. |




#### Responses


**200**: OK



### GET /sys/experiments

**Operation ID:** `list-experimental-features`


Returns the available and enabled experiments


#### Responses


**200**: OK



### GET /sys/generate-root

**Operation ID:** `root-token-generation-read-progress2`


Read the configuration and progress of the current root generation attempt.


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `complete` | boolean | no |  |
| `encoded_root_token` | string | no |  |
| `encoded_token` | string | no |  |
| `nonce` | string | no |  |
| `otp` | string | no |  |
| `otp_length` | integer | no |  |
| `pgp_fingerprint` | string | no |  |
| `progress` | integer | no |  |
| `required` | integer | no |  |
| `started` | boolean | no |  |





### POST /sys/generate-root

**Operation ID:** `root-token-generation-initialize-2`


Initializes a new root generation attempt.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `pgp_key` | string | no | Specifies a base64-encoded PGP public key. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `complete` | boolean | no |  |
| `encoded_root_token` | string | no |  |
| `encoded_token` | string | no |  |
| `nonce` | string | no |  |
| `otp` | string | no |  |
| `otp_length` | integer | no |  |
| `pgp_fingerprint` | string | no |  |
| `progress` | integer | no |  |
| `required` | integer | no |  |
| `started` | boolean | no |  |





### DELETE /sys/generate-root

**Operation ID:** `root-token-generation-cancel-2`


Cancels any in-progress root generation attempt.


#### Responses


**204**: OK



### GET /sys/generate-root/attempt

**Operation ID:** `root-token-generation-read-progress`


Read the configuration and progress of the current root generation attempt.


**Available without authentication:** yes


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `complete` | boolean | no |  |
| `encoded_root_token` | string | no |  |
| `encoded_token` | string | no |  |
| `nonce` | string | no |  |
| `otp` | string | no |  |
| `otp_length` | integer | no |  |
| `pgp_fingerprint` | string | no |  |
| `progress` | integer | no |  |
| `required` | integer | no |  |
| `started` | boolean | no |  |





### POST /sys/generate-root/attempt

**Operation ID:** `root-token-generation-initialize`


Initializes a new root generation attempt.


**Available without authentication:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `pgp_key` | string | no | Specifies a base64-encoded PGP public key. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `complete` | boolean | no |  |
| `encoded_root_token` | string | no |  |
| `encoded_token` | string | no |  |
| `nonce` | string | no |  |
| `otp` | string | no |  |
| `otp_length` | integer | no |  |
| `pgp_fingerprint` | string | no |  |
| `progress` | integer | no |  |
| `required` | integer | no |  |
| `started` | boolean | no |  |





### DELETE /sys/generate-root/attempt

**Operation ID:** `root-token-generation-cancel`


Cancels any in-progress root generation attempt.


**Available without authentication:** yes


#### Responses


**204**: OK



### POST /sys/generate-root/update

**Operation ID:** `root-token-generation-update`


Enter a single unseal key share to progress the root generation attempt.


**Available without authentication:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key` | string | no | Specifies a single unseal key share. |
| `nonce` | string | no | Specifies the nonce of the attempt. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `complete` | boolean | no |  |
| `encoded_root_token` | string | no |  |
| `encoded_token` | string | no |  |
| `nonce` | string | no |  |
| `otp` | string | no |  |
| `otp_length` | integer | no |  |
| `pgp_fingerprint` | string | no |  |
| `progress` | integer | no |  |
| `required` | integer | no |  |
| `started` | boolean | no |  |





### GET /sys/ha-status

**Operation ID:** `ha-status`


Check the HA status of a Stronghold cluster


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `nodes` | array | no |  |





### GET /sys/health

**Operation ID:** `read-health-status`


Returns the health status of Stronghold.


**Available without authentication:** yes


#### Responses


**200**: initialized, unsealed, and active



**429**: unsealed and standby



**472**: data recovery mode replication secondary and active



**501**: not initialized



**503**: sealed



### GET /sys/host-info

**Operation ID:** `collect-host-information`


Information about the host instance that this Stronghold server is running on.


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `cpu` | array | no |  |
| `cpu_times` | array | no |  |
| `disk` | array | no |  |
| `host` | object | no |  |
| `memory` | object | no |  |
| `timestamp` | string | no |  |





### GET /sys/in-flight-req

**Operation ID:** `collect-in-flight-request-information`


reports in-flight requests


#### Responses


**200**: OK



### GET /sys/init

**Operation ID:** `read-initialization-status`


Returns the initialization status of Stronghold.


**Available without authentication:** yes


#### Responses


**200**: OK



### POST /sys/init

**Operation ID:** `initialize`


Initialize a new Stronghold.


**Available without authentication:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `pgp_keys` | array | no | Specifies an array of PGP public keys used to encrypt the output unseal keys. Ordering is preserved. The keys must be base64-encoded from their original binary representation. The size of this array must be the same as `secret_shares`. |
| `recovery_pgp_keys` | array | no | Specifies an array of PGP public keys used to encrypt the output recovery keys. Ordering is preserved. The keys must be base64-encoded from their original binary representation. The size of this array must be the same as `recovery_shares`. |
| `recovery_shares` | integer | no | Specifies the number of shares to split the recovery key into. |
| `recovery_threshold` | integer | no | Specifies the number of shares required to reconstruct the recovery key. This must be less than or equal to `recovery_shares`. |
| `root_token_pgp_key` | string | no | Specifies a PGP public key used to encrypt the initial root token. The key must be base64-encoded from its original binary representation. |
| `secret_shares` | integer | no | Specifies the number of shares to split the unseal key into. |
| `secret_threshold` | integer | no | Specifies the number of shares required to reconstruct the unseal key. This must be less than or equal secret_shares. If using Stronghold HSM with auto-unsealing, this value must be the same as `secret_shares`. |
| `stored_shares` | integer | no | Specifies the number of shares that should be encrypted by the HSM and stored for auto-unsealing. Currently must be the same as `secret_shares`. |




#### Responses


**200**: OK



### GET /sys/internal/counters/activity

**Operation ID:** `internal-client-activity-report-counts`


Report the client count metrics, for this namespace and all child namespaces.


#### Responses


**200**: OK



### GET /sys/internal/counters/activity/export

**Operation ID:** `internal-client-activity-export`


Report the client count metrics, for this namespace and all child namespaces.


#### Responses


**200**: OK



### GET /sys/internal/counters/activity/monthly

**Operation ID:** `internal-client-activity-report-counts-this-month`


Report the number of clients for this month, for this namespace and all child namespaces.


#### Responses


**200**: OK



### GET /sys/internal/counters/config

**Operation ID:** `internal-client-activity-read-configuration`


Read the client count tracking configuration.


#### Responses


**200**: OK



### POST /sys/internal/counters/config

**Operation ID:** `internal-client-activity-configure`


Enable or disable collection of client count, set retention period, or set default reporting period.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `default_report_months` | integer (default: 12) | no | Number of months to report if no start date specified. |
| `enabled` | string (default: default) | no | Enable or disable collection of client count: enable, disable, or default. |
| `retention_months` | integer (default: 24) | no | Number of months of client data to retain. Setting to 0 will clear all existing data. |




#### Responses


**200**: OK



### GET /sys/internal/counters/entities

**Operation ID:** `internal-count-entities`


Backwards compatibility is not guaranteed for this API


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `counters` | object | no |  |





### GET /sys/internal/counters/requests

**Operation ID:** `internal-count-requests`


Backwards compatibility is not guaranteed for this API


#### Responses


**200**: OK



### GET /sys/internal/counters/tokens

**Operation ID:** `internal-count-tokens`


Backwards compatibility is not guaranteed for this API


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `counters` | object | no |  |





### GET /sys/internal/inspect/router/{tag}

**Operation ID:** `internal-inspect-router`


Expose the route entry and mount entry tables present in the router


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `tag` | string | path | yes | Name of subtree being observed |




#### Responses


**200**: OK



### GET /sys/internal/specs/openapi

**Operation ID:** `internal-generate-open-api-document`


Generate an OpenAPI 3 document of all mounted paths.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `generic_mount_paths` | boolean | query | no | Use generic mount paths |




#### Responses


**200**: OK



### POST /sys/internal/specs/openapi

**Operation ID:** `internal-generate-open-api-document-with-parameters`


Generate an OpenAPI 3 document of all mounted paths.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `generic_mount_paths` | boolean | query | no | Use generic mount paths |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `context` | string | no | Context string appended to every operationId |




#### Responses


**200**: OK



### GET /sys/internal/ui/feature-flags

**Operation ID:** `internal-ui-list-enabled-feature-flags`


Lists enabled feature flags.


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `feature_flags` | array | no |  |





### GET /sys/internal/ui/mounts

**Operation ID:** `internal-ui-list-enabled-visible-mounts`


Lists all enabled and visible auth and secrets mounts.


**Available without authentication:** yes


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `auth` | object | no | auth mounts |
| `secret` | object | no | secret mounts |





### GET /sys/internal/ui/mounts/{path}

**Operation ID:** `internal-ui-read-mount-information`


Return information about the given mount.


**Available without authentication:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The path of the mount. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `accessor` | string | no |  |
| `config` | object | no |  |
| `description` | string | no |  |
| `external_entropy_access` | boolean | no |  |
| `local` | boolean | no |  |
| `options` | object | no |  |
| `path` | string | no |  |
| `plugin_version` | string | no |  |
| `running_plugin_version` | string | no |  |
| `running_sha256` | string | no |  |
| `seal_wrap` | boolean | no |  |
| `type` | string | no |  |
| `uuid` | string | no |  |





### GET /sys/internal/ui/namespaces

**Operation ID:** `internal-ui-list-namespaces`


Backwards compatibility is not guaranteed for this API


**Available without authentication:** yes


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no | field is only returned if there are one or more namespaces |





### GET /sys/internal/ui/resultant-acl

**Operation ID:** `internal-ui-read-resultant-acl`


Backwards compatibility is not guaranteed for this API


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `exact_paths` | object | no |  |
| `glob_paths` | object | no |  |
| `root` | boolean | no |  |





**204**: empty response returned if no client token



### GET /sys/key-status

**Operation ID:** `encryption-key-status`


Provides information about the backend encryption key.


#### Responses


**200**: OK



### GET /sys/leader

**Operation ID:** `leader-status`


Returns the high availability status and current leader instance of Stronghold.


**Available without authentication:** yes


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `active_time` | string | no |  |
| `ha_enabled` | boolean | no |  |
| `is_self` | boolean | no |  |
| `last_wal` | integer | no |  |
| `leader_address` | string | no |  |
| `leader_cluster_address` | string | no |  |
| `performance_standby` | boolean | no |  |
| `performance_standby_last_remote_wal` | integer | no |  |
| `raft_applied_index` | integer | no |  |
| `raft_committed_index` | integer | no |  |





### GET /sys/leases

**Operation ID:** `leases-list`


List leases associated with this Stronghold cluster


**Required sudo:** yes


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `counts` | integer | no | Number of matching leases per mount |
| `lease_count` | integer | no | Number of matching leases |





### GET /sys/leases/count

**Operation ID:** `leases-count`


Count of leases associated with this Stronghold cluster


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `counts` | integer | no | Number of matching leases per mount |
| `lease_count` | integer | no | Number of matching leases |





### POST /sys/leases/lookup

**Operation ID:** `leases-read-lease`


View or list lease metadata.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `lease_id` | string | no | The lease identifier to renew. This is included with a lease. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `expire_time` | string | no | Optional lease expiry time |
| `id` | string | no | Lease id |
| `issue_time` | string | no | Timestamp for the lease's issue time |
| `last_renewal` | string | no | Optional Timestamp of the last time the lease was renewed |
| `renewable` | boolean | no | True if the lease is able to be renewed |
| `ttl` | integer | no | Time to Live set for the lease, returns 0 if unset |





### GET /sys/leases/lookup/

**Operation ID:** `leases-look-up`


View or list lease metadata.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no | A list of lease ids |





### GET /sys/leases/lookup/{prefix}

**Operation ID:** `leases-look-up-with-prefix`


View or list lease metadata.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `prefix` | string | path | yes | The path to list leases under. Example: "aws/creds/deploy" |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no | A list of lease ids |





### POST /sys/leases/renew

**Operation ID:** `leases-renew-lease`


Renews a lease, requesting to extend the lease.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `increment` | integer | no | The desired increment in seconds to the lease |
| `lease_id` | string | no | The lease identifier to renew. This is included with a lease. |
| `url_lease_id` | string | no | The lease identifier to renew. This is included with a lease. |




#### Responses


**204**: OK



### POST /sys/leases/renew/{url_lease_id}

**Operation ID:** `leases-renew-lease-with-id`


Renews a lease, requesting to extend the lease.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `url_lease_id` | string | path | yes | The lease identifier to renew. This is included with a lease. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `increment` | integer | no | The desired increment in seconds to the lease |
| `lease_id` | string | no | The lease identifier to renew. This is included with a lease. |




#### Responses


**204**: OK



### POST /sys/leases/revoke

**Operation ID:** `leases-revoke-lease`


Revokes a lease immediately.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `lease_id` | string | no | The lease identifier to renew. This is included with a lease. |
| `sync` | boolean (default: True) | no | Whether or not to perform the revocation synchronously |
| `url_lease_id` | string | no | The lease identifier to renew. This is included with a lease. |




#### Responses


**204**: OK



### POST /sys/leases/revoke-force/{prefix}

**Operation ID:** `leases-force-revoke-lease-with-prefix`


Revokes all secrets or tokens generated under a given prefix immediately


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `prefix` | string | path | yes | The path to revoke keys under. Example: "prod/aws/ops" |




#### Responses


**204**: OK



### POST /sys/leases/revoke-prefix/{prefix}

**Operation ID:** `leases-revoke-lease-with-prefix`


Revokes all secrets (via a lease ID prefix) or tokens (via the tokens' path property) generated under a given prefix immediately.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `prefix` | string | path | yes | The path to revoke keys under. Example: "prod/aws/ops" |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `sync` | boolean (default: True) | no | Whether or not to perform the revocation synchronously |




#### Responses


**204**: OK



### POST /sys/leases/revoke/{url_lease_id}

**Operation ID:** `leases-revoke-lease-with-id`


Revokes a lease immediately.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `url_lease_id` | string | path | yes | The lease identifier to renew. This is included with a lease. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `lease_id` | string | no | The lease identifier to renew. This is included with a lease. |
| `sync` | boolean (default: True) | no | Whether or not to perform the revocation synchronously |




#### Responses


**204**: OK



### POST /sys/leases/tidy

**Operation ID:** `leases-tidy`


This endpoint performs cleanup tasks that can be run if certain error conditions have occurred.


#### Responses


**204**: OK



### GET /sys/locked-users

**Operation ID:** `locked-users-list`


Report the locked user count metrics, for this namespace and all child namespaces.


#### Responses


**200**: OK



### POST /sys/locked-users/{mount_accessor}/unlock/{alias_identifier}

**Operation ID:** `locked-users-unlock`


Unlocks the user with given mount_accessor and alias_identifier


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `alias_identifier` | string | path | yes | It is the name of the alias (user). For example, if the alias belongs to userpass backend, the name should be a valid username within userpass auth method. If the alias belongs to an approle auth method, the name should be a valid RoleID |
| `mount_accessor` | string | path | yes | MountAccessor is the identifier of the mount entry to which the user belongs |




#### Responses


**200**: OK



### GET /sys/loggers

**Operation ID:** `loggers-read-verbosity-level`


Read the log level for all existing loggers.


#### Responses


**200**: OK



### POST /sys/loggers

**Operation ID:** `loggers-update-verbosity-level`


Modify the log level for all existing loggers.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `level` | string | no | Log verbosity level. Supported values (in order of detail) are "trace", "debug", "info", "warn", and "error". |




#### Responses


**204**: OK



### DELETE /sys/loggers

**Operation ID:** `loggers-revert-verbosity-level`


Revert the all loggers to use log level provided in config.


#### Responses


**204**: OK



### GET /sys/loggers/{name}

**Operation ID:** `loggers-read-verbosity-level-for`


Read the log level for a single logger.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the logger to be modified. |




#### Responses


**200**: OK



### POST /sys/loggers/{name}

**Operation ID:** `loggers-update-verbosity-level-for`


Modify the log level of a single logger.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the logger to be modified. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `level` | string | no | Log verbosity level. Supported values (in order of detail) are "trace", "debug", "info", "warn", and "error". |




#### Responses


**204**: OK



### DELETE /sys/loggers/{name}

**Operation ID:** `loggers-revert-verbosity-level-for`


Revert a single logger to use log level provided in config.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the logger to be modified. |




#### Responses


**204**: OK



### GET /sys/managed-keys/{type}

**Operation ID:** `enterprise-stub-list-managed-keys-type`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `type` | string | path | yes |  |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /sys/managed-keys/{type}/{name}

**Operation ID:** `enterprise-stub-read-managed-keys-type-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |
| `type` | string | path | yes |  |




#### Responses


**200**: OK



### POST /sys/managed-keys/{type}/{name}

**Operation ID:** `enterprise-stub-write-managed-keys-type-name`



**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |
| `type` | string | path | yes |  |




#### Responses


**200**: OK



### DELETE /sys/managed-keys/{type}/{name}

**Operation ID:** `enterprise-stub-delete-managed-keys-type-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |
| `type` | string | path | yes |  |




#### Responses


**204**: empty body



### POST /sys/managed-keys/{type}/{name}/test/sign

**Operation ID:** `enterprise-stub-write-managed-keys-type-name-test-sign`



**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |
| `type` | string | path | yes |  |




#### Responses


**200**: OK



### GET /sys/metrics

**Operation ID:** `metrics`


Export the metrics aggregated for telemetry purpose.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `format` | string | query | no | Format to export metrics into. Currently accepts only "prometheus". |




#### Responses


**200**: OK



### POST /sys/mfa/validate

**Operation ID:** `mfa-validate`


Validates the login for the given MFA methods. Upon successful validation, it returns an auth response containing the client token


**Available without authentication:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `mfa_payload` | object | yes | A map from MFA method ID to a slice of passcodes or an empty slice if the method does not use passcodes |
| `mfa_request_id` | string | yes | ID for this MFA request |




#### Responses


**200**: OK



### GET /sys/monitor

**Operation ID:** `monitor`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `log_format` | string | query | no | Output format of logs. Supported values are "standard" and "json". The default is "standard". |
| `log_level` | string | query | no | Log level to view system logs at. Currently supported values are "trace", "debug", "info", "warn", "error". |




#### Responses


**200**: OK



### GET /sys/mounts

**Operation ID:** `mounts-list-secrets-engines`


List the currently mounted backends.


#### Responses


**200**: OK



### GET /sys/mounts/{path}

**Operation ID:** `mounts-read-configuration`


Read the configuration of the secret engine at the given path.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The path to mount to. Example: "aws/east" |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `accessor` | string | no |  |
| `config` | object | no | Configuration for this mount, such as default_lease_ttl and max_lease_ttl. |
| `deprecation_status` | string | no |  |
| `description` | string | no | User-friendly description for this mount. |
| `external_entropy_access` | boolean | no |  |
| `local` | boolean (default: False) | no | Mark the mount as a local mount, which is not replicated and is unaffected by replication. |
| `options` | object | no | The options to pass into the backend. Should be a json object with string keys and values. |
| `plugin_version` | string | no | The semantic version of the plugin to use. |
| `running_plugin_version` | string | no |  |
| `running_sha256` | string | no |  |
| `seal_wrap` | boolean (default: False) | no | Whether to turn on seal wrapping for the mount. |
| `type` | string | no | The type of the backend. Example: "passthrough" |
| `uuid` | string | no |  |





### POST /sys/mounts/{path}

**Operation ID:** `mounts-enable-secrets-engine`


Enable a new secrets engine at the given path.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The path to mount to. Example: "aws/east" |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `config` | object | no | Configuration for this mount, such as default_lease_ttl and max_lease_ttl. |
| `description` | string | no | User-friendly description for this mount. |
| `external_entropy_access` | boolean (default: False) | no | Whether to give the mount access to Stronghold's external entropy. |
| `local` | boolean (default: False) | no | Mark the mount as a local mount, which is not replicated and is unaffected by replication. |
| `options` | object | no | The options to pass into the backend. Should be a json object with string keys and values. |
| `plugin_name` | string | no | Name of the plugin to mount based from the name registered in the plugin catalog. |
| `plugin_version` | string | no | The semantic version of the plugin to use. |
| `seal_wrap` | boolean (default: False) | no | Whether to turn on seal wrapping for the mount. |
| `type` | string | no | The type of the backend. Example: "passthrough" |




#### Responses


**204**: OK



### DELETE /sys/mounts/{path}

**Operation ID:** `mounts-disable-secrets-engine`


Disable the mount point specified at the given path.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The path to mount to. Example: "aws/east" |




#### Responses


**200**: OK



### GET /sys/mounts/{path}/tune

**Operation ID:** `mounts-read-tuning-information`


Tune backend configuration parameters for this mount.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The path to mount to. Example: "aws/east" |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allowed_managed_keys` | array | no |  |
| `allowed_response_headers` | array | no | A list of headers to whitelist and allow a plugin to set on responses. |
| `audit_non_hmac_request_keys` | array | no |  |
| `audit_non_hmac_response_keys` | array | no |  |
| `default_lease_ttl` | integer | no | The default lease TTL for this mount. |
| `description` | string | no | User-friendly description for this credential backend. |
| `external_entropy_access` | boolean | no |  |
| `force_no_cache` | boolean | no |  |
| `listing_visibility` | string | no |  |
| `max_lease_ttl` | integer | no | The max lease TTL for this mount. |
| `options` | object | no | The options to pass into the backend. Should be a json object with string keys and values. |
| `passthrough_request_headers` | array | no |  |
| `plugin_version` | string | no | The semantic version of the plugin to use. |
| `token_type` | string | no | The type of token to issue (service or batch). |
| `user_lockout_counter_reset_duration` | integer | no |  |
| `user_lockout_disable` | boolean | no |  |
| `user_lockout_duration` | integer | no |  |
| `user_lockout_threshold` | integer | no |  |





### POST /sys/mounts/{path}/tune

**Operation ID:** `mounts-tune-configuration-parameters`


Tune backend configuration parameters for this mount.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The path to mount to. Example: "aws/east" |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `allowed_managed_keys` | array | no |  |
| `allowed_response_headers` | array | no | A list of headers to whitelist and allow a plugin to set on responses. |
| `audit_non_hmac_request_keys` | array | no | The list of keys in the request data object that will not be HMAC'ed by audit devices. |
| `audit_non_hmac_response_keys` | array | no | The list of keys in the response data object that will not be HMAC'ed by audit devices. |
| `cmd_enable_repl` | boolean | no | Enable the replication for this mount |
| `default_lease_ttl` | string | no | The default lease TTL for this mount. |
| `description` | string | no | User-friendly description for this credential backend. |
| `listing_visibility` | string | no | Determines the visibility of the mount in the UI-specific listing endpoint. Accepted value are 'unauth' and 'hidden', with the empty default ('') behaving like 'hidden'. |
| `max_lease_ttl` | string | no | The max lease TTL for this mount. |
| `options` | object | no | The options to pass into the backend. Should be a json object with string keys and values. |
| `passthrough_request_headers` | array | no | A list of headers to whitelist and pass from the request to the plugin. |
| `plugin_version` | string | no | The semantic version of the plugin to use. |
| `src_ca_cert` | string | no |  |
| `src_secret_path` | array | no |  |
| `src_token` | string | no |  |
| `sync_period_min` | integer | no |  |
| `token_type` | string | no | The type of token to issue (service or batch). |
| `user_lockout_config` | object | no | The user lockout configuration to pass into the backend. Should be a json object with string keys and values. |




#### Responses


**200**: OK



### GET /sys/namespaces/

**Operation ID:** `namespaces-list-namespaces`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /sys/namespaces/{path}

**Operation ID:** `namespaces-read-namespace`


Read namespace info


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The path to mount to. Example: "aws/east" |




#### Responses


**200**: OK



### POST /sys/namespaces/{path}

**Operation ID:** `namespaces-create-namespace`


Create a new namespace at the given path.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The path to mount to. Example: "aws/east" |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `custom_metadata` | object | no | User-provided key-value pairs that are used to describe information about a secret. |




#### Responses


**204**: OK



### DELETE /sys/namespaces/{path}

**Operation ID:** `namespaces-delete-namespace`


Delete namespace specified at the given path.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `path` | string | path | yes | The path to mount to. Example: "aws/east" |




#### Responses


**200**: OK



### GET /sys/plugins/catalog

**Operation ID:** `plugins-catalog-list-plugins`


Lists all the plugins known to Stronghold


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `detailed` | object | no |  |





### GET /sys/plugins/catalog/{name}

**Operation ID:** `plugins-catalog-read-plugin-configuration`


Return the configuration data for the plugin with the given name.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the plugin |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `args` | array | no | The args passed to plugin command. |
| `builtin` | boolean | no |  |
| `command` | string | no | The command used to start the plugin. The executable defined in this command must exist in stronghold's plugin directory. |
| `deprecation_status` | string | no |  |
| `name` | string | no | The name of the plugin |
| `sha256` | string | no | The SHA256 sum of the executable used in the command field. This should be HEX encoded. |
| `version` | string | no | The semantic version of the plugin to use. |





### POST /sys/plugins/catalog/{name}

**Operation ID:** `plugins-catalog-register-plugin`


Register a new plugin, or updates an existing one with the supplied name.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the plugin |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `args` | array | no | The args passed to plugin command. |
| `command` | string | no | The command used to start the plugin. The executable defined in this command must exist in stronghold's plugin directory. |
| `env` | array | no | The environment variables passed to plugin command. Each entry is of the form "key=value". |
| `sha256` | string | no | The SHA256 sum of the executable used in the command field. This should be HEX encoded. |
| `type` | string | no | The type of the plugin, may be auth, secret, or database |
| `version` | string | no | The semantic version of the plugin to use. |




#### Responses


**200**: OK



### DELETE /sys/plugins/catalog/{name}

**Operation ID:** `plugins-catalog-remove-plugin`


Remove the plugin with the given name.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the plugin |




#### Responses


**200**: OK



### GET /sys/plugins/catalog/{type}

**Operation ID:** `plugins-catalog-list-plugins-with-type`


List the plugins in the catalog.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `type` | string | path | yes | The type of the plugin, may be auth, secret, or database |
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no | List of plugin names in the catalog |





### GET /sys/plugins/catalog/{type}/{name}

**Operation ID:** `plugins-catalog-read-plugin-configuration-with-type`


Return the configuration data for the plugin with the given name.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the plugin |
| `type` | string | path | yes | The type of the plugin, may be auth, secret, or database |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `args` | array | no | The args passed to plugin command. |
| `builtin` | boolean | no |  |
| `command` | string | no | The command used to start the plugin. The executable defined in this command must exist in stronghold's plugin directory. |
| `deprecation_status` | string | no |  |
| `name` | string | no | The name of the plugin |
| `sha256` | string | no | The SHA256 sum of the executable used in the command field. This should be HEX encoded. |
| `version` | string | no | The semantic version of the plugin to use. |





### POST /sys/plugins/catalog/{type}/{name}

**Operation ID:** `plugins-catalog-register-plugin-with-type`


Register a new plugin, or updates an existing one with the supplied name.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the plugin |
| `type` | string | path | yes | The type of the plugin, may be auth, secret, or database |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `args` | array | no | The args passed to plugin command. |
| `command` | string | no | The command used to start the plugin. The executable defined in this command must exist in stronghold's plugin directory. |
| `env` | array | no | The environment variables passed to plugin command. Each entry is of the form "key=value". |
| `sha256` | string | no | The SHA256 sum of the executable used in the command field. This should be HEX encoded. |
| `version` | string | no | The semantic version of the plugin to use. |




#### Responses


**200**: OK



### DELETE /sys/plugins/catalog/{type}/{name}

**Operation ID:** `plugins-catalog-remove-plugin-with-type`


Remove the plugin with the given name.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the plugin |
| `type` | string | path | yes | The type of the plugin, may be auth, secret, or database |




#### Responses


**200**: OK



### POST /sys/plugins/reload/backend

**Operation ID:** `plugins-reload-backends`


Reload mounted plugin backends.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `mounts` | array | no | The mount paths of the plugin backends to reload. |
| `plugin` | string | no | The name of the plugin to reload, as registered in the plugin catalog. |
| `scope` | string | no |  |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `reload_id` | string | no |  |





**202**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `reload_id` | string | no |  |





### GET /sys/plugins/reload/backend/status

**Operation ID:** `enterprise-stub-read-plugins-reload-backend-status`



#### Responses


**200**: OK



### GET /sys/policies/acl

**Operation ID:** `policies-list-acl-policies`


List the configured access control policies.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no |  |
| `policies` | array | no |  |





### GET /sys/policies/acl/{name}

**Operation ID:** `policies-read-acl-policy`


Retrieve information about the named ACL policy.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the policy. Example: "ops" |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `name` | string | no |  |
| `policy` | string | no |  |
| `rules` | string | no |  |





### POST /sys/policies/acl/{name}

**Operation ID:** `policies-write-acl-policy`


Add a new or update an existing ACL policy.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the policy. Example: "ops" |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `policy` | string | no | The rules of the policy. |




#### Responses


**204**: OK



### DELETE /sys/policies/acl/{name}

**Operation ID:** `policies-delete-acl-policy`


Delete the ACL policy with the given name.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the policy. Example: "ops" |




#### Responses


**204**: OK



### GET /sys/policies/egp

**Operation ID:** `enterprise-stub-list-policies-egp`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /sys/policies/egp/{name}

**Operation ID:** `enterprise-stub-read-policies-egp-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |




#### Responses


**200**: OK



### POST /sys/policies/egp/{name}

**Operation ID:** `enterprise-stub-write-policies-egp-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |




#### Responses


**200**: OK



### DELETE /sys/policies/egp/{name}

**Operation ID:** `enterprise-stub-delete-policies-egp-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |




#### Responses


**204**: empty body



### GET /sys/policies/password

**Operation ID:** `policies-list-password-policies`


List the existing password policies.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no |  |





### GET /sys/policies/password/{name}

**Operation ID:** `policies-read-password-policy`


Retrieve an existing password policy.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the password policy. |




#### Responses


**204**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `policy` | string | no |  |





### POST /sys/policies/password/{name}

**Operation ID:** `policies-write-password-policy`


Add a new or update an existing password policy.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the password policy. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `policy` | string | no | The password policy |




#### Responses


**204**: OK



### DELETE /sys/policies/password/{name}

**Operation ID:** `policies-delete-password-policy`


Delete a password policy.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the password policy. |




#### Responses


**204**: OK



### GET /sys/policies/password/{name}/generate

**Operation ID:** `policies-generate-password-from-password-policy`


Generate a password from an existing password policy.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the password policy. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `password` | string | no |  |





### GET /sys/policies/rgp

**Operation ID:** `enterprise-stub-list-policies-rgp`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /sys/policies/rgp/{name}

**Operation ID:** `enterprise-stub-read-policies-rgp-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |




#### Responses


**200**: OK



### POST /sys/policies/rgp/{name}

**Operation ID:** `enterprise-stub-write-policies-rgp-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |




#### Responses


**200**: OK



### DELETE /sys/policies/rgp/{name}

**Operation ID:** `enterprise-stub-delete-policies-rgp-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |




#### Responses


**204**: empty body



### GET /sys/policy

**Operation ID:** `policies-list`


List the configured access control policies.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string | query | no | Return a list if `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no |  |
| `policies` | array | no |  |





### GET /sys/policy/{name}

**Operation ID:** `policies-read-acl-policy2`


Retrieve the policy body for the named policy.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the policy. Example: "ops" |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `name` | string | no |  |
| `policy` | string | no |  |
| `rules` | string | no |  |





### POST /sys/policy/{name}

**Operation ID:** `policies-write-acl-policy2`


Add a new or update an existing policy.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the policy. Example: "ops" |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `policy` | string | no | The rules of the policy. |
| `rules` | string | no | ⚠️ Deprecated. The rules of the policy. |




#### Responses


**204**: OK



### DELETE /sys/policy/{name}

**Operation ID:** `policies-delete-acl-policy2`


Delete the policy with the given name.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | The name of the policy. Example: "ops" |




#### Responses


**204**: OK



### GET /sys/pprof

**Operation ID:** `pprof-index`


Returns an HTML page listing the available profiles.


#### Responses


**200**: OK



### GET /sys/pprof/allocs

**Operation ID:** `pprof-memory-allocations`


Returns a sampling of all past memory allocations.


#### Responses


**200**: OK



### GET /sys/pprof/block

**Operation ID:** `pprof-blocking`


Returns stack traces that led to blocking on synchronization primitives


#### Responses


**200**: OK



### GET /sys/pprof/cmdline

**Operation ID:** `pprof-command-line`


Returns the running program's command line.


#### Responses


**200**: OK



### GET /sys/pprof/goroutine

**Operation ID:** `pprof-goroutines`


Returns stack traces of all current goroutines.


#### Responses


**200**: OK



### GET /sys/pprof/heap

**Operation ID:** `pprof-memory-allocations-live`


Returns a sampling of memory allocations of live object.


#### Responses


**200**: OK



### GET /sys/pprof/mutex

**Operation ID:** `pprof-mutexes`


Returns stack traces of holders of contended mutexes


#### Responses


**200**: OK



### GET /sys/pprof/profile

**Operation ID:** `pprof-cpu-profile`


Returns a pprof-formatted cpu profile payload.


#### Responses


**200**: OK



### GET /sys/pprof/symbol

**Operation ID:** `pprof-symbols`


Returns the program counters listed in the request.


#### Responses


**200**: OK



### GET /sys/pprof/threadcreate

**Operation ID:** `pprof-thread-creations`


Returns stack traces that led to the creation of new OS threads


#### Responses


**200**: OK



### GET /sys/pprof/trace

**Operation ID:** `pprof-execution-trace`


Returns the execution trace in binary form.


#### Responses


**200**: OK



### GET /sys/quotas/config

**Operation ID:** `rate-limit-quotas-read-configuration`


Create, update and read the quota configuration.


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `enable_rate_limit_audit_logging` | boolean | no |  |
| `enable_rate_limit_response_headers` | boolean | no |  |
| `rate_limit_exempt_paths` | array | no |  |





### POST /sys/quotas/config

**Operation ID:** `rate-limit-quotas-configure`


Create, update and read the quota configuration.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `enable_rate_limit_audit_logging` | boolean | no | If set, starts audit logging of requests that get rejected due to rate limit quota rule violations. |
| `enable_rate_limit_response_headers` | boolean | no | If set, additional rate limit quota HTTP headers will be added to responses. |
| `rate_limit_exempt_paths` | array | no | Specifies the list of exempt paths from all rate limit quotas. If empty no paths will be exempt. |




#### Responses


**204**: OK



### GET /sys/quotas/lease-count

**Operation ID:** `enterprise-stub-list-quotas-lease-count`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /sys/quotas/lease-count/{name}

**Operation ID:** `enterprise-stub-read-quotas-lease-count-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |




#### Responses


**200**: OK



### POST /sys/quotas/lease-count/{name}

**Operation ID:** `enterprise-stub-write-quotas-lease-count-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |




#### Responses


**200**: OK



### DELETE /sys/quotas/lease-count/{name}

**Operation ID:** `enterprise-stub-delete-quotas-lease-count-name`



#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes |  |




#### Responses


**204**: empty body



### GET /sys/quotas/rate-limit

**Operation ID:** `rate-limit-quotas-list`


Lists the names of all the rate limit quotas.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | array | no |  |





### GET /sys/quotas/rate-limit/{name}

**Operation ID:** `rate-limit-quotas-read`


Get, create or update rate limit resource quota for an optional namespace or mount.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the quota rule. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `block_interval` | integer | no |  |
| `interval` | integer | no |  |
| `name` | string | no |  |
| `path` | string | no |  |
| `rate` | number | no |  |
| `role` | string | no |  |
| `type` | string | no |  |





### POST /sys/quotas/rate-limit/{name}

**Operation ID:** `rate-limit-quotas-write`


Get, create or update rate limit resource quota for an optional namespace or mount.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the quota rule. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `block_interval` | integer | no | If set, when a client reaches a rate limit threshold, the client will be prohibited from any further requests until after the 'block_interval' has elapsed. |
| `interval` | integer | no | The duration to enforce rate limiting for (default '1s'). |
| `path` | string | no | Path of the mount or namespace to apply the quota. A blank path configures a global quota. For example namespace1/ adds a quota to a full namespace, namespace1/auth/userpass adds a quota to userpass in namespace1. |
| `rate` | number | no | The maximum number of requests in a given interval to be allowed by the quota rule. The 'rate' must be positive. |
| `role` | string | no | Login role to apply this quota to. Note that when set, path must be configured to a valid auth method with a concept of roles. |
| `type` | string | no | Type of the quota rule. |




#### Responses


**204**: No Content



### DELETE /sys/quotas/rate-limit/{name}

**Operation ID:** `rate-limit-quotas-delete`


Get, create or update rate limit resource quota for an optional namespace or mount.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the quota rule. |




#### Responses


**204**: OK



### GET /sys/rekey/backup

**Operation ID:** `rekey-read-backup-key`


Return the backup copy of PGP-encrypted unseal keys.


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | object | no |  |
| `keys_base64` | object | no |  |
| `nonce` | string | no |  |





### DELETE /sys/rekey/backup

**Operation ID:** `rekey-delete-backup-key`


Delete the backup copy of PGP-encrypted unseal keys.


#### Responses


**204**: OK



### GET /sys/rekey/init

**Operation ID:** `rekey-attempt-read-progress`


Reads the configuration and progress of the current rekey attempt.


**Available without authentication:** yes


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `backup` | boolean | no |  |
| `n` | integer | no |  |
| `nounce` | string | no |  |
| `pgp_fingerprints` | array | no |  |
| `progress` | integer | no |  |
| `required` | integer | no |  |
| `started` | string | no |  |
| `t` | integer | no |  |
| `verification_nonce` | string | no |  |
| `verification_required` | boolean | no |  |





### POST /sys/rekey/init

**Operation ID:** `rekey-attempt-initialize`


Initializes a new rekey attempt.


**Available without authentication:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `backup` | boolean | no | Specifies if using PGP-encrypted keys, whether Stronghold should also store a plaintext backup of the PGP-encrypted keys. |
| `pgp_keys` | array | no | Specifies an array of PGP public keys used to encrypt the output unseal keys. Ordering is preserved. The keys must be base64-encoded from their original binary representation. The size of this array must be the same as secret_shares. |
| `require_verification` | boolean | no | Turns on verification functionality |
| `secret_shares` | integer | no | Specifies the number of shares to split the unseal key into. |
| `secret_threshold` | integer | no | Specifies the number of shares required to reconstruct the unseal key. This must be less than or equal secret_shares. If using Stronghold HSM with auto-unsealing, this value must be the same as secret_shares. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `backup` | boolean | no |  |
| `n` | integer | no |  |
| `nounce` | string | no |  |
| `pgp_fingerprints` | array | no |  |
| `progress` | integer | no |  |
| `required` | integer | no |  |
| `started` | string | no |  |
| `t` | integer | no |  |
| `verification_nonce` | string | no |  |
| `verification_required` | boolean | no |  |





### DELETE /sys/rekey/init

**Operation ID:** `rekey-attempt-cancel`


Cancels any in-progress rekey.


**Available without authentication:** yes


#### Responses


**200**: OK



### GET /sys/rekey/recovery-key-backup

**Operation ID:** `rekey-read-backup-recovery-key`


Allows fetching or deleting the backup of the rotated unseal keys.


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `keys` | object | no |  |
| `keys_base64` | object | no |  |
| `nonce` | string | no |  |





### DELETE /sys/rekey/recovery-key-backup

**Operation ID:** `rekey-delete-backup-recovery-key`


Allows fetching or deleting the backup of the rotated unseal keys.


#### Responses


**204**: OK



### POST /sys/rekey/update

**Operation ID:** `rekey-attempt-update`


Enter a single unseal key share to progress the rekey of the Stronghold.


**Available without authentication:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key` | string | no | Specifies a single unseal key share. |
| `nonce` | string | no | Specifies the nonce of the rekey attempt. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `backup` | boolean | no |  |
| `complete` | boolean | no |  |
| `keys` | array | no |  |
| `keys_base64` | array | no |  |
| `n` | integer | no |  |
| `nounce` | string | no |  |
| `pgp_fingerprints` | array | no |  |
| `progress` | integer | no |  |
| `required` | integer | no |  |
| `started` | string | no |  |
| `t` | integer | no |  |
| `verification_nonce` | string | no |  |
| `verification_required` | boolean | no |  |





### GET /sys/rekey/verify

**Operation ID:** `rekey-verification-read-progress`


Read the configuration and progress of the current rekey verification attempt.


**Available without authentication:** yes


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `n` | integer | no |  |
| `nounce` | string | no |  |
| `progress` | integer | no |  |
| `started` | string | no |  |
| `t` | integer | no |  |





### POST /sys/rekey/verify

**Operation ID:** `rekey-verification-update`


Enter a single new key share to progress the rekey verification operation.


**Available without authentication:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key` | string | no | Specifies a single unseal share key from the new set of shares. |
| `nonce` | string | no | Specifies the nonce of the rekey verification operation. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `complete` | boolean | no |  |
| `nounce` | string | no |  |





### DELETE /sys/rekey/verify

**Operation ID:** `rekey-verification-cancel`


Cancel any in-progress rekey verification operation.


**Available without authentication:** yes


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `n` | integer | no |  |
| `nounce` | string | no |  |
| `progress` | integer | no |  |
| `started` | string | no |  |
| `t` | integer | no |  |





### POST /sys/remount

**Operation ID:** `remount`


Initiate a mount migration


**Required sudo:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `from` | string | no | The previous mount point. |
| `to` | string | no | The new mount point. |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `migration_id` | string | no |  |





### GET /sys/remount/status/{migration_id}

**Operation ID:** `remount-status`


Check status of a mount migration


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `migration_id` | string | path | yes | The ID of the migration operation |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `migration_id` | string | no |  |
| `migration_info` | object | no |  |





### POST /sys/renew

**Operation ID:** `leases-renew-lease2`


Renews a lease, requesting to extend the lease.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `increment` | integer | no | The desired increment in seconds to the lease |
| `lease_id` | string | no | The lease identifier to renew. This is included with a lease. |
| `url_lease_id` | string | no | The lease identifier to renew. This is included with a lease. |




#### Responses


**204**: OK



### POST /sys/renew/{url_lease_id}

**Operation ID:** `leases-renew-lease-with-id2`


Renews a lease, requesting to extend the lease.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `url_lease_id` | string | path | yes | The lease identifier to renew. This is included with a lease. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `increment` | integer | no | The desired increment in seconds to the lease |
| `lease_id` | string | no | The lease identifier to renew. This is included with a lease. |




#### Responses


**204**: OK



### GET /sys/replication/status

**Operation ID:** `system-read-replication-status`



**Available without authentication:** yes


#### Responses


**200**: OK



### POST /sys/revoke

**Operation ID:** `leases-revoke-lease2`


Revokes a lease immediately.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `lease_id` | string | no | The lease identifier to renew. This is included with a lease. |
| `sync` | boolean (default: True) | no | Whether or not to perform the revocation synchronously |
| `url_lease_id` | string | no | The lease identifier to renew. This is included with a lease. |




#### Responses


**204**: OK



### POST /sys/revoke-force/{prefix}

**Operation ID:** `leases-force-revoke-lease-with-prefix2`


Revokes all secrets or tokens generated under a given prefix immediately


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `prefix` | string | path | yes | The path to revoke keys under. Example: "prod/aws/ops" |




#### Responses


**204**: OK



### POST /sys/revoke-prefix/{prefix}

**Operation ID:** `leases-revoke-lease-with-prefix2`


Revokes all secrets (via a lease ID prefix) or tokens (via the tokens' path property) generated under a given prefix immediately.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `prefix` | string | path | yes | The path to revoke keys under. Example: "prod/aws/ops" |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `sync` | boolean (default: True) | no | Whether or not to perform the revocation synchronously |




#### Responses


**204**: OK



### POST /sys/revoke/{url_lease_id}

**Operation ID:** `leases-revoke-lease-with-id2`


Revokes a lease immediately.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `url_lease_id` | string | path | yes | The lease identifier to renew. This is included with a lease. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `lease_id` | string | no | The lease identifier to renew. This is included with a lease. |
| `sync` | boolean (default: True) | no | Whether or not to perform the revocation synchronously |




#### Responses


**204**: OK



### POST /sys/rotate

**Operation ID:** `encryption-key-rotate`


Rotates the backend encryption key used to persist data.


**Required sudo:** yes


#### Responses


**204**: OK



### GET /sys/rotate/config

**Operation ID:** `encryption-key-read-rotation-configuration`


Configures settings related to the backend encryption key management.


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `enabled` | boolean | no |  |
| `interval` | integer | no |  |
| `max_operations` | integer | no |  |





### POST /sys/rotate/config

**Operation ID:** `encryption-key-configure-rotation`


Configures settings related to the backend encryption key management.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `enabled` | boolean | no | Whether automatic rotation is enabled. |
| `interval` | integer | no | How long after installation of an active key term that the key will be automatically rotated. |
| `max_operations` | integer | no | The number of encryption operations performed before the barrier key is automatically rotated. |




#### Responses


**204**: OK



### POST /sys/seal

**Operation ID:** `seal`


Seal the Stronghold.


#### Responses


**204**: OK



### GET /sys/seal-status

**Operation ID:** `seal-status`


Check the seal status of a Stronghold.


**Available without authentication:** yes


#### Responses


**200**: 



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `build_date` | string | no |  |
| `cluster_id` | string | no |  |
| `cluster_name` | string | no |  |
| `hcp_link_resource_ID` | string | no |  |
| `hcp_link_status` | string | no |  |
| `initialized` | boolean | no |  |
| `migration` | boolean | no |  |
| `n` | integer | no |  |
| `nonce` | string | no |  |
| `progress` | integer | no |  |
| `recovery_seal` | boolean | no |  |
| `sealed` | boolean | no |  |
| `storage_type` | string | no |  |
| `t` | integer | no |  |
| `type` | string | no |  |
| `version` | string | no |  |





### GET /sys/sealwrap/rewrap

**Operation ID:** `system-read-sealwrap-rewrap`



#### Responses


**200**: OK



### POST /sys/sealwrap/rewrap

**Operation ID:** `system-write-sealwrap-rewrap`



#### Responses


**200**: OK



### POST /sys/step-down

**Operation ID:** `step-down-leader`


Cause the node to give up active status.


#### Responses


**204**: empty body



### GET /sys/storage/raft/autopilot/configuration

**Operation ID:** `system-read-storage-raft-autopilot-configuration`


Returns autopilot configuration.


#### Responses


**200**: OK



### POST /sys/storage/raft/autopilot/configuration

**Operation ID:** `system-write-storage-raft-autopilot-configuration`


Returns autopilot configuration.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `cleanup_dead_servers` | boolean | no | Controls whether to remove dead servers from the Raft peer list periodically or when a new server joins. |
| `dead_server_last_contact_threshold` | integer | no | Limit on the amount of time a server can go without leader contact before being considered failed. This takes effect only when cleanup_dead_servers is set. |
| `disable_upgrade_migration` | boolean | no | Whether or not to perform automated version upgrades. |
| `dr_operation_token` | string | no | DR operation token used to authorize this request (if a DR secondary node). |
| `last_contact_threshold` | integer | no | Limit on the amount of time a server can go without leader contact before being considered unhealthy. |
| `max_trailing_logs` | integer | no | Amount of entries in the Raft Log that a server can be behind before being considered unhealthy. |
| `min_quorum` | integer | no | Minimum number of servers allowed in a cluster before autopilot can prune dead servers. This should at least be 3. |
| `server_stabilization_time` | integer | no | Minimum amount of time a server must be in a stable, healthy state before it can be added to the cluster. |




#### Responses


**200**: OK



### GET /sys/storage/raft/autopilot/state

**Operation ID:** `system-read-storage-raft-autopilot-state`


Returns the state of the raft cluster under integrated storage as seen by autopilot.


#### Responses


**200**: OK



### POST /sys/storage/raft/bootstrap/answer

**Operation ID:** `system-write-storage-raft-bootstrap-answer`


Accepts an answer from the peer to be joined to the fact cluster.


**Available without authentication:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `answer` | string | no |  |
| `cluster_addr` | string | no |  |
| `non_voter` | boolean | no |  |
| `server_id` | string | no |  |




#### Responses


**200**: OK



### POST /sys/storage/raft/bootstrap/challenge

**Operation ID:** `system-write-storage-raft-bootstrap-challenge`


Creates a challenge for the new peer to be joined to the raft cluster.


**Available without authentication:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `server_id` | string | no |  |




#### Responses


**200**: OK



### GET /sys/storage/raft/configuration

**Operation ID:** `system-read-storage-raft-configuration`


Returns the configuration of the raft cluster.


#### Responses


**200**: OK



### POST /sys/storage/raft/configuration

**Operation ID:** `system-write-storage-raft-configuration`


Returns the configuration of the raft cluster in a DR secondary cluster.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `dr_operation_token` | string | no | DR operation token used to authorize this request (if a DR secondary node). |




#### Responses


**200**: OK



### POST /sys/storage/raft/remove-peer

**Operation ID:** `system-write-storage-raft-remove-peer`


Remove a peer from the raft cluster.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `dr_operation_token` | string | no | DR operation token used to authorize this request (if a DR secondary node). |
| `server_id` | string | no |  |




#### Responses


**200**: OK



### GET /sys/storage/raft/snapshot

**Operation ID:** `system-read-storage-raft-snapshot`


Returns a snapshot of the current state of vault.


#### Responses


**200**: OK



### POST /sys/storage/raft/snapshot

**Operation ID:** `system-write-storage-raft-snapshot`


Installs the provided snapshot, returning the cluster to the state defined in it.


#### Responses


**200**: OK



### GET /sys/storage/raft/snapshot-auto/config

**Operation ID:** `system-list-storage-raft-snapshot-auto-config`


Lists all automatic snapshot configuration names.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



### GET /sys/storage/raft/snapshot-auto/config/{name}

**Operation ID:** `system-read-storage-raft-snapshot-auto-config-name`


Gets the configuration of the automatic snapshot.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the configuration to modify. |




#### Responses


**200**: OK



### POST /sys/storage/raft/snapshot-auto/config/{name}

**Operation ID:** `system-write-storage-raft-snapshot-auto-config-name`


Updates the configuration of the automatic snapshot.


**Required sudo:** yes


**Creation supported:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the configuration to modify. |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `aws_access_key_id` | string | no | S3 access key ID. |
| `aws_s3_bucket` | string | yes | S3 bucket to write snapshots to. |
| `aws_s3_ca_certificate` | string (default: ) | no | S3 CA certificate PEM. |
| `aws_s3_disable_tls` | boolean (default: False) | no | Disable TLS for the S3 endpoint. This should only be used for testing purposes, typically in conjunction with `s3_endpoint`. |
| `aws_s3_endpoint` | string | no | S3 endpoint. |
| `aws_s3_region` | string (default: ) | no | S3 region bucket is in. |
| `aws_secret_access_key` | string | no | S3 secret access key. |
| `file_prefix` | string (default: stronghold-snapshot) | no | Within the directory or bucket prefix given by `path_prefix`, the file or object name of snapshot files will start with this string. |
| `interval` | integer | yes | Time between snapshots. This can be either an integer number of seconds, or a Go duration format string (e.g. 24h). |
| `local_max_space` | integer (default: 0) | yes | For `storage_type=local`, the maximum space, in bytes, to use for all snapshots with the given `file_prefix` in the `path_prefix` directory. Snapshot attempts will fail if there is not enough space left in this allowance. Value `0` disables limit. |
| `path_prefix` | string | yes | For `storage_type=local`, the directory to write the snapshots in. For cloud storage types, the bucket prefix to use, also leading `/` is ignored. The trailing `/` is optional. |
| `retain` | integer (default: 3) | no | How many snapshots are to be kept; when writing a snapshot, if there are more snapshots already stored than this number, the oldest ones will be deleted. |
| `storage_type` | string (local, aws-s3) | yes | One of "local" or "s3". The remaining parameters described below are all specific to the selected `storage_type` and prefixed accordingly. |




#### Responses


**200**: OK



### DELETE /sys/storage/raft/snapshot-auto/config/{name}

**Operation ID:** `system-delete-storage-raft-snapshot-auto-config-name`


Deletes the configuration of the automatic snapshot.


**Required sudo:** yes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the configuration to modify. |




#### Responses


**204**: empty body



### GET /sys/storage/raft/snapshot-auto/status/{name}

**Operation ID:** `system-read-storage-raft-snapshot-auto-status-name`


Shows the status of the automatic snapshot.


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `name` | string | path | yes | Name of the status to get. |




#### Responses


**200**: OK



### POST /sys/storage/raft/snapshot-force

**Operation ID:** `system-write-storage-raft-snapshot-force`


Installs the provided snapshot, returning the cluster to the state defined in it. This bypasses checks ensuring the current Autounseal or Shamir keys are consistent with the snapshot data.


#### Responses


**200**: OK



### POST /sys/tools/hash

**Operation ID:** `generate-hash`


Generate a hash sum for input data


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm` | string (default: sha2-256) | no | Algorithm to use (POST body parameter). Valid values are: * sha2-224 * sha2-256 * sha2-384 * sha2-512 Defaults to "sha2-256". |
| `format` | string (default: hex) | no | Encoding format to use. Can be "hex" or "base64". Defaults to "hex". |
| `input` | string | no | The base64-encoded input data |
| `urlalgorithm` | string | no | Algorithm to use (POST URL parameter) |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `sum` | string | no |  |





### POST /sys/tools/hash/{urlalgorithm}

**Operation ID:** `generate-hash-with-algorithm`


Generate a hash sum for input data


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `urlalgorithm` | string | path | yes | Algorithm to use (POST URL parameter) |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `algorithm` | string (default: sha2-256) | no | Algorithm to use (POST body parameter). Valid values are: * sha2-224 * sha2-256 * sha2-384 * sha2-512 Defaults to "sha2-256". |
| `format` | string (default: hex) | no | Encoding format to use. Can be "hex" or "base64". Defaults to "hex". |
| `input` | string | no | The base64-encoded input data |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `sum` | string | no |  |





### POST /sys/tools/random

**Operation ID:** `generate-random`


Generate random bytes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bytes` | integer (default: 32) | no | The number of bytes to generate (POST body parameter). Defaults to 32 (256 bits). |
| `format` | string (default: base64) | no | Encoding format to use. Can be "hex" or "base64". Defaults to "base64". |
| `source` | string (default: platform) | no | Which system to source random data from, ether "platform", "seal", or "all". |
| `urlbytes` | string | no | The number of bytes to generate (POST URL parameter) |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `random_bytes` | string | no |  |





### POST /sys/tools/random/{source}

**Operation ID:** `generate-random-with-source`


Generate random bytes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `source` | string | path | yes | Which system to source random data from, ether "platform", "seal", or "all". |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bytes` | integer (default: 32) | no | The number of bytes to generate (POST body parameter). Defaults to 32 (256 bits). |
| `format` | string (default: base64) | no | Encoding format to use. Can be "hex" or "base64". Defaults to "base64". |
| `urlbytes` | string | no | The number of bytes to generate (POST URL parameter) |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `random_bytes` | string | no |  |





### POST /sys/tools/random/{source}/{urlbytes}

**Operation ID:** `generate-random-with-source-and-bytes`


Generate random bytes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `source` | string | path | yes | Which system to source random data from, ether "platform", "seal", or "all". |
| `urlbytes` | string | path | yes | The number of bytes to generate (POST URL parameter) |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bytes` | integer (default: 32) | no | The number of bytes to generate (POST body parameter). Defaults to 32 (256 bits). |
| `format` | string (default: base64) | no | Encoding format to use. Can be "hex" or "base64". Defaults to "base64". |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `random_bytes` | string | no |  |





### POST /sys/tools/random/{urlbytes}

**Operation ID:** `generate-random-with-bytes`


Generate random bytes


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `urlbytes` | string | path | yes | The number of bytes to generate (POST URL parameter) |




#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `bytes` | integer (default: 32) | no | The number of bytes to generate (POST body parameter). Defaults to 32 (256 bits). |
| `format` | string (default: base64) | no | Encoding format to use. Can be "hex" or "base64". Defaults to "base64". |
| `source` | string (default: platform) | no | Which system to source random data from, ether "platform", "seal", or "all". |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `random_bytes` | string | no |  |





### POST /sys/unseal

**Operation ID:** `unseal`


Unseal the Stronghold.


**Available without authentication:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key` | string | no | Specifies a single unseal key share. This is required unless reset is true. |
| `reset` | boolean | no | Specifies if previously-provided unseal keys are discarded and the unseal process is reset. |




#### Responses


**200**: 



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `build_date` | string | no |  |
| `cluster_id` | string | no |  |
| `cluster_name` | string | no |  |
| `hcp_link_resource_ID` | string | no |  |
| `hcp_link_status` | string | no |  |
| `initialized` | boolean | no |  |
| `migration` | boolean | no |  |
| `n` | integer | no |  |
| `nonce` | string | no |  |
| `progress` | integer | no |  |
| `recovery_seal` | boolean | no |  |
| `sealed` | boolean | no |  |
| `storage_type` | string | no |  |
| `t` | integer | no |  |
| `type` | string | no |  |
| `version` | string | no |  |





### GET /sys/version-history

**Operation ID:** `version-history`


Returns map of historical version change entries


#### Parameters


| Parameter | Type | Location | Required | Description |
|----------|-----|--------------|--------------|----------|
| `list` | string (true) | query | yes | Must be set to `true` |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `key_info` | object | no |  |
| `keys` | array | no |  |





### GET /sys/wrapping/lookup

**Operation ID:** `read-wrapping-properties2`


Look up wrapping properties for the requester's token.


**Available without authentication:** yes


#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `creation_path` | string | no |  |
| `creation_time` | string | no |  |
| `creation_ttl` | integer | no |  |





### POST /sys/wrapping/lookup

**Operation ID:** `read-wrapping-properties`


Look up wrapping properties for the given token.


**Available without authentication:** yes


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token` | string | no |  |




#### Responses


**200**: OK



| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `creation_path` | string | no |  |
| `creation_time` | string | no |  |
| `creation_ttl` | integer | no |  |





### POST /sys/wrapping/rewrap

**Operation ID:** `rewrap`


Rotates a response-wrapped token.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token` | string | no |  |




#### Responses


**200**: OK



### POST /sys/wrapping/unwrap

**Operation ID:** `unwrap`


Unwraps a response-wrapped token.


#### Request body parameters


| Parameter | Type | Required | Description |
|----------|-----|--------------|----------|
| `token` | string | no |  |




#### Responses


**200**: OK



**204**: No content



### POST /sys/wrapping/wrap

**Operation ID:** `wrap`


Response-wraps an arbitrary JSON object.


#### Responses


**200**: OK


{% endraw %}
