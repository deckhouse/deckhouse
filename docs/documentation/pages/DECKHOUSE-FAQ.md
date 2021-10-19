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

  - As a result, Deckhouse will switch to a more stable release channel with a digest equal to the current one.

* Suppose a less stable release channel is specified than the channel that corresponds to the current tag of the Deckhouse Docker image. In that case, the script compares digests corresponding to the Docker images for the current release channel and the next, less stable one. For example, if you need to switch to the `Alpha` channel from the `EarlyAccess` channel, the script will compare the  `EarlyAccess` and `Beta` channels:

  - If digests are not equal, Deckhouse switch to the next release channel (`Beta` in our case). Such an approach ensures that some crucial migrations are performed during Deckhouse upgrades.

  - If the digests are equal, the script checks the next less stable release channel (`Alpha` in our case).

  - When the script reaches the desired release channel (`Alpha` in our example), Deckhouse will switch to it regardless of the results of the digest comparison.

Since the stabilization script runs continuously, Deckhouse will eventually end up in a state where the tag of its Docker image corresponds to the release channel set.

## How do I run Deckhouse on a particular node?
Set the `nodeSelector` [parameter](modules/020-deckhouse/configuration.html) of the `deckhouse` module and don't set `tolerations`. The necessary values for the `tolerations` parameter will be set automatically.

You should also avoid using **CloudEphemeral** nodes. Otherwise, a situation may occur when the target node is not in the cluster and node ordering for some reason is impossible.

Here is an example of the module configuration:
```yaml
deckhouse: |
  nodeSelector:
    node-role.deckhouse.io/deckhouse: ""
```
## How to setup deckhouse from third-party registry?

During bootstrap, Deckhouse can be configured to work from a third-party registry (for example, a proxying registry inside a closed circuits).

### Bootstrap
#### Config
Setting up parameters in `InitConfiguration`:
- `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/<DECKHOUSE_REVISION>`. Deckhouse image address with edition (ce/ee/fe) in third-party registry.
Example: `registry.deckhouse.io/deckhouse/ce`.
- `registryDockerCfg: <BASE64>`. Trird-party registry auth credentials in BASE64.

If anonymous access is enabled, `registryDockerCfg` must be:
```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```
`registryDockerCfg` must be BASE64 encoded.

If anonymous access don't enabled, `registryDockerCfg` must be:
```json
{"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
```

`<AUTH_BASE64>` — BASE64 encoded auth string `<PROXY_USERNAME>:<PROXY_PASSWORD>`.

registryDockerCfg` must be BASE64 encoded.

* `<PROXY_USERNAME>` — auth username for `<PROXY_REGISTRY>`.
* `<PROXY_PASSWORD>` — auth password for `<PROXY_REGISTRY>`.
* `<PROXY_REGISTRY>` — registry address`<HOSTNAME>[:PORT]`.

To configure non-standard configurations of third-party registry, two more parameters are provided in the `InitConfiguration` resource:
- `registryCA` - root CA certificate, which is used to validate third-party registry https certificate (if third-party registry uses self-signed certificates).
- `registryScheme` - registry scheme (`HTTP` or `HTTPS`). Default - `HTTPS`.

#### Bootstrap
Use `dhctl` key `--dont-use-public-control-plane-images` to tell Deckhouse to use `control-plane` images from third-party registry instead of public (`k8s.gcr.io`).

#### Third-party registry setup tips

**Attention:** Deckhouse supports only Bearer token registry auth.

##### Nexus
If [Nexus](https://github.com/sonatype/nexus-public) registry-proxy is used, some parameters must be set:

* Enable `Docker Bearer Token Realm`
  ![](../images/registry/nexus/Nexus1.png)

* Enable anonymous registry access (without anonymous access Bearer Token auth [don't work](https://help.sonatype.com/repomanager3/system-configuration/user-authentication#UserAuthentication-security-realms))
  ![](../images/registry/nexus/Nexus2.png)

* Set `Maximum metadata age` to 0 (Deckhouse autoupdate don't work properly if metadata caching enabled)
  ![](../images/registry/nexus/Nexus3.png)

## How to switch running Deckhouse cluster to work with third-party registry?

* Change secret `d8-system/deckhouse-registry.`
  * Change `.dockerconfigjson` to third-party registry credentials.
  * Change `address` to third-party registry host address (for example, `registry.example.com`).
  * Change `path` to repo path in third-party registry (for example, `/deckhouse/fe`).
  * If necessary, change `scheme` to `http` (if third-party registry uses http scheme).
  * If necessary, change or add `ca` field with root CA certificate, which is used to validate third-party registry https certificate (if third-party registry uses self-signed certificates).
* Restart Deckhouse pod.
* Wait while Deckhouse converge is finished.
* Wait while bashible converge on master nodes is finished.
* Update `image` field in `d8-system/deckhouse` deployment to Deckhouse image address in third-party-registry.
