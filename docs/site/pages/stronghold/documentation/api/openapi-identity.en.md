---
title: "API - Identity"
permalink: en/stronghold/documentation/api/openapi-identity.html
search: true
sitemap_include: false
description: API reference - Identity
lang: en
---

{% raw %}

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




{% endraw %}
