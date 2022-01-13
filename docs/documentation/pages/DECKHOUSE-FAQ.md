---
title: FAQ
permalink: en/deckhouse-faq.html
---

## How do I find out all Deckhouse parameters?

All the essential Deskhouse settings (including module parameters) are stored in the `deckhouse` ConfigMap in the `d8-system` namespace. You can view its contents using the command below:
```
kubectl -n d8-system get cm deckhouse -o yaml
```

## How do I find the documentation for the version installed?

The documentation for the Deckhouse version running in the cluster is available at `deckhouse.<cluster_domain>`, where `<cluster_domain>` is the DNS name that matches the template defined in the `global.modules.publicDomainTemplate` parameter.

## How do I set the desired release channel?
Change (set) the module's `releaseChannel` parameter to automatically switch to another release channel (and minimize version drift in the cluster). It will activate the mechanism of [automatic stabilization of the release channel](#how-does-the-mechanism-of-automatic-stabilization-of-the-release-channel-work).

Here is an example of the module configuration:
```yaml
deckhouse: |
  releaseChannel: RockSolid
```

## How does the mechanism of automatic stabilization of the release channel work?
Deckhouse will switch to the image with the corresponding Docker image tag in response to setting the `releaseChannel` parameter. No other action is required on the part of the user.

**Note:** Switching is not instantaneous and relies on the Deckhouse update process.

The release channel stabilization script runs every 10 minutes. It implements the following algorithm:
* If the specified release channel matches the Deckhouse Docker image's tag — do nothing;
* When switching to a more stable release channel (e.g., `Alpha` -> `EarlyAccess`), the gradual transition takes place:

  - First, the script compares the [digests](https://success.mirantis.com/article/images-tagging-vs-digests) of Docker image tags that correspond to the current release channel and the next more stable channel (`Alpha` and `Beta` in our example).

  - If the digests are equal, the script checks the next tag (in our example, this tag corresponds to the `EarlyAccess` release channel).

  - In the end, Deckhouse will switch to a more stable release channel with a digest equal to the current one.

* Suppose a less stable release channel is specified than the channel that corresponds to the current tag of the Deckhouse Docker image. In that case, the script compares digests corresponding to the Docker images for the current release channel and the next, less stable one. For example, when switching to the `Alpha` channel from the `EarlyAccess` channel, the script compares the  `EarlyAccess` and `Beta` channels:

  - If the digests are not equal, Deckhouse switches to the next release channel (`Beta` in our case). Such an approach ensures that some crucial migrations are performed during Deckhouse upgrades.

  - If the digests are equal, the script checks the next less stable release channel (`Alpha` in our case).

  - When the script reaches the desired release channel (`Alpha` in our example), Deckhouse will switch to it regardless of the digest comparison results.

Since the stabilization script runs continuously, Deckhouse will eventually end up in a state where the tag of its Docker image corresponds to the release channel selected.

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
- `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/<DECKHOUSE_REVISION>`. The path to the Deckhouse image in the third-party registry matching the edition used (ce/ee/fe), for example `imagesRepo: registry.deckhouse.io/deckhouse/ce`.
- `registryDockerCfg: <BASE64>`. BASE64-encoded auth credentials of the third-party registry.

Use the following `registryDockerCfg` if anonymous access to Deckhouse images is allowed in the third-party registry:
```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```
`registryDockerCfg` must be BASE64-encoded.

Use the following `registryDockerCfg` if authentication is required to access Deckhouse images in the third-party registry:
```json
{"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
```

`<AUTH_BASE64>` — BASE64-encoded `<PROXY_USERNAME>:<PROXY_PASSWORD>` auth string.

registryDockerCfg` must be BASE64-encoded.

* `<PROXY_USERNAME>` — auth username for `<PROXY_REGISTRY>`.
* `<PROXY_PASSWORD>` — auth password for `<PROXY_REGISTRY>`.
* `<PROXY_REGISTRY>` — registry address: `<HOSTNAME>[:PORT]`.

The `InitConfiguration` resource provides two more parameters for non-standard third-party registry configurations:
- `registryCA` - root CA certificate to validate the third-party registry's HTTPS certificate (if self-signed certificates are used).
- `registryScheme` - registry scheme (`http` or `https`). The default value is `https`.

### Installing
Use the `dhctl`'s `--dont-use-public-control-plane-images` key to instruct Deckhouse to use `control-plane` images from the third-party registry instead of the public one (`k8s.gcr.io`).

### Tips for configuring the third-party registry

**Note** that Deckhouse only supports Bearer authentication for registries.

#### Nexus
The following parameters must be set if the [Nexus](https://github.com/sonatype/nexus-public) repository manager is used:

* Enable `Docker Bearer Token Realm`
  ![](../images/registry/nexus/Nexus1.png)

* Enable anonymous registry access (otherwise, Bearer authentication [won't work](https://help.sonatype.com/repomanager3/system-configuration/user-authentication#UserAuthentication-security-realms))
  ![](../images/registry/nexus/Nexus2.png)

* Set the `Maximum metadata age` to 0 (otherwise, the automatic update of Deckhouse will fail due to caching)
  ![](../images/registry/nexus/Nexus3.png)

#### Harbor
You need to use the Proxy Cache feature of a [Harbor](https://github.com/goharbor/harbor).

* Create a Registry 
  - `Administration -> Registries -> New Endpoint`
  - `Provider`: `Docker Registry`
  - `Name` — specify any of your choice.
  - `Endpoint URL`: `https://registry.deckhouse.io`
  - Specify the `Access ID` and `Access Secret` if you use Deckhouse Enterprise Edition; otherwise, leave them blank.  
![](images/registry/harbor/harbor1.png)

* Create a new Project
  - `Projects -> New Project`
  - `Project Name` will be used in the URL. You can choose any name, for example, `d8s`.
  - `Access Level`: `Public`
  - `Proxy Cache` — enable and choose the Registry, created in the previous step.
![](images/registry/harbor/harbor2.png)

Thus, Deckhouse images will be available at `https://your-harbor.com/d8s/deckhouse/{d8s-edition}:{d8s-version}`.

## How do I switch a running Deckhouse cluster to use a third-party registry?

* Edit the `d8-system/deckhouse-registry` secret (note that all parameters are BASE64-encoded):
  * Insert third-party registry credentials into `.dockerconfigjson`.
  * Replace `address` with the third-party registry's host address (e.g., `registry.example.com`).
  * Change `path` to point to a repo in the third-party registry (e.g., `/deckhouse/fe`).
  * If necessary, change `scheme` to `http` (if the third-party registry uses HTTP scheme).
  * If necessary, change or add the `ca` field with the root CA certificate that validates the third-party registry's https certificate (if the third-party registry uses self-signed certificates).
* Update the `image` field in the `d8-system/deckhouse` deployment to contain the address of the Deckhouse image in the third-party-registry.
* Wait for the Deckhouse Pod to become Ready.
* Wait for bashible to apply the new settings on the master node. The bashible log on the master node (`journalctl -u bashible`) should contain the message `Configuration is in sync, nothing to do`.
* Remove `releaseChannel` setting from configmap `d8-system/deckhouse`
