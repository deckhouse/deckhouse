---
title: Application settings
permalink: en/architecture/marketplace/settings.html
description: "Settings and values schemas of an Application package: validation, defaults, CEL rules, grantable resources, immutable fields, and web interface form extensions."
---

The user configures an application instance in the `spec.settings` field of the [Application](../../reference/api/cr.html#application) resource. The package describes the allowed settings with an OpenAPI schema. Deckhouse Platform (DP) validates the settings against this schema, applies default values, and passes the result to templates and hooks. The DP web interface builds the application settings form from the same schema.

## Schema files

The `openapi/` directory of the package contains two schemas:

- `settings.yaml` — the schema of `Application.spec.settings`, that is, of the settings the user can change. The legacy file name `config-values.yaml` is supported for backward compatibility: DP reads it only if `settings.yaml` is absent.
- `values.yaml` — the schema of the full set of values available to templates as `.Values`, including internal values set by hooks.
- `doc-ru-settings.yaml` — Russian descriptions of the settings. The `d8 package verify` command requires a `doc-ru-<NAME>.yaml` file for every schema file except `values.yaml`.

The settings are located at the root of the values, so the values schema must declare them too. To include all settings in the values schema without duplicating them, add the `x-extend` extension to `openapi/values.yaml`. Without it, the values fail the validation when a hook changes them.

Example `openapi/values.yaml`:

```yaml
x-extend:
  schema: settings.yaml
type: object
properties:
  internal:
    type: object
    default: {}
    properties:
      adminPassword:
        type: string
```

When DP scans a repository, it publishes both schemas in the `status.packageSchemas` field of the [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) resource. The web interface and the Application validating webhook use the published schemas. Only standard OpenAPI keywords and the `x-deckhouse-*` extensions described on this page are published. Other `x-*` extensions are removed from the published schema.

## How DP builds values

DP builds values from the following sources, in the order listed:

1. Default values from `openapi/settings.yaml`.
2. Project defaults for fields with the [`x-deckhouse-grantable-resource`](#defaulting-a-grantable-cluster-wide-resource-value-x-deckhouse-grantable-resource) extension.
3. User settings from `Application.spec.settings`. They override the values from the previous sources.
4. Default values from `openapi/values.yaml` for the fields that are still not set.
5. Values set by hooks.

Templates get the result as `.Values`. The effective settings (sources 1–3) are also available to templates as `.Application.Settings`. After the settings are applied successfully, DP saves the effective settings in the `status.lastAppliedConfiguration` field of the Application.

## Validation

### Undeclared fields

DP adds `additionalProperties: false` to every object of both schemas that doesn't define `additionalProperties` explicitly. As a result, a setting or a value that is not declared in the schema is rejected.

To allow arbitrary keys in an object, for example, in a map of labels, define `additionalProperties` explicitly:

```yaml
type: object
properties:
  podLabels:
    type: object
    additionalProperties:
      type: string
```

{% alert level="warning" %}
Fields declared only inside `oneOf`, `anyOf`, or `allOf` branches are also rejected. Declare every field of an object in its `properties`, and use the branches only to restrict combinations of values.
{% endalert %}

### When settings are validated

| When | What DP checks | Result of a failed check |
|---|---|---|
| An Application is created or changed | The settings schema published in ApplicationPackageVersion, including CEL rules that don't reference `oldSelf`, and fields with `x-deckhouse-immutable` | The request is rejected |
| The settings of an installed application are changed (`spec.packageVersion` matches the installed version) | Additionally: CEL rules that reference `oldSelf`, availability of grantable resources, and the [settings validation hook](hooks.html#settings-validation-hook) | The request is rejected |
| DP applies the settings | The same as in the previous row | The settings are not applied, the error is reported in the Application conditions |
| A hook changes values | The `openapi/values.yaml` schema, including CEL rules | The hook fails |

### CEL rules (x-deckhouse-validations)

Both schemas support the `x-deckhouse-validations` extension with rules in the Common Expression Language (CEL). The syntax and the semantics are the same as in [module schemas](../module-development/structure/#validations-with-x-deckhouse-validations): a rule consists of `expression` and `message`, the value at the current level is available as `self`, and the previous value is available as `oldSelf`.

Example `openapi/settings.yaml` with a rule that checks two fields together:

```yaml
type: object
properties:
  replicas:
    type: integer
    default: 1
  maxReplicas:
    type: integer
    default: 3
x-deckhouse-validations:
  - expression: "self.replicas <= self.maxReplicas"
    message: "replicas must not exceed maxReplicas"
```

Rules that reference `oldSelf` compare the new settings with the previously applied ones. Such rules are checked only when the settings of an installed application change: they are skipped on installation and when `spec.packageVersion` changes. To forbid changing a field after installation, use [`x-deckhouse-immutable`](#immutable-fields-x-deckhouse-immutable), which is checked on every update.

### Defaulting a grantable cluster-wide resource value (x-deckhouse-grantable-resource)

A `settings` field of type `string` can be bound to a grantable cluster-wide resource managed by the
[`multitenancy-manager`](/modules/multitenancy-manager/) (for example, a StorageClass).

When the field is bound and the user leaves it empty, the resource name configured as the project default is injected into `values`. When the user provides a value, it is checked against the resources available to the project. A value that is not in this list is rejected.

To bind the field to a cluster-wide resource, add the `x-deckhouse-grantable-resource` extension and specify the name of the resource available to the project through a grant (`AvailableClusterResource` or `GrantableClusterResourceDefinition`), for example, `storageclasses`.

{% alert level="info" %}
The resource group, version, and kind (GVK) are defined by the grant. You do not need to specify them in `openapi/settings.yaml`.
{% endalert %}

Example `openapi/settings.yaml` with `x-deckhouse-grantable-resource`:

```yaml
type: object
properties:
  storageClass:
    type: string
    x-deckhouse-grantable-resource: storageclasses
  postgres:
    type: object
    properties:
      storageClass:
        type: string
        x-deckhouse-grantable-resource: postgresclasses
```

Behavior:

- The default is resolved per project from the AvailableClusterResource in the Application's namespace, so different projects can receive different defaults.
- An explicitly specified user value has higher priority than the project default.
- If the custom resource definition (CRD) is absent, no catalog exists for the project, or the catalog has no default value, the field remains unchanged. No value is injected or validated.

### Immutable fields (x-deckhouse-immutable)

Some settings should not be changed after the application configuration is applied. For example, changing a `storageClass` after volumes have been created either has no effect or can cause application failures. To prevent changes to the value of such a field after the application configuration has been successfully applied, add the `x-deckhouse-immutable: true` extension to it.

Example `openapi/settings.yaml` with `x-deckhouse-immutable`:

```yaml
type: object
properties:
  storageClass:
    type: string
    default: default
    x-deckhouse-immutable: true
  postgres:
    type: object
    properties:
      storageClass:
        type: string
        x-deckhouse-immutable: true
      volumeSize:
        type: string
```

Behavior:

- The extension is effective only when set to `true`. Any other value is ignored.
- When `x-deckhouse-immutable` is added to an object, the entire object becomes immutable. After the application configuration has been successfully applied for the first time, changing any nested field is rejected, even if `x-deckhouse-immutable` is not set on that field. Set the extension on an object only when the entire object must be immutable. To make only one field immutable, add `x-deckhouse-immutable` directly to it, as with `postgres.storageClass` in the example above. In this case, the restriction does not apply to `postgres.volumeSize`, and its value can be changed.
- If an update changes an immutable value, the validating webhook rejects it and reports the field name.
- In the web interface, the value of a field with the extension can be set when installing the application. In the edit form of an installed application, the field is read-only.
- Comparison uses the configuration that was actually applied, after schema defaults are applied. Therefore, a field with the extension can be omitted from the manifest only if its `default` matches the value that has already been applied. If there is no default value or it differs from the applied value, the change is rejected.

{% alert level="info" %}
If an entire object is removed from the manifest, default values are not applied to its nested fields. Therefore, removing an object that contains fields using `x-deckhouse-immutable` can cause frozen values to be lost and is rejected.
{% endalert %}

- The mark is not inherited into array elements or map entries that the update adds — a new element has no previous value to be frozen against.

## Web interface form

The web interface builds the application settings form from `openapi/settings.yaml`. The following extensions control how fields are displayed:

| Extension | Value | Effect |
|---|---|---|
| `x-deckhouse-ui-order` | Integer | Display order of the field. Fields with lower values are shown first |
| `x-deckhouse-ui-group` | String | The field is shown in the root-level group with this name. The path of the field in the settings doesn't change |
| `x-deckhouse-ui-advanced` | `true` | The field is hidden behind the advanced settings toggle. The extension is taken into account only for top-level fields |
| `x-deckhouse-ui-validation-message` | String | The message shown under the field instead of the schema validation error, for example, instead of a regular expression that the value doesn't match |
| `x-deckhouse-ui-resource-name` | Object with the `apiVersion` and `kind` fields and an optional `labelSelector` field | For a field of type `string`: a drop-down list with the names of resources of the specified kind from the application namespace. `labelSelector` is a standard Kubernetes label selector that narrows the list |
| `x-deckhouse-enum-switch-settings` | List, declared on a `oneOf` branch | Confirmation dialogs shown before switching to another branch. See [the next section](#confirmation-before-switching-a-mode-x-deckhouse-enum-switch-settings) |

Example `openapi/settings.yaml` with form extensions:

```yaml
type: object
properties:
  replicas:
    type: integer
    default: 1
    x-deckhouse-ui-order: 10
  domain:
    type: string
    pattern: '^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$'
    x-deckhouse-ui-order: 20
    x-deckhouse-ui-validation-message: "Enter a domain name, for example, app.example.com."
  tlsSecretName:
    type: string
    x-deckhouse-ui-group: TLS
    x-deckhouse-ui-resource-name:
      apiVersion: v1
      kind: Secret
      labelSelector:
        matchLabels:
          example.com/tls: "true"
  logLevel:
    type: string
    enum: [Info, Debug]
    default: Info
    x-deckhouse-ui-advanced: true
```

### Confirmation before switching a mode (x-deckhouse-enum-switch-settings)

Some settings switches can't be reverted without data loss, for example, switching the storage of an application from an in-cluster one to an external one. To make the web interface ask for a confirmation before such a switch, add the `x-deckhouse-enum-switch-settings` extension to a `oneOf` branch. The confirmation is shown when the branch discriminator (the field whose `enum` value selects the branch) changes from a value of this branch to one of the target values.

Each list item contains the following fields:

- `to` — target discriminator values the confirmation applies to. The values are compared as strings, so boolean discriminators are specified as `"true"` and `"false"`.
- `impact` — severity of the switch: `info` (default), `warning`, or `destructive`. The web interface applies a `destructive` switch only after an explicit acknowledgement and records it in the `applications.deckhouse.io/allow-destructive-settings-switch` annotation of the Application.
- `messages` — dialog text in Markdown by language code (`en`, `ru`).

Example `openapi/settings.yaml` with `x-deckhouse-enum-switch-settings`:

```yaml
type: object
properties:
  storage:
    type: object
    default: {}
    properties:
      mode:
        type: string
        enum: [Internal, External]
        default: Internal
      size:
        type: string
      url:
        type: string
    oneOf:
      - properties:
          mode:
            enum: [Internal]
        x-deckhouse-enum-switch-settings:
          - to: [External]
            impact: destructive
            messages:
              en: "Switching to external storage **deletes the data** stored in the cluster."
              ru: "<RU_MESSAGE>"
      - properties:
          mode:
            enum: [External]
        required: [url]
```
