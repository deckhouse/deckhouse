---
title: "The namespace-configurator module"
---

The `namespace-configurator` module allows to assign annotations and labels to namespaces automatically.

It facilitates to enable new namespaces to monitoring system by adding `extended-monitoring.deckhouse.io/enabled=true` label.

## How does it work?

This module monitors the namespaces and configuration:

* It assigns labels and annotations from the configuration to all namespaces that match the `includeNames` pattern and do not match the `excludeNames` pattern.
* When changing the module configuration, namespace labels and annotations will be reassigned according to the configuration.

## What do I need to configure?

All you need to do is to specify list of desired labels and annotations and matching patterns for namespaces in the module configuration.
