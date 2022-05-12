---
title: "The namespace-configurator module"
---

This module allows to assign annotations and labels to namespaces automatically.

It facilitates to enable new namespaces to monitoring system by adding `extended-monitoring.flant.com/enabled=true` annotation.

### How does it work?

This module monitors the namespaces and the configuration.
* All namespaces matching pattern from `includeNames` and not matching pattern from `excludeNames`, will have assigned labels and annotations according to the configuration;
* When changing the module configuration, the corresponding labels and annotations will be reassigned according to the configuration;

### What do I need to configure?

All you need to do is to specify list of desired labels and annotations and matching patterns for namespaces in the module configuration.
