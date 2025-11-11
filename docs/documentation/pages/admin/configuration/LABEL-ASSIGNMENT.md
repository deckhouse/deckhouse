---
title: Automatic assignment of namespace labels and annotations
permalink: en/admin/configuration/label-assignment.html
---

You can automate the assignment of labels and annotations to namespaces in a Deckhouse cluster
based on predefined patterns.
This can be useful, for example, to automatically include new namespaces in monitoring
without having to manually edit each one.

## How it works

- All namespaces whose names match the patterns in `includeNames` and do not match patterns in `excludeNames`
  automatically receive the specified labels and annotations.
- When the configuration is changed, labels and annotations of existing namespaces are updated accordingly.
- Newly created namespaces that match the defined patterns also receive the specified labels and annotations automatically.

## Configuring automatic label and annotation assignment

Enable the [`namespace-configurator`](/modules/namespace-configurator/) module:

```shell  
d8 system module enable namespace-configurator
```

Configure automatic label and annotation assignment in ModuleConfig [`namespace-configurator`](/modules/namespace-configurator/configuration.html):

1. Specify the annotations and labels to be applied to namespaces in the fields `settings.configurations.annotations` and `settings.configurations.labels`, respectively;
1. Define the matching rules for namespace names:
   - In `includeNames`, list regular expressions for names you want to match.
   - In `excludeNames`, list names that should be excluded.

## Example configuration

In the following configuration example, automatic addition of the label `extended-monitoring.deckhouse.io/enabled=true` and the annotation `foo=bar` is configured for all namespaces whose names start with `prod-` or `infra-`, except for `infra-test`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: namespace-configurator
spec:
  version: 1
  enabled: true
  settings:
    configurations:
    - annotations:
        foo: bar
      labels:
        extended-monitoring.deckhouse.io/enabled: "true"
      includeNames:
      - "^prod"
      - "^infra"
      excludeNames:
      - "infra-test"
```
