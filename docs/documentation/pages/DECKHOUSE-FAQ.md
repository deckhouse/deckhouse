---
title: FAQ
permalink: en/deckhouse-faq.html
---

## How do I find out all Deckhouse parameters?

All the Deskhouse settings (including module parameters) are stored in cluster scoped ModuleConfig resources and custom resources. The `global` ModuleConfig contains global Deckhouse settings. Read more in [the documentation](./).

To get list of all ModuleConfigs:

```shell
kubectl get mc
```

To view global Deckhouse settings:

```shell
kubectl get mc global -o yaml
```

## How do I find the documentation for the version installed?

> Documentation in the cluster is available when the [deckhouse-web](modules/810-deckhouse-web/) module is enabled (it is enabled by default except the `Minimal` [bundle](modules/002-deckhouse/configuration.html#parameters-bundle)).

The documentation for the Deckhouse version running in the cluster is available at `deckhouse.<cluster_domain>`, where `<cluster_domain>` is the DNS name that matches the template defined in the `global.modules.publicDomainTemplate` parameter.

## How do I set the desired release channel?

Change (set) the `releaseChannel` parameter in the `deckhouse` module [configuration](modules/002-deckhouse/configuration.html#parameters-releasechannel) to automatically switch to another release channel.

It will activate the mechanism of [automatic stabilization of the release channel](#how-does-automatic-deckhouse-update-work).

Here is an example of the `deckhouse` module configuration with the `Stable` release channel:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
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
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
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

> **Note!** Deckhouse only supports Bearer authentication for registries.

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

* If you want to install Deckhouse with automatic updates disabled:
  * Use the tag of the installer image of the corresponding version. For example, use the image `your.private.registry.com/deckhouse/install:v1.44.3`, if you want to install release `v1.44.3`.
  * Set the corresponding version number in the [deckhouse.devBranch](installing/configuration.html#initconfiguration-deckhouse-devbranch) parameter of the `InitConfiguration` resource.
  * **Do not** set the [deckhouse.releaseChannel](installing/configuration.html#initconfiguration-deckhouse-releasechannel) parameter of the `InitConfiguration` resource.
* If you want to disable automatic updates for an already installed Deckhouse (including patch release updates), then delete the [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel) parameter from the `deckhouse` module configuration.

## Using a proxy server

### Setting up a proxy server

* Prepare the VM for setting up the proxy. The machine must be accessible to the nodes that will use it as a proxy and be connected to the Internet.
* Install Squid on the server (here and further examples for Ubuntu):

  ```shell
  apt-get install squid
  ```

* Create a config file:

  ```shell
  cat <<EOF > /etc/squid/squid.conf
  auth_param basic program /usr/lib/squid3/basic_ncsa_auth /etc/squid/passwords
  auth_param basic realm proxy
  acl authenticated proxy_auth REQUIRED
  http_access allow authenticated
  
  # Choose the port you want. Below we set it to default 3128.
  http_port 3128
  ```

* Create a user for proxy-server authentication:

  Example for the user `test` with the password `test` (be sure to change):
  
  ```shell
  echo "test:$(openssl passwd -crypt test)" >> /etc/squid/passwords
  ```

* Start squid and enable the system to start it up automatically:

  ```shell
  systemctl restart squid
  systemctl enable squid
  ```

### Configuring proxy usage in Deckhouse

Use the [proxy](installing/configuration.html#clusterconfiguration-proxy) parameter of the `ClusterConfiguration` resource to configure proxy usage.

An example:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: OpenStack
  prefix: main
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.23"
cri: "Containerd"
clusterDomain: "cluster.local"
proxy:
  httpProxy: "http://user:password@proxy.company.my:3128"
  httpsProxy: "https://user:password@proxy.company.my:8443"
```

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
* If you want to disable Deckhouse automatic updates, remove the `releaseChannel` parameter from the `deckhouse` module configuration.
* Check if there are Pods with original registry in cluster (if there are — restart them):

  ```shell
  kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io")))) 
    | .metadata.namespace + "\t" + .metadata.name' -r
  ```

## How to switch Deckhouse EE to CE?

> The instruction implies using the public address of the container registry: `registry.deckhouse.io`. If you use a different container registry address, change the commands or use [the instruction](#how-do-i-configure-deckhouse-to-use-a-third-party-registry) for switching Deckhouse to using a third-party registry.

To switch Deckhouse Enterprise Edition to Community Edition, follow these steps:

1. Make sure that the modules used in the cluster [are supported in Deckhouse CE](revision-comparison.html). Disable modules that are not supported in Deckhouse CE.

   > Please note that Deckhouse CE does not support cloud clusters on OpenStack and VMware vSphere.
1. Run the following command:

   ```shell
   bash -c "$(curl -Ls https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/change-registry.sh)" -- --registry-url https://registry.deckhouse.io/deckhouse/ce && \
   kubectl -n d8-system set image deployment/deckhouse deckhouse=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.spec.template.spec.containers[?(@.name=="deckhouse")].image}' | awk -F: '{print "registry.deckhouse.io/deckhouse/ce:" $2}')
   ```

1. Wait for the Deckhouse Pod to become `Ready`:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

1. Restart Deckhouse Pod if it will be in `ImagePullBackoff` state:

   ```shell
   kubectl -n d8-system delete po -l app=deckhouse
   ```

1. Wait for Deckhouse to restart and to complete all tasks in the queue:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   ```

   Example of output when there are still jobs in the queue (`length 38`):

   ```console
   # kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   Queue 'main': length 38, status: 'run first task'
   ```

   Example of output when the queue is empty (`length 0`):

   ```console
   # kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   Queue 'main': length 0, status: 'waiting for task 0s'
   ```

1. On the master node, check the application of the new settings.

   The message `Configuration is in sync, nothing to do` should appear in the `bashible` systemd service log on the master node.

   An example::

   ```console
   # journalctl -u bashible -n 5
   Jan 12 12:38:20 demo-master-0 bashible.sh[868379]: Configuration is in sync, nothing to do.
   Jan 12 12:38:20 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   Jan 12 12:39:18 demo-master-0 systemd[1]: Started Bashible service.
   Jan 12 12:39:19 demo-master-0 bashible.sh[869714]: Configuration is in sync, nothing to do.
   Jan 12 12:39:19 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Check if there are any Pods left in the cluster with the Deckhouse EE registry address:

   ```shell
   kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io/deckhouse/ee")))) 
     | .metadata.namespace + "\t" + .metadata.name' -r | sort | uniq
   ```

   Sometimes, some static Pods may remain running (for example, `kubernetes-api-proxy-*`). This is due to the fact that kubelet does not restart the Pod despite changing the corresponding manifest, because the image used is the same for the Deckhouse CE and EE editions. To make sure that the corresponding manifests have also been changed, run the following command on any master node:

   ```shell
   grep -ri 'deckhouse.io/deckhouse/ee' /etc/kubernetes | grep -v backup
   ```

   The output of the command should be empty.

## How to switch Deckhouse CE to EE?

You will need a valid license key (you can [request a trial license key](https://deckhouse.io/products/enterprise_edition.html) if necessary).

> The instruction implies using the public address of the container registry: `registry.deckhouse.io`. If you use a different container registry address, change the commands or use [the instruction](#how-do-i-configure-deckhouse-to-use-a-third-party-registry) for switching Deckhouse to using a third-party registry.

To switch Deckhouse Community Edition to Enterprise Edition, follow these steps:

1. Run the following command:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   bash -c "$(curl -Ls https://raw.githubusercontent.com/deckhouse/deckhouse/main/tools/change-registry.sh)" -- --user license-token --password $LICENSE_TOKEN --registry-url https://registry.deckhouse.io/deckhouse/ee && \
   kubectl -n d8-system set image deployment/deckhouse deckhouse=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.spec.template.spec.containers[?(@.name=="deckhouse")].image}' | awk -F: '{print "registry.deckhouse.io/deckhouse/ee:" $2}') 
   ```

1. Wait for the Deckhouse Pod to become `Ready`:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

1. Restart Deckhouse Pod if it will be in `ImagePullBackoff` state:

   ```shell
   kubectl -n d8-system delete po -l app=deckhouse
   ```

1. Wait for Deckhouse to restart and to complete all tasks in the queue:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   ```

   Example of output when there are still jobs in the queue (`length 38`):

   ```console
   # kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   Queue 'main': length 38, status: 'run first task'
   ```

   Example of output when the queue is empty (`length 0`):

   ```console
   # kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller queue main | grep status:
   Queue 'main': length 0, status: 'waiting for task 0s'
   ```

1. On the master node, check the application of the new settings.

   The message `Configuration is in sync, nothing to do` should appear in the `bashible` systemd service log on the master node.

   An example:

   ```console
   # journalctl -u bashible -n 5
   Jan 12 12:38:20 demo-master-0 bashible.sh[868379]: Configuration is in sync, nothing to do.
   Jan 12 12:38:20 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   Jan 12 12:39:18 demo-master-0 systemd[1]: Started Bashible service.
   Jan 12 12:39:19 demo-master-0 bashible.sh[869714]: Configuration is in sync, nothing to do.
   Jan 12 12:39:19 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Check if there are any Pods left in the cluster with the Deckhouse CE registry address:

   ```shell
   kubectl get pods -A -o json | jq '.items[] | select(.spec.containers[] | select((.image | contains("deckhouse.io/deckhouse/ce")))) 
     | .metadata.namespace + "\t" + .metadata.name' -r | sort | uniq
   ```

   Sometimes, some static Pods may remain running (for example, `kubernetes-api-proxy-*`). This is due to the fact that kubelet does not restart the Pod despite changing the corresponding manifest, because the image used is the same for the Deckhouse CE and EE editions. To make sure that the corresponding manifests have also been changed, run the following command on any master node:

   ```shell
   grep -ri 'deckhouse.io/deckhouse/ce' /etc/kubernetes | grep -v backup
   ```

   The output of the command should be empty.

## How do I change the configuration of a cluster?

The general cluster parameters are stored in the [ClusterConfiguration](installing/configuration.html#clusterconfiguration) structure.

To change the general cluster parameters, run the command:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit cluster-configuration
```

After saving the changes, Deckhouse will bring the cluster configuration to the state according to the changed configuration. Depending on the size of the cluster, this may take some time.

## How do I change the configuration of a cloud provider in a cluster?

Cloud provider setting of a cloud of hybrid cluster are stored in the `<PROVIDER_NAME>ClusterConfiguration` structure, where `<PROVIDER_NAME>` — name/code of the cloud provider. E.g., for an OpenStack provider, the structure will be called [OpenStackClusterConfiguration]({% if site.mode == 'local' and site.d8Revision == 'CE' %}{{ site.urls[page.lang] }}/documentation/v1/{% endif %}modules/030-cloud-provider-openstack/cluster_configuration.html).

Regardless of the cloud provider used, its settings can be changed using the command:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

## How do I change the configuration of a static cluster?

Settings of a static cluster are stored in the [StaticClusterConfiguration](installing/configuration.html#staticclusterconfiguration) structure.

To change the settings of a static cluster, run the command:

```shell
kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit static-cluster-configuration
```

## How do I upgrade the Kubernetes version in a cluster?

To upgrade the Kubernetes version in a cluster change the [kubernetesVersion](installing/configuration.html#clusterconfiguration-kubernetesversion) parameter in the [ClusterConfiguration](installing/configuration.html#clusterconfiguration) structure by making the following steps:
1. Run the command:

   ```shell
   kubectl -n d8-system exec -ti deploy/deckhouse -- deckhouse-controller edit cluster-configuration
   ```

1. Change the `kubernetesVersion` field.
1. Save the changes. Cluster nodes will start updating sequentially.
1. Wait for the update to finish. You can track the progress of the update using the `kubectl get no` command. The update is completed when the new version appears in the command's output for each cluster node in the `VERSION` column.
