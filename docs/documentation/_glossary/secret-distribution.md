---
title: Secret distribution
description: Automatic copying of secrets from the default namespace that carry the `secret-copier.deckhouse.io/enabled` label to all namespaces of the cluster.
lang: en
---

Deckhouse Platform copies the labelled secrets to every existing and every new [namespace](glossary.html#namespace), propagates updates of the original, overwrites manual changes of the copies and deletes the copies when the original is deleted. A nightly synchronization keeps the copies up to date. This removes manual copying of secrets in CI/CD pipelines, for example registry credentials.

The mechanism is not compatible with the multitenancy mode: a secret distributed to all namespaces can reach the isolated environment of a [project](glossary.html#project). Shared secrets for projects are defined in the project template instead.

For details, see [Automatic distribution of secrets across namespaces](admin/configuration/security/secret-distribution.html).
