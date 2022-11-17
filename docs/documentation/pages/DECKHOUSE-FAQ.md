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

> Documentation in the cluster is available when the [deckhouse-web](modules/810-deckhouse-web/) module is enabled (it is enabled by default except the `Minimal` [bundle](modules/002-deckhouse/configuration.html#parameters-bundle)).

The documentation for the Deckhouse version running in the cluster is available at `deckhouse.<cluster_domain>`, where `<cluster_domain>` is the DNS name that matches the template defined in the `global.modules.publicDomainTemplate` parameter.

## How do I set the desired release channel?

Change (set) the `releaseChannel` parameter in the `deckhouse` module [configuration](modules/002-deckhouse/configuration.html#parameters-releasechannel) to automatically switch to another release channel.

It will activate the mechanism of [automatic stabilization of the release channel](#how-does-automatic-deckhouse-update-work).

Here is an example of the module configuration:

```yaml
deckhouse: |
  releaseChannel: Stable
```

## How do I disable automatic updates?

To completely disable the Deckhouse update mechanism, remove the `releaseChannel` parameter in the `deckhouse' module [configuration](modules/002-deckhouse/configuration.html#parameters-releasechannel).

In this case, Deckhouse does not check for updates and even doesn't apply patch releases.

> It is highly not recommended to disable automatic updates! It will block updates to patch releases that may contain critical vulnerabilities and bugs fixes.

## How does automatic Deckhouse update work?

Every minute Deckhouse checks a new release appeared in the release channel specified by the `releaseChannel` parameter.

When a new release appears on the release channel, Deckhouse downloads it and creates CustomResource `DeckhouseRelease`.

After creating a `DeckhouseRelease` CR in a cluster, Deckhouse updates the `deckhouse` Deployment and sets the image tag to a specified release tag according to [selected](modules/002-deckhouse/configuration.html#parameters-update) update mode and update windows (automatic at any time by default).

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
  * Then Deckhouse performs the update according to the [update parameters](modules/002-deckhouse/configuration.html#parameters-update).

## How do I run Deckhouse on a particular node?

Set the `nodeSelector` [parameter](modules/002-deckhouse/configuration.html) of the `deckhouse` module and avoid setting `tolerations`. The necessary values will be assigned to the `tolerations` parameter automatically.

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
* `registryScheme` - registry scheme (`HTTP` or `HTTPS`). The default value is `HTTPS`.

### Tips for configuring the third-party registry

**Note** that Deckhouse only supports Bearer authentication for registries.

#### Nexus

##### Requirements

The following requirements must be met if the [Nexus](https://github.com/sonatype/nexus-public) repository manager is used:

* `Docker Bearer Token Realm` must be enabled.
* Docker proxy repository must be pre-created.
* `Allow anonymous docker pull` must be enabled.
* Access control must be configured as follows:
  * The Nexus role with the `nx-repository-view-docker-<repo>-browse` and `nx-repository-view-docker-<repo>-read` permissions must be created.
  * The Nexus user must be created with the above role granted.
* `Maximum metadata age` for the created repository must be set to 0.

##### Configuration

* Enable `Docker Bearer Token Realm`:
  ![Enable `Docker Bearer Token Realm`](images/registry/nexus/nexus-realm.png)

* Create a docker proxy repository pointing to the [Deckhouse registry](https://registry.deckhouse.io/):
  ![Create docker proxy repository](images/registry/nexus/nexus-repository.png)
  
* Fill in the fields on the Create page  as follows:
  * `Name` must contain the name of the repository you created earlier, e.g., `d8-proxy`.
  * `Repository Connectors / HTTP` or `Repository Connectors / HTTPS` must contain a dedicated port for the created repository, e.g., `8123` or other.
  * `Allow anonymous docker pull` must be enabled for the Bearer token authentication to [work](https://help.sonatype.com/repomanager3/system-configuration/user-authentication#UserAuthentication-security-realms). Note, however, that anonymous access [won't work](https://help.sonatype.com/repomanager3/nexus-repository-administration/formats/docker-registry/docker-authentication#DockerAuthentication-UnauthenticatedAccesstoDockerRepositories) unless it is explicitly enabled in Settings -> Security -> Anonymous Access and the `anonymous` user has been granted access rights to the created repository.
  * `Remote storage` must be set to `https://registry.deckhouse.io/`.
  * You can disable `Auto blocking enabled` and `Not found cache enabled` for debugging purposes, otherwise they must be enabled.
  * `Maximum Metadata Age` must be set to 0.
  * `Authentication` must be enabled if you plan to use Deckhouse Enterprise Edition and the related fields must be set as follows:
    * `Authentication Type` must be set to `Username`.
    * `Username` must be set to `license-token`.
    * `Password` must contain your license key for Deckhouse Enterprise Edition.

  ![Repository settings example 1](images/registry/nexus/nexus-repo-example-1.png)
  ![Repository settings example 2](images/registry/nexus/nexus-repo-example-2.png)
  ![Repository settings example 3](images/registry/nexus/nexus-repo-example-3.png)

* Configure Nexus access control to allow Nexus access to the created repository:
  * Create a Nexus role with the `nx-repository-view-docker-<repo>-browse` and `nx-repository-view-docker-<repo>-read` permissions.
  ![Create a Nexus role](images/registry/nexus/nexus-role.png)
  * Create a Nexus user with the role above granted.
  ![Create a Nexus user](images/registry/nexus/nexus-user.png)

#### Harbor

You need to use the Proxy Cache feature of a [Harbor](https://github.com/goharbor/harbor).

* Create a Registry:
  * `Administration -> Registries -> New Endpoint`.
  * `Provider`: `Docker Registry`.
  * `Name` — specify any of your choice.
  * `Endpoint URL`: `https://registry.deckhouse.io`.
  * Specify the `Access ID` and `Access Secret` if you use Deckhouse Enterprise Edition; otherwise, leave them blank.

![Create a Registry](images/registry/harbor/harbor1.png)

* Create a new Project:
  * `Projects -> New Project`.
  * `Project Name` will be used in the URL. You can choose any name, for example, `d8s`.
  * `Access Level`: `Public`.
  * `Proxy Cache` — enable and choose the Registry, created in the previous step.

![Create a new Project](images/registry/harbor/harbor2.png)

Thus, Deckhouse images will be available at `https://your-harbor.com/d8s/deckhouse/{d8s-edition}:{d8s-version}`.

### Manually upload images to an air-gapped registry

Download script on a host that have access to `registry.deckhouse.io`. `Docker`, `crane` and `jq` must be installed on this host.

```shell
curl -fsSL -o d8-pull.sh https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/release/d8-pull.sh
chmod 700 d8-pull.sh
```

Example of pulling images:

```shell
./d8-pull.sh --license YOUR_DECKHOUSE_LICENSE_KEY --output-dir /your/output-dir/
```

Upload the folder from the previous step to a host with access to an air-gapped registry. `Crane` must be installed on this host.
Download script on a host that has access to an air-gapped registry.

```shell
curl -fsSL -o d8-push.sh https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/release/d8-push.sh
chmod 700 d8-push.sh
```

Example of pushing images:

```shell
./d8-push.sh --source-dir /your/source-dir/ --path your.private.registry.com/deckhouse --username YOUR_USERNAME --password YOUR_PASSWORD
```

## How to bootstrap a cluster and run Deckhouse without the usage of release channels?

This case is only valid if you don't have release channel images in your air-gapped registry.

* If you want to bootstrap a cluster, you have to use the exact tag of a Docker image to install the Deckhouse Platform.
For example, if you want to install release v1.32.13, you have to use image `your.private.registry.com/deckhouse/install:v1.32.13`. And you have to use `devBranch: v1.32.13` instead of `releaseChannel: XXX` in `config.yml`.
* If you already have a cluster up and running, you have to remove `releaseChannel` setting from the ConfigMap `d8-system/deckhouse` and set the appropriate `image` parameter for `d8-system/deckhouse` Deployment. Further updates have to be done by a manual update of the `image` parameter of the `d8-system/deckhouse` Deployment.
For example, you have to set `image` to `your.private.registry.com/deckhouse:v1.32.13` for release v1.32.13.

## How do I switch a running Deckhouse cluster to use a third-party registry?

To switch the Deckhouse cluster to using a third-party registry, follow these steps:

* Update the `image` field in the `d8-system/deckhouse` deployment to contain the address of the Deckhouse image in the third-party-registry;
* Download script to the master node and run it with parameters for a new registry.
  * Example:

  ```shell
  curl -fsSL -o change-registry.sh https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/change-registry.sh
  chmod 700 change-registry.sh
  ./change-registry.sh --registry-url https://my-new-registry/deckhouse --user my-user --password my-password
  ```
  
  * If the registry uses a self-signed certificate, put the root CA certificate that validates the registry's HTTPS certificate to file `ca.crt` near the script and add the `--ca-file ca.crt` option to the script.
* Wait for the Deckhouse Pod to become `Ready`. Restart Deckhouse Pod if it will be in `ImagePullBackoff` state.
* Wait for bashible to apply the new settings on the master node. The bashible log on the master node (`journalctl -u bashible`) should contain the message `Configuration is in sync, nothing to do`.
* If you want to disable Deckhouse automatic updates, remove the `releaseChannel` parameter from the `d8-system/deckhouse` ConfigMap.
* Check if there are Pods with original registry in cluster (if there are — restart them):

  ```shell
  kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io")))) | .metadata.namespace + "\t" + .metadata.name' -r
  ```

## How do I change the configuration of a cluster?

The general cluster parameters are stored in the `ClusterConfiguration` structure. It contains parameters such as:

- cluster domain: `clusterDomain`;
- CRI used in the cluster: `defaultCRI`;
- Kubernetes control plane version: `kubernetesVersion`;
- cluster type (Static, Cloud): `clusterType`;
- address space of the cluster's Pods: `podSubnetCIDR`;
- address space of the cluster's services: `serviceSubnetCIDR` etc.

To change the general cluster parameters, run the command:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit cluster-configuration
```

## How do I change the configuration of a cloud provider in a cluster?

Cloud provider setting of a cloud of hybrid cluster are stored in the `<PROVIDER_NAME>ClusterConfiguration` structure, where `<PROVIDER_NAME>` — name/code of the cloud provider. E.g., for an OpenStack provider, the structure will be called `OpenStackClusterConfiguration`.

Regardless of the cloud provider used, its settings can be changed using the command:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

## How do I change the configuration of a static cluster?

Settings of a static cluster are stored in the `StaticClusterConfiguration` structure.

To change the settings of a static cluster, run the command:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit static-cluster-configuration
```

## How do I upgrade the Kubernetes version in a cluster?

To upgrade the Kubernetes version in a cluster change the `kubernetesVersion` parameter in the `ClusterConfiguration` structure by making the following steps:
1. Run the command:

   ```shell
   kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit cluster-configuration
   ```

1. Change the `kubernetesVersion` field.
1. Save the changes.
