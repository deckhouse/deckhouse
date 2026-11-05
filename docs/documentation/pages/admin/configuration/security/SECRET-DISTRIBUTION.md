---
title: Automatic distribution of secrets across namespaces
permalink: en/admin/configuration/security/secret-distribution.html
description: "Configure automatic secret distribution across namespaces in Deckhouse Platform. Secret replication, CI/CD integration, and secure secret management automation."
---

Deckhouse Platform (DP) lets you automatically distribute secrets across all namespaces in the cluster.
This helps eliminate the need to manually copy secrets in CI/CD pipelines — for example,
when pulling container images or working with Ceph RBD storage.

## How it works

- Deckhouse monitors secrets in the `default` namespace
  that have the special label `secret-copier.deckhouse.io/enabled: ""`.
- These secrets are automatically copied to all namespaces in the cluster.
- When the original secret in the `default` namespace is updated, the new content is propagated to all namespaces.
- If the original secret in `default` is deleted, all of its copies in other namespaces are also deleted.
- If a secret copy is manually modified in a namespace,
  it will be automatically overwritten with the content of the original secret from `default`.
- When new namespaces are created,
  all secrets from `default` labeled with `secret-copier.deckhouse.io/enabled: ""` are automatically copied into them.
- There's a regular nightly synchronization that ensures all secrets remain up to date.
- Deckhouse Platform never creates, updates or deletes the secret `d8-system/cluster-key` this way: it holds the cluster key of Deckhouse Platform licensing.
- Secrets of the `bootstrap.kubernetes.io/token` type are not copied. The copies of such secrets that earlier Deckhouse Platform versions made are deleted.
- Deckhouse Platform updates and deletes only the copies it made. If a namespace already has a secret with the same name that Deckhouse Platform did not create, the secret is left as is and is not copied into that namespace.

### Incompatibility with multitenancy mode

The automatic [secret distribution](../../../glossary.html#secret-distribution) mechanism is not compatible with the [multitenancy mode](../../multitenancy.html).

Multitenancy mode creates isolated environments for users within [projects](../../../glossary.html#project).
Automatically distributing secrets to all namespaces may violate this security model.
If sensitive data accidentally reaches a user’s isolated environment, it may lead to data exposure.

If you need to provide a shared certificate (for example, a wildcard certificate for an internal environment)
or a shared registry access token, do not use automatic distribution.
Instead, include such secrets in the project template.
A cluster administrator must define them in the project configuration.

## Configuring automatic secret distribution

1. Create a Secret resource in the `default` namespace.
1. Add the label `secret-copier.deckhouse.io/enabled: ""`.
1. Apply the resource using the `d8 k apply` command.
   It will then be automatically distributed to all namespaces.

{% alert level="warning" %}
Only secrets from the `default` namespace are automatically distributed.
Secrets with the label `secret-copier.deckhouse.io/enabled: ""` created in any other namespace
will be automatically deleted.
{% endalert %}

## Distributing secrets to selected namespaces

To copy a secret to specific namespaces only,
use the annotation with a label selector `secret-copier.deckhouse.io/target-namespace-selector: "app=custom"`.
In this case, the secret will be copied only to namespaces that match the specified selector.
