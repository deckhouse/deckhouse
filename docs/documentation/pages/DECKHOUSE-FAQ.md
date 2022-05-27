---
title: FAQ
permalink: en/deckhouse-faq.html
---

## How do I find out all Deckhouse parameters?

All the essential Deskhouse settings (including module parameters) are stored in the `deckhouse` ConfigMap in the `d8-system` namespace.

To view Deckhouse settings use the following command:

```shell
kubectl -n d8-system get cm deckhouse -o yaml
```

## How do I find the documentation for the version installed?

> Documentation in the cluster is available when the [deckhouse-web](modules/810-deckhouse-web/) module is enabled (it is enabled by default except the `Minimal` [bundle](modules/020-deckhouse/configuration.html#parameters-bundle)).

The documentation for the Deckhouse version running in the cluster is available at `deckhouse.<cluster_domain>`, where `<cluster_domain>` is the DNS name that matches the template defined in the `global.modules.publicDomainTemplate` parameter.

## How do I set the desired release channel?

Change (set) the `releaseChannel` parameter in the `deckhouse` module [configuration](modules/020-deckhouse/configuration.html#parameters-releasechannel) to automatically switch to another release channel.

It will activate the mechanism of [automatic stabilization of the release channel](#how-does-automatic-deckhouse-update-work).

Here is an example of the module configuration:

```yaml
deckhouse: |
  releaseChannel: Stable
```

## How do I disable automatic updates?

To completely disable the Deckhouse update mechanism, remove the `releaseChannel` parameter in the `deckhouse' module [configuration](modules/020-deckhouse/configuration.html#parameters-releasechannel).

In this case, Deckhouse does not check for updates and even doesn't apply patch releases.

> It is highly not recommended to disable automatic updates! It will block updates to patch releases that may contain critical vulnerabilities and bugs fixes.

## How does automatic Deckhouse update work?

Every minute Deckhouse checks a new release appeared in the release channel specified by the `releaseChannel` parameter.

When a new release appears on the release channel, Deckhouse downloads it and creates CustomResource `DeckhouseRelease`.

After creating a `DeckhouseRelease` CR in a cluster, Deckhouse updates the `deckhouse` Deployment and sets the image tag to a specified release tag according to [selected](modules/020-deckhouse/configuration.html#parameters-update) update mode and update windows (automatic at any time by default).

To get list and status of all releases use the following command:

```shell
kubectl get deckhousereleases
```

> Patch releases (e.g., an update from version `1.30.1` to version `1.30.2`) ignore update windows settings and apply as soon as they are available.

### Change the release channel

* When switching to a **more stable** release channel (e.g., from `Alpha` to `EarlyAccess`), Deckhouse downloads release data from the release channel (the `EarlyAccess` release channel in the example) and compares it with the existing `DeckhouseReleases`:
  * Deckhouse deletes *later* releases (by semver) that have not yet been applied (with the `Pending` status).
  * if *the latest* releases have been already Deployed, then Deckhouse will hold the current release until a later release appears on the update channel (on the `EarlyAccess` release channel in the example).
* When switching to a less stable release channel (e.g., from `EarlyAcess` to `Alpha`), the following actions take place:
  * Deckhouse downloads release data from the release channel (the `Alpha` release channel in the example) and compares it with the existing `DeckhouseReleases`.
  * Then Deckhouse performs the update according to the [update parameters](modules/020-deckhouse/configuration.html#parameters-update).

## How do I run Deckhouse on a particular node?

Set the `nodeSelector` [parameter](modules/020-deckhouse/configuration.html) of the `deckhouse` module and avoid setting `tolerations`. The necessary values will be assigned to the `tolerations` parameter automatically.

You should also avoid using **CloudEphemeral** nodes. Otherwise, a situation may occur when the target node is not in the cluster and node ordering for some reason is impossible.

Here is an example of the module configuration:

```yaml
deckhouse: |
  nodeSelector:
    node-role.deckhouse.io/deckhouse: ""
```

## How do I configure Deckhouse to use a third-party registry?

Deckhouse can be configured to work with a third-party registry (e.g., a proxy registry inside private environments).

### Configuring

Define the following parameters in the `InitConfiguration` resource:

* `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/<DECKHOUSE_REVISION>`. The path to the Deckhouse image in the third-party registry matching the edition used (CE/EE), for example `imagesRepo: registry.deckhouse.io/deckhouse/ce`;
* `registryDockerCfg: <BASE64>`. Base64-encoded auth credentials of the third-party registry.

Use the following `registryDockerCfg` if anonymous access to Deckhouse images is allowed in the third-party registry:

```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```

`registryDockerCfg` must be Base64-encoded.

Use the following `registryDockerCfg` if authentication is required to access Deckhouse images in the third-party registry:

```json
{"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
```

* `<PROXY_USERNAME>` — auth username for `<PROXY_REGISTRY>`.
* `<PROXY_PASSWORD>` — auth password for `<PROXY_REGISTRY>`.
* `<PROXY_REGISTRY>` — registry address: `<HOSTNAME>[:PORT]`.
* `<AUTH_BASE64>` — Base64-encoded `<PROXY_USERNAME>:<PROXY_PASSWORD>` auth string.

`registryDockerCfg` must be Base64-encoded.

The `InitConfiguration` resource provides two more parameters for non-standard third-party registry configurations:

* `registryCA` - root CA certificate to validate the third-party registry's HTTPS certificate (if self-signed certificates are used);
* `registryScheme` - registry scheme (`http` or `https`). The default value is `https`.

### Tips for configuring the third-party registry

**Note** that Deckhouse only supports Bearer authentication for registries.

#### Nexus

The following parameters must be set if the [Nexus](https://github.com/sonatype/nexus-public) repository manager is used:

* Enable `Docker Bearer Token Realm`:
  ![](images/registry/nexus/nexus1.png)

* Enable anonymous registry access (otherwise, Bearer authentication [won't work](https://help.sonatype.com/repomanager3/system-configuration/user-authentication#UserAuthentication-security-realms)):
  ![](images/registry/nexus/nexus2.png)

* Set the `Maximum metadata age` to 0 (otherwise, the automatic update of Deckhouse will fail due to caching):
  ![](images/registry/nexus/nexus3.png)

#### Harbor

You need to use the Proxy Cache feature of a [Harbor](https://github.com/goharbor/harbor).

* Create a Registry:
  * `Administration -> Registries -> New Endpoint`.
  * `Provider`: `Docker Registry`.
  * `Name` — specify any of your choice.
  * `Endpoint URL`: `https://registry.deckhouse.io`.
  * Specify the `Access ID` and `Access Secret` if you use Deckhouse Enterprise Edition; otherwise, leave them blank.

![](images/registry/harbor/harbor1.png)

* Create a new Project:
  * `Projects -> New Project`.
  * `Project Name` will be used in the URL. You can choose any name, for example, `d8s`.
  * `Access Level`: `Public`.
  * `Proxy Cache` — enable and choose the Registry, created in the previous step.

![](images/registry/harbor/harbor2.png)

Thus, Deckhouse images will be available at `https://your-harbor.com/d8s/deckhouse/{d8s-edition}:{d8s-version}`.

## How do I switch a running Deckhouse cluster to use a third-party registry?

To switch the Deckhouse cluster to using a third-party registry, follow these steps:

* Update the `image` field in the `d8-system/deckhouse` deployment to contain the address of the Deckhouse image in the third-party-registry;
* Edit the `d8-system/deckhouse-registry` secret (note that all parameters are Base64-encoded):
  * Insert third-party registry credentials into `.dockerconfigjson`.
  * Replace `address` with the third-party registry's host address (e.g., `registry.example.com`).
  * Change `path` to point to a repo in the third-party registry (e.g., `/deckhouse/ee`).
  * If necessary, change `scheme` to `http` (if the third-party registry uses HTTP scheme).
  * If necessary, change or add the `ca` field with the root CA certificate that validates the third-party registry's https certificate (if the third-party registry uses self-signed certificates).
* Wait for the Deckhouse Pod to become `Ready`. Restart Deckhouse Pod if it will be in `ImagePullBackoff` state.
* Wait for bashible to apply the new settings on the master node. The bashible log on the master node (`journalctl -u bashible`) should contain the message `Configuration is in sync, nothing to do`.
* Only if Deckhouse won't be updated using a third-party registry, then you have to remove `releaseChannel` setting from configmap `d8-system/deckhouse`.
