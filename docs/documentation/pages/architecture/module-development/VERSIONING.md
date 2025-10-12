---
title: "Versioning modules"
permalink: en/architecture/module-development/versioning/
---

Deckhouse Kubernetes Platform (DKP) uses [semantic versioning](https://semver.org/lang/en/) for modules.

When choosing a version, stick to the following guidelines:

- updating the **patch version** (e.g., from `0.0.1` to `0.0.2`) means that some issue has been fixed;
- updating the **minor version** (e.g., from `0.0.1` to `0.1.0`) means that some new feature has been added;
- updating the **major version** (e.g., from `0.0.1` to `1.0.0`) means that a feature has been added that radically changes the module's capabilities; the interface has undergone significant changes or a major phase of development has been completed.

The git tag and registry container **always** have a "v" before the version number, e.g., `v0.0.73` or `v1.0.0`.

## Release channels

Release channels allow you to publish a module version not for all users at once, but for a limited group of users. You define the stability level of the module version and the maximum release channel to which it can be published.

Note that the choice of a specific release channel does not determine how stable the module is. The channels are an instrument for delivery of updates and determine the degree of stability of a particular release.

When publishing a new version of the module to the [release channel](../../../reference/release-channels.html), first use the `Alpha` release channel. Then, if module version has no issues, publish the module version sequentially to other release channels, taking their stability into account: `Alpha` → `Beta` → `Early Access` → `Stable` → `Rock Solid`. If the module version requires fixing, the publication of that version should be halted. After releasing a version with fixes, the publication stage must be repeated starting from the `Alpha` release channel.

### Procedure for publishing a new version

The recommended sequence of publication of the module version in release channels:

1. Publish a new version of the module in the `Alpha` release channel.
1. If the version works stably, publish it in the following release channels sequentially:
   `Beta` → `Earlyaccess` →`Stable` → `Rocksolid`.
1. If errors arise, stop the publication and correct them.
1. Repeat the publication of the version, starting from the release channel `alpha`.

## Module lifecycle

During its life cycle, the module goes through the following stages:

- **Experimental** refers t to an experimental version. The module functionality may undergo significant changes. Compatibility with future versions is not guaranteed.
  > Modules in the `Experimental` stage cannot be enabled by default.
  > To allow the use of such modules, set the [allowExperimentalModules parameter](/modules/deckhouse/configuration.html#parameters-allowexperimentalmodules) to true.
- **Preview** refers to a preliminary version. The module functionality may change, but the basic features will be preserved. Compatibility with future versions is ensured, but may require additional migration actions.
- **General Availability (GA)** refers to a generally available version. The module is ready to be used in production environments.
- **Deprecated** refers to a module version that has been deprecated. Further development and support of the module is discontinued.

## Definition of module stability

Depending on the stage of the module lifecycle and the release channel from which the specific module version was installed, the overall stability can be determined by the following table:

<table class="versioning-table">
<thead>
    <tr class="header-row">
      <th rowspan="2">Lifecycle Stage</th>
      <th colspan="5">Release Channels</th>
    </tr>
    <tr class="sub-header">
      <th>Alpha</th>
      <th>Beta</th>
      <th class="middle">Early Access</th>
      <th>Stable</th>
      <th>Rock Solid</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><strong>Experimental</strong></td>
      <td style="background-color:#FFCDD2">Experiments</td>
      <td style="background-color:#FFCDD2">Experiments</td>
      <td style="background-color:#FFCDD2">Experiments</td>
      <td style="background-color:#FFE0B2">Beta testing</td>
      <td style="background-color:#FFE0B2">Beta testing</td>
    </tr>
    <tr>
      <td><strong>Preview</strong></td>
      <td style="background-color:#FFCDD2">Experiments</td>
      <td style="background-color:#BBDEFB">Limited use</td>
      <td style="background-color:#BBDEFB">Limited use</td>
      <td style="background-color:#C8E6C9">Production use</td>
      <td style="background-color:#C8E6C9">Production use</td>
    </tr>
    <tr>
      <td><strong>General Availability</strong></td>
      <td style="background-color:#FFCDD2">Experiments</td>
      <td style="background-color:#BBDEFB">Limited use</td>
      <td style="background-color:#BBDEFB">Limited use</td>
      <td style="background-color:#C8E6C9">Production use</td>
      <td style="background-color:#A5D6A7">Production use in critical systems</td>
    </tr>
    <tr>
      <td><strong>Deprecated</strong></td>
      <td style="background-color:#F5F5F5">Deprecated</td>
      <td style="background-color:#F5F5F5">Deprecated</td>
      <td style="background-color:#F5F5F5">Deprecated</td>
      <td style="background-color:#F5F5F5">Deprecated</td>
      <td style="background-color:#F5F5F5">Deprecated</td>
    </tr>
  </tbody>
</table>

- **Experiments** — Functionality checks, experiments, and testing;
- **Beta testing** — Functionality checks, experiments, and testing. Limited use by experienced users in environments equivalent to production;
- **Limited use** — Development environments, pilot projects, low-impact production environments;
- **Production use** — Production environments and those equivalent to them;
- **Production use in critical systems** — Mission-critical production environments and those equivalent to them;
- **Deprecated** — Should be phased out of use.

Highlights:

- A module in `Experimental` status on the `Stable` channel is recommended to be used in production environments only with limitations.
- A module in `General Availability` status on the `Alpha` channel is also not recommended for use in production environments.
- Only `GA` modules installed from `Early Access`, `Stable`, or `Rock Solid` channels are suitable for production environments.
- `Deprecated` modules are recommended to be replaced.

## API versioning

Modules in DKP use custom resources to interact with users. The `apiVersion` parameter with the API version of these resources is set according to the following rules:

- `v1alphaX` refers to an API that has just been published. This API needs to be tested to see how user-friendly it is, as well as how valid and consistent its settings are.
- `v1betaX` refers to the API that has passed initial testing. Its logical development and refinement is in progress.
- `v1stableX` refers to a stable API. At this point, its fields are kept in the specification while its validation rules do not change to be more stringent.

If necessary, a new version of API v2 can be released, which goes through the same stages but with the prefix `v2`. It's important to remember that after the release of the `v1stableX` version, Kubernetes will consider it more prioritized than the `alpha` or `beta` versions until a new stable version `v2stableX` is released. When executing `kubectl apply` and `kubectl edit` commands, the `v1stableX` version will be used by default.

### Release of the new API version

The reasons for releasing a new version may be as follows:

- structure changes;
- updating obsolete parameters.

You can add new parameters without changing the version.

### Automatic API Conversion

To enable automatic conversion of module parameters from one version to another, you can include the appropriate [module conversions](../structure/#conversions). Conversions may be necessary when a parameter is renamed or moved to a different location in a new version of the OpenAPI specification.

### Recommendations for Releasing New Versions of CRD

Please follow these recommendations when releasing a new version of the CustomResourceDefinition (CRD):

- Set the parameter `deprecated: true` for previous versions of the CRD. For more details on working with deprecated versions of CRD, refer to the [Kubernetes documentation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#version-deprecation).
- Do not change the [storage version](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#upgrade-existing-objects-to-a-new-stored-version), in which data is stored inside etcd, until at least two months have passed since the release of the new version.
