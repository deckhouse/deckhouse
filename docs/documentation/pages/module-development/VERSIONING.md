---
title: "Versioning modules"
permalink: en/module-development/versioning/
---

We use [semantic versioning](https://semver.org/) to control module versions.

When choosing a version, stick to the following guidelines:
- updating the **patch version** (e.g., from `0.0.1` to `0.0.2`) means that some issue has been fixed;
- updating the **minor version** (e.g., from `0.0.1` to `0.1.0`) means that some new feature has been added;
- updating the **major version** (e.g., from `0.0.1` to `1.0.0`) means that a feature has been added that radically changes the module's capabilities; the interface has undergone significant changes or a major phase of development has been completed.

The git tag and registry container **always** have a "v" before the version number, e.g., `v0.0.73` or `v1.0.0`.

## Release channels

Once published, the module version *moves* through the [release channels](../../deckhouse-release-channels.html) from less stable to more stable: `Alpha` -> `Beta` -> `EarlyAccess` -> `Stable` -> `RockSolid`.

Release channels allow you to publish a version of a module to a limited group of users and get early feedback. You decide how stable the module version is and to which release channel you want to publish it.

Note that the choice of a specific release channel does not determine how stable a module is or what stage of its lifecycle it is in. Channels are a delivery tool and are intended to measure the stability level of a particular release.

## Module lifecycle

During development, a module may be at any of the following stages:

**Experimental** refers t to an experimental version. The module functionality may undergo significant changes. Compatibility with future versions is not guaranteed.

**Preview** refers to a preliminary version. The module functionality may change, but the basic features will be preserved. Compatibility with future versions is ensured, but may require additional migration actions.

**General Availability (GA)** refers to a generally available version. The module is ready to be used in production environments.

**Deprecated** refers to a module version that has been deprecated.

## How do I figure out how stable a module is?

Depending on the stage of the module lifecycle and the release channel from which the specific module version was installed, the overall stability can be determined according to the following table:

![Module_Stability](../../images/module-development/module_stability.png)

Highlights:
- `Experimental` modules in the `Stable` channel are not recommended for use in production environments.
- `GA` modules in the `Alpha` channel are also not recommended for use in production environments.
- Only `GA` modules installed from `EarlyAccess`, `Stable`, or `RockSolid` channels are suitable for production environments.
- `Deprecated` modules are recommended to be replaced according to the guidelines provided in the documentation.

<!--
## Stages of specific module features @TODO

The *ModuleConfig* resource allows you to control additional module options. These options can be marked as `Experimental`, `Preview`, `GA` or `Deprecated` in the `x-feature-stage` parameter in the OpenAPI schema `x-feature-stage: Experimental|Preview|GA|Deprecated` (the default value is `GA`).

A warning is shown when attempting to enable functions that have stages other than `GA`.

In the Deckhouse Kubernetes Platform (DKP) settings, you can define global rules that determine which features and at what stage can be enabled in a cluster. This helps prevent Experimental features from being used accidentally in production environments.
-->

## API versioning

Modules in DKP use custom resources to interact with users. The `apiVersion` parameter with the API version of these resources is set according to the following rules:

- `v1alphaX` refers to an API that has just been published. This API needs to be tested to see how user-friendly it is, as well as how valid and consistent its settings are.
- `v1betaX` refers to the API that has passed initial testing. Its logical development and refinement is in progress.
- `v1stableX` refers to a stable API. At this point, its fields are kept in the specification while its validation rules do not change to be more stringent.

You can release a new version v2 of the API that goes through the same steps, but with the prefix `v2`. Keep in mind that once `v1stableX` has been released, Kubernetes will treat it as higher priority than `alpha` or `beta` versions until a new stable version of `v2stableX` is released. The `kubectl apply` and `kubectl edit` commands will use `v1stableX`.

The reasons for releasing a new version may be as follows:
* structure changes;
* updating obsolete parameters.

You can add new parameters without changing the version.

To enable automatic conversion of module parameters from one version to another, you must include the appropriate [conversions](../structure/#conversions) in the module. Conversions may be necessary when a parameter is renamed or moved to a different location in a new version of the OpenAPI specification.

Please follow these recommendations when releasing a new version of the *CustomResourceDefinition* (CRD):
* Set `deprecated: true` for previous versions (read more in the [Kubernetes documentation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#version-deprecation)).
* The version that stores data in etcd ([storage version](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#upgrade-existing-objects-to-a-new-stored-version)) should not be changed earlier than two months after the new version has been released.
