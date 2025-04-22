---
title: FAQ
permalink: en/deckhouse-faq.html
---

## How do I find out all Deckhouse parameters?

Deckhouse is configured using global settings, module settings, and various custom resources. Read more in the [documentation](./).

1. Display global Deckhouse settings:

   ```shell
   kubectl get mc global -o yaml
   ```

1. List the status of all modules (available for Deckhouse version 1.47+):

   ```shell
   kubectl get modules
   ```

1. Display the settings of the `user-authn` module configuration:

   ```shell
   kubectl get moduleconfigs user-authn -o yaml
   ```

## How do I find the documentation for the version installed?

The documentation for the Deckhouse version running in the cluster is available at `documentation.<cluster_domain>`, where `<cluster_domain>` is the DNS name that matches the template defined in the [modules.publicDomainTemplate](deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) parameter.

{% alert level="warning" %}
Documentation is available when the [documentation](modules/documentation/) module is enabled. It is enabled by default except the `Minimal` [bundle](modules/deckhouse/configuration.html#parameters-bundle).
{% endalert %}

## Deckhouse update

### How to find out in which mode the cluster is being updated?

You can view the cluster update mode in the [configuration](modules/deckhouse/configuration.html) of the `deckhouse` module. To do this, run the following command:

```shell
kubectl get mc deckhouse -oyaml
```

Example of the output:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  creationTimestamp: "2022-12-14T11:13:03Z"
  generation: 1
  name: deckhouse
  resourceVersion: "3258626079"
  uid: c64a2532-af0d-496b-b4b7-eafb5d9a56ee
spec:
  settings:
    releaseChannel: Stable
    update:
      windows:
      - days:
        - Mon
        from: "19:00"
        to: "20:00"
  version: 1
status:
  state: Enabled
  status: ""
  type: Embedded
  version: "1"
```

There are three possible update modes:

* **Automatic + update windows are not set.** The cluster will be updated after the new version appears on the corresponding [release channel](deckhouse-release-channels.html).
* **Automatic + update windows are set.** The cluster will be updated in the nearest available window after the new version appears on the release channel.
* **Manual.** [Manual action](modules/deckhouse/usage.html#manual-update-confirmation) is required to apply the update.

### How do I set the desired release channel?

Change (set) the [releaseChannel](modules/deckhouse/configuration.html#parameters-releasechannel) parameter in the `deckhouse` module [configuration](modules/deckhouse/configuration.html) to automatically switch to another release channel.

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

### How do I disable automatic updates?

To completely disable the Deckhouse update mechanism, remove the [releaseChannel](modules/deckhouse/configuration.html#parameters-releasechannel) parameter in the `deckhouse` module [configuration](modules/deckhouse/configuration.html).

In this case, Deckhouse does not check for updates and doesn't apply patch releases.

{% alert level="danger" %}
It is highly not recommended to disable automatic updates! It will block updates to patch releases that may contain critical vulnerabilities and bugs fixes.
{% endalert %}

### How do I apply an update without having to wait for the update window, canary-release and manual update mode?

To apply an update immediately, set the `release.deckhouse.io/apply-now : "true"` annotation on the [DeckhouseRelease](cr.html#deckhouserelease) resource.

{% alert level="info" %}
**Caution!** In this case, the update windows, settings [canary-release](cr.html#deckhouserelease-v1alpha1-spec-applyafter) and [manual cluster update mode](modules/deckhouse/configuration.html#parameters-update-disruptionapprovalmode) will be ignored. The update will be applied immediately after the annotation is installed.
{% endalert %}

An example of a command to set the annotation to skip the update windows for version `v1.56.2`:

```shell
kubectl annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
```

An example of a resource with the update window skipping annotation in place:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  annotations:
    release.deckhouse.io/apply-now: "true"
...
```

### How to understand what changes the update contains and how it will affect the cluster?

You can find all the information about Deckhouse versions in the list of [Deckhouse releases](https://github.com/deckhouse/deckhouse/releases).

Summary information about important changes, component version updates, and which components in the cluster will be restarted during the update process can be found in the description of the zero patch version of the release. For example, [v1.46.0](https://github.com/deckhouse/deckhouse/releases/tag/v1.46.0) for the v1.46 Deckhouse release.

A detailed list of changes can be found in the Changelog, which is referenced in each [release](https://github.com/deckhouse/deckhouse/releases).

### How do I understand that the cluster is being updated?

During the update:

- The `DeckhouseUpdating` alert is displayed.
- The `deckhouse` Pod is not the `Ready` status. If the Pod does not go to the `Ready` status for a long time, then this may indicate that there are problems in the work of Deckhouse. Diagnosis is necessary.

### How do I know that the update was successful?

If the `DeckhouseUpdating` alert is resolved, then the update is complete.

You can also check the status of Deckhouse [releases](cr.html#deckhouserelease) by running the following command:

```bash
kubectl get deckhouserelease
```

Example output:

```console
NAME       PHASE        TRANSITIONTIME   MESSAGE
v1.46.8    Superseded   13d
v1.46.9    Superseded   11d
v1.47.0    Superseded   4h12m
v1.47.1    Deployed     4h12m
```

The `Deployed` status of the corresponding version indicates that the switch to the corresponding version was performed (but this does not mean that it ended successfully).

Check the status of the Deckhouse Pod:

```shell
kubectl -n d8-system get pods -l app=deckhouse
```

Example output:

```console
NAME                   READY  STATUS   RESTARTS  AGE
deckhouse-7844b47bcd-qtbx9  1/1   Running  0       1d
```

* If the status of the Pod is `Running`, and `1/1` indicated in the READY column, the update was completed successfully.
* If the status of the Pod is `Running`, and `0/1` indicated in the READY column, the update is not over yet. If this goes on for more than 20-30 minutes, then this may indicate that there are problems in the work of Deckhouse. Diagnosis is necessary.
* If the status of the Pod is not `Running`, then this may indicate that there are problems in the work of Deckhouse. Diagnosis is necessary.

{% alert level="info" %}
Possible options for action if something went wrong:

1. Check Deckhouse logs using the following command:

   ```shell
   kubectl -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
   ```

1. [Collect debugging information](modules/deckhouse/faq.html#how-to-collect-debug-info) and contact technical support.
1. Ask for help from the [community](https://deckhouse.io/community/about.html).
{% endalert %}

### How do I know that a new version is available for the cluster?

As soon as a new version of Deckhouse appears on the release channel installed in the cluster:

- The alert `DeckhouseReleaseIsWaitingManualApproval` fires, if the cluster uses manual update mode (the [update.mode](modules/deckhouse/configuration.html#parameters-update-mode) parameter is set to `Manual`).
- There is a new custom resource [DeckhouseRelease](cr.html#deckhouserelease). Use the `kubectl get deckhousereleases` command, to view the list of releases. If the `DeckhouseRelease` is in the `Pending` state, the specified version has not yet been installed. Possible reasons why `DeckhouseRelease` may be in `Pending`:
  - Manual update mode is set (the [update.mode](modules/deckhouse/configuration.html#parameters-update-mode) parameter is set to `Manual`).
  - The automatic update mode is set, and the [update windows](modules/deckhouse/usage.html#update-windows-configuration) are configured, the interval of which has not yet come.
  - The automatic update mode is set, update windows are not configured, but the installation of the version has been postponed for a random time due to the mechanism of reducing the load on the repository of container images. There will be a corresponding message in the `status.message` field of the `DeckhouseRelease` resource.
  - The [update.notification.minimalNotificationTime](modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) parameter is set, and the specified time has not passed yet.

### How do I get information about the upcoming update in advance?

You can get information in advance about updating minor versions of Deckhouse on the release channel in the following ways:

- Configure manual [update mode](modules/deckhouse/configuration.html#parameters-update-mode). In this case, when a new version appears on the release channel, the alert `DeckhouseReleaseIsWaitingManualApproval` will be displayed and a new custom resource [DeckhouseRelease](cr.html#deckhouserelease) will be applied in the cluster.
- Configure automatic [update mode](modules/deckhouse/configuration.html#parameters-update-mode) and specify the minimum time in the [minimalNotificationTime](modules/deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) parameter for which the update will be postponed. In this case, when a new version appears on the release channel, a new custom resource [DeckhouseRelease](cr.html#deckhouserelease) will appear in the cluster. And if you specify a URL in the [update.notification.webhook](modules/deckhouse/configuration.html#parameters-update-notification-webhook) parameter, then the webhook will be called additionally.

### How do I find out which version of Deckhouse is on which release channel?

Information about which version of Deckhouse is on which release channel can be obtained at <https://releases.deckhouse.io>.

### How does automatic Deckhouse update work?

Every minute Deckhouse checks a new release appeared in the release channel specified by the [releaseChannel](modules/deckhouse/configuration.html#parameters-releasechannel) parameter.

When a new release appears on the release channel, Deckhouse downloads it and creates CustomResource [DeckhouseRelease](cr.html#deckhouserelease).

After creating a `DeckhouseRelease` custom resource in a cluster, Deckhouse updates the `deckhouse` Deployment and sets the image tag to a specified release tag according to [selected](modules/deckhouse/configuration.html#parameters-update) update mode and update windows (automatic at any time by default).

To get list and status of all releases use the following command:

```shell
kubectl get deckhousereleases
```

{% alert %}
Patch releases (e.g., an update from version `1.30.1` to version `1.30.2`) ignore update windows settings and apply as soon as they are available.
{% endalert %}

### What happens when the release channel changes?

* When switching to a **more stable** release channel (e.g., from `Alpha` to `EarlyAccess`), Deckhouse downloads release data from the release channel (the `EarlyAccess` release channel in the example) and compares it with the existing `DeckhouseReleases`:
  * Deckhouse deletes *later* releases (by semver) that have not yet been applied (with the `Pending` status).
  * if *the latest* releases have been already Deployed, then Deckhouse will hold the current release until a later release appears on the release channel (on the `EarlyAccess` release channel in the example).
* When switching to a less stable release channel (e.g., from `EarlyAccess` to `Alpha`), the following actions take place:
  * Deckhouse downloads release data from the release channel (the `Alpha` release channel in the example) and compares it with the existing `DeckhouseReleases`.
  * Then Deckhouse performs the update according to the [update parameters](modules/deckhouse/configuration.html#parameters-update).

{% offtopic title="The scheme of using the releaseChannel parameter during Deckhouse installation and operation" %}
![The scheme of using the releaseChannel parameter during Deckhouse installation and operation](images/common/deckhouse-update-process.png)
{% endofftopic %}

### What do I do if Deckhouse fails to retrieve updates from the release channel?

1. Make sure that the desired release channel is [configured](#how-do-i-set-the-desired-release-channel).
1. Make sure that the DNS name of the Deckhouse container registry is resolved correctly.
1. Retrieve and compare the IP addresses of the Deckhouse container registry (`registry.deckhouse.io`) on one of the nodes and in the Deckhouse pod. They should match.

   To retrieve the IP address of the Deckhouse container registry on a node, run the following command:

   ```shell
   getent ahosts registry.deckhouse.io
   ```

   Example output:

   ```console
   46.4.145.194    STREAM registry.deckhouse.io
   46.4.145.194    DGRAM
   46.4.145.194    RAW
   ```

   To retrieve the IP address of the Deckhouse container registry in a pod, run the following command:

   ```shell
   kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- getent ahosts registry.deckhouse.io
   ```

   Example output:
  
   ```console
   46.4.145.194    STREAM registry.deckhouse.io
   46.4.145.194    DGRAM  registry.deckhouse.io
   ```

   If the retrieved IP addresses do not match, inspect the DNS settings on the host.
   Specifically, check the list of domains in the `search` parameter of the `/etc/resolv.conf` file (it affects name resolution in the Deckhouse pod). If the `search` parameter of the `/etc/resolv.conf` file includes a domain where wildcard record resolution is configured, it may result in incorrect resolution of the IP address of the Deckhouse container registry (see the following example).

{% offtopic title="Example of DNS settings that may cause errors in resolving the IP address of the Deckhouse container registry..." %}

In the example, DNS settings produce different results when resolving names on the host and in the Kubernetes pod:

- The `/etc/resolv.conf` file on the node:

  ```text
  nameserver 10.0.0.10
  search company.my
  ```

  > Note that the `ndot` parameter defaults to 1 (`options ndots:1`) on the node. But in Kubernetes pods, the `ndot` parameter is set to **5**. Therefore, the logic for resolving DNS names with 5 dots or less in the name is different on the host and in the pod.

- The `company.my` DNS zone is configured to resolve wildcard records `*.company.my` to `10.0.0.100`. That is, any DNS name in the `company.my` zone for which there is no specific DNS entry is resolved to `10.0.0.100`.

In this case, subject to the `search` parameter specified in the `/etc/resolv.conf` file, when accessing the `registry.deckhouse.io` address **on the node**, the system will try to obtain the IP address for the `registry.deckhouse.io` name (it treats it as a fully qualified name given the default setting of `options ndots:1`).

On the other hand, when accessing `registry.deckhouse.io` **from a Kubernetes pod**, given the `options ndots:5` parameter (the default one in Kubernetes) and the `search` parameter, the system will initially try to resolve the IP address for the `registry.deckhouse.io.company.my` name. The `registry.deckhouse.io.company.my` name will be resolved to `10.0.0.100` because the `company.my` DNS zone is configured to resolve wildcard records `*.company.my` to `10.0.0.100`. As a result, the `registry.deckhouse.io` host and information about the available Deckhouse updates will be unreachable.

{% endofftopic %}

### How to check the job queue in Deckhouse?

To view the status of all Deckhouse job queues, run the following command:

```shell
kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
```

Example of the output (queues are empty):

```console
Summary:
- 'main' queue: empty.
- 88 other queues (0 active, 88 empty): 0 tasks.
- no tasks to handle.
```

To view the status of the `main` Deckhouse task queue, run the following command:

```shell
kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
```

Example of the output (38 tasks in the `main` queue):

```console
Queue 'main': length 38, status: 'run first task'
```

Example of the output (the `main` queue is empty):

```console
Queue 'main': length 0, status: 'waiting for task 0s'
```

## Air-gapped environment; working via proxy and third-party registry

### How do I configure Deckhouse to use a third-party registry?

{% alert level="warning" %}
This feature is available in Enterprise Edition only.
{% endalert %}

{% alert level="warning" %}
Deckhouse only supports Bearer authentication for container registries.

Tested and guaranteed to work with the following container registries:
{%- for registry in site.data.supported_versions.registries %}
[{{- registry[1].shortname }}]({{- registry[1].url }})
{%- unless forloop.last %}, {% endunless %}
{%- endfor %}.
{% endalert %}

Deckhouse can be configured to work with a third-party registry (e.g., a proxy registry inside private environments).

Define the following parameters in the `InitConfiguration` resource:

* `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/ee`. The path to the Deckhouse EE image in the third-party registry, for example `imagesRepo: registry.deckhouse.io/deckhouse/ee`;
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

You can use the following script to generate `registryDockerCfg`:

```shell
declare MYUSER='<PROXY_USERNAME>'
declare MYPASSWORD='<PROXY_PASSWORD>'
declare MYREGISTRY='<PROXY_REGISTRY>'

MYAUTH=$(echo -n "$MYUSER:$MYPASSWORD" | base64 -w0)
MYRESULTSTRING=$(echo -n "{\"auths\":{\"$MYREGISTRY\":{\"username\":\"$MYUSER\",\"password\":\"$MYPASSWORD\",\"auth\":\"$MYAUTH\"}}}" | base64 -w0)

echo "$MYRESULTSTRING"
```

The `InitConfiguration` resource provides two more parameters for non-standard third-party registry configurations:

* `registryCA` - root CA certificate to validate the third-party registry's HTTPS certificate (if self-signed certificates are used);
* `registryScheme` - registry scheme (`HTTP` or `HTTPS`). The default value is `HTTPS`.

<div markdown="0" style="height: 0;" id="tips-for-configuring-the-third-party-registry"></div>

### Tips for configuring Nexus

{% alert level="warning" %}
When interacting with a `docker` repository located in Nexus (e. g. executing `docker pull`, `docker push` commands), you must specify the address in the `<NEXUS_URL>:<REPOSITORY_PORT>/<PATH>` format.

Using the `URL` value from the Nexus repository options is **not acceptable**
{% endalert %}

The following requirements must be met if the [Nexus](https://github.com/sonatype/nexus-public) repository manager is used:

* Docker **proxy** repository must be pre-created (*Administration* -> *Repository* -> *Repositories*):
  * The `Maximum metadata age` parameter is set to `0` for the repository.
* Access control configured as follows:
  * The **Nexus** role is created (*Administration* -> *Security* -> *Roles*) with the following permissions:
    * `nx-repository-view-docker-<repository>-browse`
    * `nx-repository-view-docker-<repository>-read`
  * A user (*Administration* -> *Security* -> *Users*) with the **Nexus** role is created.

**Configuration**:

1. Create a docker **proxy** repository (*Administration* -> *Repository* -> *Repositories*) pointing to the [Deckhouse registry](https://registry.deckhouse.io/):
  ![Create docker proxy repository](images/registry/nexus/nexus-repository.png)

1. Fill in the fields on the Create page as follows:
   * `Name` must contain the name of the repository you created earlier, e.g., `d8-proxy`.
   * `Repository Connectors / HTTP` or `Repository Connectors / HTTPS` must contain a dedicated port for the created repository, e.g., `8123` or other.
   * `Remote storage` must be set to `https://registry.deckhouse.io/`.
   * You can disable `Auto blocking enabled` and `Not found cache enabled` for debugging purposes, otherwise they must be enabled.
   * `Maximum Metadata Age` must be set to `0`.
   * `Authentication` must be enabled if you plan to use Deckhouse Enterprise Edition and the related fields must be set as follows:
     * `Authentication Type` must be set to `Username`.
     * `Username` must be set to `license-token`.
     * `Password` must contain your license key for Deckhouse Enterprise Edition.

   ![Repository settings example 1](images/registry/nexus/nexus-repo-example-1.png)
   ![Repository settings example 2](images/registry/nexus/nexus-repo-example-2.png)
   ![Repository settings example 3](images/registry/nexus/nexus-repo-example-3.png)

1. Configure Nexus access control to allow Nexus access to the created repository:
   * Create a **Nexus** role (*Administration* -> *Security* -> *Roles*) with the `nx-repository-view-docker-<repository>-browse` and `nx-repository-view-docker-<repository>-read` permissions.

   ![Create a Nexus role](images/registry/nexus/nexus-role.png)

   * Create a user with the role above granted.

   ![Create a Nexus user](images/registry/nexus/nexus-user.png)

Thus, Deckhouse images will be available at `https://<NEXUS_HOST>:<REPOSITORY_PORT>/deckhouse/ee:<d8s-version>`.

### Tips for configuring Harbor

Use the [Harbor Proxy Cache](https://github.com/goharbor/harbor) feature.

* Create a Registry:
  * `Administration -> Registries -> New Endpoint`.
  * `Provider`: `Docker Registry`.
  * `Name` — specify any of your choice.
  * `Endpoint URL`: `https://registry.deckhouse.io`.
  * Specify the `Access ID` and `Access Secret` for Deckhouse Enterprise Edition.

  ![Create a Registry](images/registry/harbor/harbor1.png)

* Create a new Project:
  * `Projects -> New Project`.
  * `Project Name` will be used in the URL. You can choose any name, for example, `d8s`.
  * `Access Level`: `Public`.
  * `Proxy Cache` — enable and choose the Registry, created in the previous step.

  ![Create a new Project](images/registry/harbor/harbor2.png)

Thus, Deckhouse images will be available at `https://your-harbor.com/d8s/deckhouse/ee:{d8s-version}`.

### Manually uploading Deckhouse Kubernetes Platform, vulnerability scanner DB and Deckhouse modules to private registry

{% alert level="warning" %}
The `d8 mirror` command group is not available for Community Edition (CE) and Basic Edition (BE).
{% endalert %}

{% alert level="info" %}
Check [releases.deckhouse.io](https://releases.deckhouse.io) for the current status of the release channels.
{% endalert %}

1. [Download and install the Deckhouse CLI tool](deckhouse-cli/).

1. Pull Deckhouse images using the `d8 mirror pull` command.

   By default, `d8 mirror` pulls only the latest available patch versions for every actual Deckhouse release, latest enterprise security scanner databases (if your edition supports it) and the current set of officially supplied modules.
   For example, for Deckhouse 1.59, only version `1.59.12` will be pulled, since this is sufficient for updating Deckhouse from 1.58 to 1.59.

   Run the following command (specify the edition code and the license key) to download actual images:

   ```shell
   d8 mirror pull \
     --source='registry.deckhouse.io/deckhouse/<EDITION>' \
     --license='<LICENSE_KEY>' /home/user/d8-bundle
   ```

   where:
   - `<EDITION>` — the edition code of the Deckhouse Kubernetes Platform (for example, `ee`, `se`, `se-plus`).
   - `<LICENSE_KEY>` — Deckhouse Kubernetes Platform license key.
   - `/home/user/d8-bundle` — the directory to store the resulting bundle into. It will be created if not present.

   > If the loading of images is interrupted, rerunning the command will resume the loading if no more than a day has passed since it stopped.

   You can also use the following command options:
   - `--no-pull-resume` — to forcefully start the download from the beginning;
   - `--no-platform` — to skip downloading the Deckhouse Kubernetes Platform package (platform.tar);
   - `--no-modules` — to skip downloading modules packages (module-*.tar);
   - `--no-security-db` — to skip downloading security scanner databases (security.tar);
   - `--since-version=X.Y` — to download all versions of Deckhouse starting from the specified minor version. This parameter will be ignored if a version higher than the version on the Rock Solid updates channel is specified. This parameter cannot be used simultaneously with the `--deckhouse-tag` parameter;
   - `--deckhouse-tag` — to download only a specific build of Deckhouse (without considering update channels). This parameter cannot be used simultaneously with the `--since-version` parameter;
   - `--include-module` / `-i` = `name[@Major.Minor]` — to download only a specific whitelist of modules (and optionally their minimal versions). Specify multiple times to whitelist more modules. This flags are ignored if used with `--no-modules`.
   - `--exclude-module` / `-e` = `name` — to skip downloading of a specific blacklisted set of modules. Specify multiple times to blacklist more modules. Ignored if `--no-modules` or `--include-module` are used.
   - `--modules-path-suffix` — to change the suffix of the module repository path in the main Deckhouse repository. By default, the suffix is `/modules`. (for example, the full path to the repository with modules will look like `registry.deckhouse.io/deckhouse/EDITION/modules` with this default).
   - `--gost-digest` — for calculating the checksums of the bundle in the format of GOST R 34.11-2012 (Streebog). The checksum for each package will be displayed and written to a file with the extension `.tar.gostsum` in the folder with the package;
   - `--source` — to specify the address of the Deckhouse source registry;
      - To authenticate in the official Deckhouse image registry, you need to use a license key and the `--license` parameter;
      - To authenticate in a third-party registry, you need to use the `--source-login` and `--source-password` parameters;
   - `--images-bundle-chunk-size=N` — to specify the maximum file size (in GB) to split the image archive into. As a result of the operation, instead of a single file archive, a set of `.chunk` files will be created (e.g., `d8.tar.NNNN.chunk`). To upload images from such a set of files, specify the file name without the `.NNNN.chunk` suffix in the `d8 mirror push` command (e.g., `d8.tar` for files like `d8.tar.NNNN.chunk`);
   - `--tmp-dir` — path to a temporary directory to use for image pulling and pushing. All processing is done in this directory, so make sure there is enough free disk space to accommodate the entire bundle you are downloading. By default, `.tmp` subdirectory under the bundle directory is used.

   Additional configuration options for the `d8 mirror` family of commands are available as environment variables:
    - `HTTP_PROXY`/`HTTPS_PROXY` — URL of the proxy server for HTTP(S) requests to hosts that are not listed in the variable `$NO_PROXY`;
    - `NO_PROXY` — comma-separated list of hosts to exclude from proxying. Supported value formats include IP addresses (`1.2.3.4`), CIDR notations (`1.2.3.4/8`), domains, and the asterisk character (`*`).  The IP addresses and domain names can also include a literal port number (`1.2.3.4:80`). The domain name matches that name and all the subdomains. The domain name with a leading `.` matches subdomains only. For example, `foo.com` matches `foo.com` and `bar.foo.com`; `.y.com` matches `x.y.com` but does not match `y.com`. A single asterisk `*` indicates that no proxying should be done;
    - `SSL_CERT_FILE` — path to the SSL certificate. If the variable is set, system certificates are not used;
    - `SSL_CERT_DIR` — list of directories to search for SSL certificate files, separated by a colon. If set, system certificates are not used. [See more...](https://www.openssl.org/docs/man1.0.2/man1/c_rehash.html);
    - `MIRROR_BYPASS_ACCESS_CHECKS` — set to `1` to skip validation of registry credentials;

   Example of a command to download all versions of Deckhouse EE starting from version 1.59 (provide the license key):

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --source='registry.deckhouse.io/deckhouse/ee' \
   --since-version=1.59 /home/user/d8-bundle
   ```

   Example of a command to download versions of Deckhouse SE for every release-channel available:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --source='registry.deckhouse.io/deckhouse/se' \
   /home/user/d8-bundle
   ```

   Example of a command to download all versions of Deckhouse hosted on a third-party registry:

   ```shell
   d8 mirror pull \
   --source='corp.company.com:5000/sys/deckhouse' \
   --source-login='<USER>' --source-password='<PASSWORD>' /home/user/d8-bundle
   ```

   Example of a command to download latest vulnerability scanner databases (if available for your deckhouse edition):

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --source='registry.deckhouse.io/deckhouse/ee' \
   --no-platform --no-modules /home/user/d8-bundle
   ```

   Example of a command to download all of Deckhouse modules available in registry:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --source='registry.deckhouse.io/deckhouse/ee' \
   --no-platform --no-security-db /home/user/d8-bundle
   ```

   Example of a command to download `stronghold` and `secrets-store-integration` Deckhouse modules:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --source='registry.deckhouse.io/deckhouse/ee' \
   --no-platform --no-security-db \
   --include-module stronghold \
   --include-module secrets-store-integration \
   /home/user/d8-bundle
   ```

1. Upload the bundle with the pulled Deckhouse images to a host with access to the air-gapped registry and install the [Deckhouse CLI](deckhouse-cli/) tool onto it.

1. Push the images to the air-gapped registry using the `d8 mirror push` command.

   The `d8 mirror push` command uploads images from all packages present in the given directory to the repository.
   If you need to upload only some specific packages to the repository, you can either run the command for each required package, passing in the direct path to the tar package instead of the directory, or by removing the `.tar` extension from unnecessary packages or moving them outside the directory.

   Example of a command for pushing images from the `/mnt/MEDIA/d8-images` directory (specify authorization data if necessary):

   ```shell
   d8 mirror push /mnt/MEDIA/d8-images 'corp.company.com:5000/sys/deckhouse' \
     --registry-login='<USER>' --registry-password='<PASSWORD>'
   ```

   > Before pushing images, make sure that the path for loading into the registry exists (`/sys/deckhouse` in the example above), and the account being used has write permissions.
   > Harbor users, please note that you will not be able to upload images to the project root; instead use a dedicated repository in the project to host Deckhouse images.

1. Once pushing images to the air-gapped private registry is complete, you are ready to install Deckhouse from it. Refer to the [Getting started](/products/kubernetes-platform/gs/bm-private/step2.html) guide.

   When launching the installer, use a repository where Deckhouse images have previously been loaded instead of official Deckhouse registry. For example, the address for launching the installer will look like `corp.company.com:5000/sys/deckhouse/install:stable` instead of `registry.deckhouse.io/deckhouse/ee/install:stable`.

   During installation, add your registry address and authorization data to the [InitConfiguration](installing/configuration.html#initconfiguration) resource (the [imagesRepo](installing/configuration.html#initconfiguration-deckhouse-imagesrepo) and [registryDockerCfg](installing/configuration.html#initconfiguration-deckhouse-registrydockercfg) parameters; you might refer to [step 3]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/bm-private/step3.html) of the Getting started guide as well).

   After installation, apply DeckhouseReleases manifests that were generated by the `d8 mirror pull` command to your cluster via Deckhouse CLI as follows:

   ```shell
   d8 k apply -f ./deckhousereleases.yaml
   ```

### How do I switch a running Deckhouse cluster to use a third-party registry?

{% alert level="warning" %}
Using a registry other than `registry.deckhouse.io` is only available in the Enterprise Edition.
{% endalert %}

To switch the Deckhouse cluster to using a third-party registry, follow these steps:

* Run `deckhouse-controller helper change-registry` inside the Deckhouse Pod with the new registry settings.
  * Example:

    ```shell
    kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee
    ```

  * If the registry uses a self-signed certificate, put the root CA certificate that validates the registry's HTTPS certificate to file `/tmp/ca.crt` in the Deckhouse Pod and add the `--ca-file /tmp/ca.crt` option to the script or put the content of CA into a variable as follows:

    ```shell
    CA_CONTENT=$(cat <<EOF
    -----BEGIN CERTIFICATE-----
    CERTIFICATE
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    CERTIFICATE
    -----END CERTIFICATE-----
    EOF
    )
    kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- bash -c "echo '$CA_CONTENT' > /tmp/ca.crt && deckhouse-controller helper change-registry --ca-file /tmp/ca.crt --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee"
    ```

  * To view the list of available keys of the `deckhouse-controller helper change-registry` command, run the following command:

    ```shell
    kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --help
    ```

    Example output:

    ```console
    usage: deckhouse-controller helper change-registry [<flags>] <new-registry>

    Change registry for deckhouse images.

    Flags:
      --help               Show context-sensitive help (also try --help-long and --help-man).
      --user=USER          User with pull access to registry.
      --password=PASSWORD  Password/token for registry user.
      --ca-file=CA-FILE    Path to registry CA.
      --scheme=SCHEME      Used scheme while connecting to registry, http or https.
      --dry-run            Don't change deckhouse resources, only print them.
      --new-deckhouse-tag=NEW-DECKHOUSE-TAG
                          New tag that will be used for deckhouse deployment image (by default
                          current tag from deckhouse deployment will be used).

    Args:
      <new-registry>  Registry that will be used for deckhouse images (example:
                      registry.deckhouse.io/deckhouse/ce). By default, https will be used, if you need
                      http - provide '--scheme' flag with http value
    ```

* Wait for the Deckhouse Pod to become `Ready`. Restart Deckhouse Pod if it will be in `ImagePullBackoff` state.
* Wait for bashible to apply the new settings on the master node. The bashible log on the master node (`journalctl -u bashible`) should contain the message `Configuration is in sync, nothing to do`.
* If you want to disable Deckhouse automatic updates, remove the [releaseChannel](modules/deckhouse/configuration.html#parameters-releasechannel) parameter from the `deckhouse` module configuration.
* Check if there are Pods with original registry in cluster (if there are — restart them):

  ```shell
  kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
    | select(.image | startswith("registry.deckhouse"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
  ```

### How to bootstrap a cluster and run Deckhouse without the usage of release channels?

{% alert level="warning" %}
This method should only be used if there are no release channel images in your air-gapped registry.
{% endalert %}

If you want to install Deckhouse with automatic updates disabled:

1. Use the installer image tag of the corresponding version. For example, if you want to install the `v1.44.3` release, use the `your.private.registry.com/deckhouse/install:v1.44.3` image.
1. Specify the corresponding version number in the [deckhouse.devBranch](installing/configuration.html#initconfiguration-deckhouse-devbranch) parameter in the [InitConfiguration](installing/configuration.html#initconfiguration) resource.
   > **Do not specify** the [deckhouse.releaseChannel](installing/configuration.html#initconfiguration-deckhouse-releasechannel) parameter in the [InitConfiguration](installing/configuration.html#initconfiguration) resource.

If you want to disable automatic updates for an already installed Deckhouse (including patch release updates), remove the [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel) parameter from the `deckhouse` module configuration.

### Using a proxy server

{% alert level="warning" %}
This feature is available in Enterprise Edition only.
{% endalert %}

{% offtopic title="Example of steps for configuring a Squid-based proxy server..." %}

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

{% endofftopic %}

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
kubernetesVersion: "Automatic"
cri: "Containerd"
clusterDomain: "cluster.local"
proxy:
  httpProxy: "http://user:password@proxy.company.my:3128"
  httpsProxy: "https://user:password@proxy.company.my:8443"
```

{% raw %}

### Autoloading proxy variables for users at CLI

Since DKP v1.67, the file `/etc/profile.d/d8-system-proxy.sh`, which sets proxy variables for users, is no longer configurable. To autoload proxy variables for users at the CLI, use the `NodeGroupConfiguration` resource:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: profile-proxy.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 99
  content: |
    {{- if .proxy }}
      {{- if .proxy.httpProxy }}
    export HTTP_PROXY={{ .proxy.httpProxy | quote }}
    export http_proxy=${HTTP_PROXY}
      {{- end }}
      {{- if .proxy.httpsProxy }}
    export HTTPS_PROXY={{ .proxy.httpsProxy | quote }}
    export https_proxy=${HTTPS_PROXY}
      {{- end }}
      {{- if .proxy.noProxy }}
    export NO_PROXY={{ .proxy.noProxy | join "," | quote }}
    export no_proxy=${NO_PROXY}
      {{- end }}
    bb-sync-file /etc/profile.d/profile-proxy.sh - << EOF
    export HTTP_PROXY=${HTTP_PROXY}
    export http_proxy=${HTTP_PROXY}
    export HTTPS_PROXY=${HTTPS_PROXY}
    export https_proxy=${HTTPS_PROXY}
    export NO_PROXY=${NO_PROXY}
    export no_proxy=${NO_PROXY}
    EOF
    {{- else }}
    rm -rf /etc/profile.d/profile-proxy.sh
    {{- end }}
```

{% endraw %}

## Changing the configuration

{% alert level="warning" %}
To apply node configuration changes, you need to run the `dhctl converge` using the Deckhouse installer. This command synchronizes the state of the nodes with the specified configuration.
{% endalert %}

### How do I change the configuration of a cluster?

The general cluster parameters are stored in the [ClusterConfiguration](installing/configuration.html#clusterconfiguration) structure.

To change the general cluster parameters, run the command:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit cluster-configuration
```

After saving the changes, Deckhouse will bring the cluster configuration to the state according to the changed configuration. Depending on the size of the cluster, this may take some time.

### How do I change the configuration of a cloud provider in a cluster?

Cloud provider setting of a cloud of hybrid cluster are stored in the `<PROVIDER_NAME>ClusterConfiguration` structure, where `<PROVIDER_NAME>` — name/code of the cloud provider. E.g., for an OpenStack provider, the structure will be called [OpenStackClusterConfiguration]({% if site.mode == 'module' and site.d8Revision == 'CE' %}{{ site.urls[page.lang] }}/products/kubernetes-platform/documentation/v1/{% endif %}modules/cloud-provider-openstack/cluster_configuration.html).

Regardless of the cloud provider used, its settings can be changed using the following command:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

### How do I change the configuration of a static cluster?

Settings of a static cluster are stored in the [StaticClusterConfiguration](installing/configuration.html#staticclusterconfiguration) structure.

To change the settings of a static cluster, run the command:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit static-cluster-configuration
```

### How to switch Deckhouse EE to CE?

{% alert level="warning" %}
The instruction implies using the public address of the container registry: `registry.deckhouse.io`. Using a registry other than `registry.deckhouse.io` is only available in the Enterprise Edition.

Deckhouse CE does not support cloud clusters on OpenStack and VMware vSphere.
{% endalert %}

Follow this steps to switch a Deckhouse Enterprise Edition to Community Edition (all commands must be executed on the master node, either by a user with a configured `kubectl` context or by a superuser):

1. Run a temporary Deckhouse CE pod to retrieve up-to-date digests and module lists:

   ```shell
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   kubectl run ce-image --image=registry.deckhouse.io/deckhouse/ce/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   > Run an image of the latest installed Deckhouse version in the cluster. To find out which version is currently installed, use the following command:
   >
   > ```shell
   > kubectl get deckhousereleases | grep Deployed
   > ```

1. Wait for the pod to become `Running` and then run the following commands:

   * Retrieve the value of `CE_SANDBOX_IMAGE`:

     ```shell
     CE_SANDBOX_IMAGE=$(kubectl exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.pause")
     ```

     Check the result of the command to make sure it was successful:

     ```shell
     echo $CE_SANDBOX_IMAGE
     ```

     Example output:

     ```console
     sha256:2a909cb9df4d0207f1fe5bd9660a0529991ba18ce6ce7b389dc008c05d9022d1
     ```

   * Retrieve the value of `CE_K8S_API_PROXY`:

     ```shell
     CE_K8S_API_PROXY=$(kubectl exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".controlPlaneManager.kubernetesApiProxy")
     ```

     Check the result of the command to make sure it was successful:

     ```shell
     echo $CE_K8S_API_PROXY
     ```

     Example output:

     ```console
     sha256:a5442437976a11dfa4860c2fbb025199d9d1b074222bb80173ed36b9006341dd
     ```

   * Retrieve the value of `CE_REGISTRY_PACKAGE_PROXY`:

     ```shell
     CE_REGISTRY_PACKAGE_PROXY=$(kubectl exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".registryPackagesProxy.registryPackagesProxy")
     ```

     Run the command:

     ```shell
     crictl pull registry.deckhouse.io/deckhouse/ce@$CE_REGISTRY_PACKAGE_PROXY
     ```

     Example output:

     ```console
     Image is up to date for sha256:8127efa0f903a7194d6fb7b810839279b9934b200c2af5fc416660857bfb7832
     ```

   * Retrieve the value of `CE_MODULES`:

     ```shell
     CE_MODULES=$(kubectl exec ce-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
     ```

     Check the result of the command to make sure it was successful:

     ```shell
     echo $CE_MODULES
     ```

     Example output:

     ```console
     common priority-class deckhouse external-module-manager registrypackages ...
     ```

   * Retrieve the value of `USED_MODULES`:

     ```shell
     USED_MODULES=$(kubectl get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
     ```

     Check the result of the command to make sure it was successful:

     ```shell
     echo $USED_MODULES
     ```

     Example output:

     ```console
     admission-policy-engine cert-manager chrony ...
     ```

   * Retrieve the value of `MODULES_WILL_DISABLE`:

     ```shell
     MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $CE_MODULES | tr ' ' '\n'))
     ```

     Check the result of the command to make sure it was successful:

     ```shell
     echo $MODULES_WILL_DISABLE
     ```

     Example output:

     ```console
     node-local-dns registry-packages-proxy
     ```

     > Note that if `registry-packages-proxy` is listed in `$MODULES_WILL_DISABLE`, you will need to enable it back, otherwise the cluster will not be able to migrate to Deckhouse CE images. See paragraph 8 on how to enable it.

1. Ensure that the modules used in the cluster are supported in Deckhouse CE.

   For this, print a list of modules that are not supported in Deckhouse CE and will be disabled:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Inspect the list and make sure that the listed modules are not in use in the cluster and that it is safe to disable them.

   Disable the modules that are not supported in Deckhouse CE:

   ```shell
   echo $MODULES_WILL_DISABLE |
     tr ' ' '\n' | awk {'print "kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module disable",$1'} | bash
   ```

   An example of the output you might get as a result of running the previous command:

   ```console
   Defaulted container "deckhouse" out of: deckhouse, kube-rbac-proxy, init-external-modules (init)
   Module node-local-dns disabled

   Defaulted container "deckhouse" out of: deckhouse, kube-rbac-proxy, init-external-modules (init)
   Module registry-packages-proxy disabled
   ```

1. Create a `NodeGroupConfiguration` resource:

   ```shell
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-ce-config.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 30
     content: |
       _on_containerd_config_changed() {
         bb-flag-set containerd-need-restart
       }
       bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

       mkdir -p /etc/containerd/conf.d
       bb-sync-file /etc/containerd/conf.d/ce-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           sandbox_image = "registry.deckhouse.io/deckhouse/ce@$CE_SANDBOX_IMAGE"
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry.deckhouse.io".auth]
               auth = ""
       EOF_TOML

       sed -i 's|image: .*|image: registry.deckhouse.io/deckhouse/ce@$CE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
       sed -i 's|crictl pull .*|crictl pull registry.deckhouse.io/deckhouse/ce@$CE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh

   EOF
   ```

   Wait for the `/etc/containerd/conf.d/ce-registry.toml` file to propagate to the nodes and for bashible synchronization to complete.

   The synchronization status can be tracked by the `UPTODATE` value (the displayed number of nodes in this status must match the total number of nodes (`NODES`) in the group):

   ```shell
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Example output:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   Also, the `Configuration is in sync, nothing to do` message must show up in the bashible systemd service log after you run the following command:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ee-to-ce-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-ce-0 bashible.sh[53407]: Annotate node master-ee-to-ce-0 with annotation node.deckhouse.io/  configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-ce-0 bashible.sh[53407]: Successful annotate node master-ee-to-ce-0 with annotation node.deckhouse.io/ configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-ce-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Update the secret to access the Deckhouse registry by running the following command:

   ```bash
   kubectl -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry.deckhouse.io\": {}}}" \
     --from-literal="address"=registry.deckhouse.io \
     --from-literal="path"=/deckhouse/ce \
     --from-literal="scheme"=https \
     --type=kubernetes.io/dockerconfigjson \
     --dry-run='client' \
     -o yaml | kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- kubectl replace -f -
   ```

1. Apply the webhook-handler image:

  ```shell
  HANDLER=$(kubectl exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".deckhouse.webhookHandler")
  kubectl --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/webhook-handler handler=registry.deckhouse.io/deckhouse/ce@$HANDLER
  ```

1. Apply the Deckhouse CE image:

   ```shell
   DECKHOUSE_KUBE_RBAC_PROXY=$(kubectl exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   DECKHOUSE_INIT_CONTAINER=$(kubectl exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   kubectl --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/deckhouse init-downloaded-modules=registry.deckhouse.io/deckhouse/ce@$DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry.deckhouse.io/deckhouse/ce@$DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry.deckhouse.io/deckhouse/ce:$DECKHOUSE_VERSION
   ```

1. Wait for the Deckhouse pod to become `Ready` and for [all the queued jobs to complete](https://deckhouse.io/products/kubernetes-platform/documentation/latest/deckhouse-faq.html#how-to-check-the-job-queue-in-deckhouse). If an `ImagePullBackOff` error is generated in the process, wait for the pod to be restarted automatically.

   Use the following command to check the Deckhouse pod's status:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

   Use the following command to check the Deckhouse queue:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller queue list
   ```

1. Check if there are any pods with the Deckhouse EE registry address left in the cluster:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
      | select(.image | contains("deckhouse.io/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   > If the module was disabled in the process, enable it again:
   >
   > ```shell
   > kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module enable registry-packages-proxy
   > ```

1. Purge temporary files, `NodeGroupConfiguration` resource, and variables:

   ```shell
   kubectl delete ngc containerd-ce-config.sh
   kubectl delete pod ce-image
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: del-temp-config.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 90
     content: |
       if [ -f /etc/containerd/conf.d/ce-registry.toml ]; then
         rm -f /etc/containerd/conf.d/ce-registry.toml
       fi
   EOF
   ```

   Once bashible synchronization is complete (you can track the synchronization status on nodes via the `UPTODATE` value of the NodeGroup), delete the NodeGroupConfiguration resource you created earlier:

   ```shell
   kubectl delete ngc del-temp-config.sh
   ```

### How to switch Deckhouse CE to EE?

You will need a valid license key (you can [request a trial license key](https://deckhouse.io/products/enterprise_edition.html) if necessary).

{% alert %}
The instruction implies using the public address of the container registry: `registry.deckhouse.io`. If you use a different container registry address, change the commands or use [the instruction](#how-do-i-configure-deckhouse-to-use-a-third-party-registry) for switching Deckhouse to using a third-party registry.
{% endalert %}

Follow this steps to switch a Deckhouse Community Edition to Enterprise Edition (all commands must be executed on the master node, either by a user with a configured `kubectl` context or by a superuser):

1. Create the variables containing the license token:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
   ```

1. Create a NodeGroupConfiguration resource for authorization in `registry.deckhouse.io` during the migration:

   ```shell
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-ee-config.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 30
     content: |
       _on_containerd_config_changed() {
         bb-flag-set containerd-need-restart
       }
       bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

       mkdir -p /etc/containerd/conf.d
       bb-sync-file /etc/containerd/conf.d/ee-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry.deckhouse.io".auth]
               auth = "$AUTH_STRING"
       EOF_TOML

   EOF
   ```

   Wait for the `/etc/containerd/conf.d/ee-registry.toml` file to propagate to the nodes and for bashible synchronization to complete.

   You can track the synchronization status via the `UPTODATE` value (the displayed number of nodes having this status must match the total number of nodes (`NODES`) in the group):

   ```shell
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Example output:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   Also, the `Configuration is in sync, nothing to do` message must show up in the bashible systemd service log after you run the following command:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ce-to-ee-0 bashible.sh[53407]: , nothing to do.
   Aug 21 11:04:28 master-ce-to-ee-0 bashible.sh[53407]: Annotate node master-ce-to-ee-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master ce-to-ee-0 bashible.sh[53407]: Successful annotate node master-ce-to-ee-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ce-to-ee-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

   Run a temporary Deckhouse EE pod to retrieve up-to-date digests and module lists:

   ```shell
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   kubectl run ee-image --image=registry.deckhouse.io/deckhouse/ee/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   > Run an image of the latest installed Deckhouse version in the cluster. To find out which version is currently installed, use the following command:
   >
   > ```shell
   > kubectl get deckhousereleases | grep Deployed
   > ```

1. Wait for the pod to become `Running` and then run the following commands:

   * Retrieve the value of `EE_SANDBOX_IMAGE`:

     ```shell
     EE_SANDBOX_IMAGE=$(kubectl exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.pause")
     ```

     Check the result of the command to make sure it was successful:

     ```shell
     echo $EE_SANDBOX_IMAGE
     ```

     Example output:

     ```console
     sha256:2a909cb9df4d0207f1fe5bd9660a0529991ba18ce6ce7b389dc008c05d9022d1
     ```

   * Retrieve the value of `EE_K8S_API_PROXY`:

     ```shell
     EE_K8S_API_PROXY=$(kubectl exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".controlPlaneManager.kubernetesApiProxy")
     ```

     Check the result of the command to make sure it was successful:

     ```shell
     echo $EE_K8S_API_PROXY
     ```

     Example output:

     ```console
     sha256:80a2cf757adad6a29514f82e1c03881de205780dbd87c6e24da0941f48355d6c
     ```

   * Retrieve the value of `EE_REGISTRY_PACKAGE_PROXY`:

     ```shell
     EE_REGISTRY_PACKAGE_PROXY=$(kubectl exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".registryPackagesProxy.registryPackagesProxy")
     ```

     Run the command:

     ```shell
     crictl pull registry.deckhouse.io/deckhouse/ee@$EE_REGISTRY_PACKAGE_PROXY
     ```

     Example output:

     ```console
     Image is up to date for sha256:8127efa0f903a7194d6fb7b810839279b9934b200c2af5fc416660857bfb7832
     ```

1. Create a NodeGroupConfiguration resource:

   ```shell
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: ee-set-sha-images.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 30
     content: |
       _on_containerd_config_changed() {
         bb-flag-set containerd-need-restart
       }
       bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

       bb-sync-file /etc/containerd/conf.d/ee-sandbox.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           sandbox_image = "registry.deckhouse.io/deckhouse/ee@$EE_SANDBOX_IMAGE"
       EOF_TOML

       sed -i 's|image: .*|image: registry.deckhouse.io/deckhouse/ee@$EE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
       sed -i 's|crictl pull .*|crictl pull registry.deckhouse.io/deckhouse/ee@$EE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh

   EOF
   ```

   Wait for the `/etc/containerd/conf.d/ee-sandbox.toml` file to propagate to the nodes and for bashible synchronization to complete.

   You can track the synchronization status via the `UPTODATE` value (the displayed number of nodes having this status must match the total number of nodes (`NODES`) in the group):

   ```shell
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Example output:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   Also, the `Configuration is in sync, nothing to do` message must show up in the bashible systemd service log after you run the following command:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ce-to-ee-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ce-to-ee-0 bashible.sh[53407]: Annotate node master-ce-to-ee-0 with annotation node.deckhouse.io/ configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ce-to-ee-0 bashible.sh[53407]: Successful annotate node master-ce-to-ee-0 with annotation node.deckhouse.io/ configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ce-to-ee-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Update the secret to access the Deckhouse registry by running the following command:

   ```shell
   kubectl -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry.deckhouse.io\": { \"username\": \"license-token\", \"password\": \"$LICENSE_TOKEN\", \"auth\":    \"$AUTH_STRING\" }}}" \
     --from-literal="address"=registry.deckhouse.io \
     --from-literal="path"=/deckhouse/ee \
     --from-literal="scheme"=https \
     --type=kubernetes.io/dockerconfigjson \
     --dry-run='client' \
     -o yaml | kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- kubectl replace -f -
   ```

1. Apply the webhook-handler image:

  ```shell
  HANDLER=$(kubectl exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".deckhouse.webhookHandler")
  kubectl --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/webhook-handler handler=registry.deckhouse.io/deckhouse/ee@$HANDLER
  ```

1. Apply the Deckhouse EE image:

   ```shell
   DECKHOUSE_KUBE_RBAC_PROXY=$(kubectl exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   DECKHOUSE_INIT_CONTAINER=$(kubectl exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   kubectl --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/deckhouse init-downloaded-modules=registry.deckhouse.io/deckhouse/ee@$DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry.deckhouse.io/deckhouse/ee@$DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry.deckhouse.io/deckhouse/ee:$DECKHOUSE_VERSION
   ```

1. Wait for the Deckhouse pod to become `Ready` and for [all the queued jobs to complete](https://deckhouse.io/products/kubernetes-platform/documentation/latest/deckhouse-faq.html#how-to-check-the-job-queue-in-deckhouse). If an `ImagePullBackOff` error is generated in the process, wait for the pod to be restarted automatically.

   Use the following command to check the Deckhouse pod's status:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

   Use the following command to check the Deckhouse queue:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller queue list
   ```

1. Check if there are any pods with the Deckhouse CE registry address left in the cluster:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
      | select(.image | contains("deckhouse.io/deckhouse/ce"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

1. Purge temporary files, `NodeGroupConfiguration` resource, and variables:

   ```shell
   kubectl delete ngc containerd-ee-config.sh ee-set-sha-images.sh
   kubectl delete pod ee-image
   kubectl apply -f - <<EOF
       apiVersion: deckhouse.io/v1alpha1
       kind: NodeGroupConfiguration
       metadata:
         name: del-temp-config.sh
       spec:
         nodeGroups:
         - '*'
         bundles:
         - '*'
         weight: 90
         content: |
           if [ -f /etc/containerd/conf.d/ee-registry.toml ]; then
             rm -f /etc/containerd/conf.d/ee-registry.toml
           fi
           if [ -f /etc/containerd/conf.d/ee-sandbox.toml ]; then
             rm -f /etc/containerd/conf.d/ee-sandbox.toml
           fi
   EOF
   ```

   Once bashible synchronization is complete (you can track the synchronization status on nodes via the `UPTODATE` value of the NodeGroup), delete the NodeGroupConfiguration resource you created earlier:

   ```shell
   kubectl delete ngc del-temp-config.sh
   ```

### How to Switch Deckhouse EE to SE?

You will need a valid license key. You can [request a trial license key](https://deckhouse.io/products/kubernetes-platform) if necessary.

{% alert level="info" %}
The instruction implies using the public address of the container registry: `registry.deckhouse.io`. If you use a different container registry address, change the commands or use [the instruction](#how-do-i-configure-deckhouse-to-use-a-third-party-registry) for switching Deckhouse to using a third-party registry.

The Deckhouse SE edition does not support certain cloud providers `dynamix`, `openstack`, `VCD`, `VSphere` and a number of modules.
{% endalert %}

To switch Deckhouse Enterprise Edition to Standard Edition, follow these steps:

{% alert level="warning" %}
All commands should be executed on a master node of the existing cluster.
{% endalert %}

1. Set up environment variables with your license token:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
   ```

1. Create a NodeGroupConfiguration resource for transitional authorization in `registry.deckhouse.io`:

   ```shell
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-se-config.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 30
     content: |
       _on_containerd_config_changed() {
         bb-flag-set containerd-need-restart
       }
       bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

       mkdir -p /etc/containerd/conf.d
       bb-sync-file /etc/containerd/conf.d/se-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry.deckhouse.io".auth]
               auth = "$AUTH_STRING"
       EOF_TOML

   EOF
   ```

   Wait for the `/etc/containerd/conf.d/se-registry.toml` file to propagate to the nodes and for bashible synchronization to complete. You can track the synchronization status via the  `UPTODATE` value (the displayed number of nodes having this status must match the total number of nodes (`NODES`) in the group):

   ```shell
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Example output:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   Also, the `Configuration is in sync, nothing to do` message must show up in the bashible systemd service log after you run the following command:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Annotate node master-ee-to-se-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master ee-to-se-0 bashible.sh[53407]: Successful annotate node master-ee-to-se-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-se-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Run a temporary Deckhouse SE pod, to retrieve up-to-date digests and module lists:

   ```shell
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   kubectl run se-image --image=registry.deckhouse.io/deckhouse/se/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   > To find the latest Deckhouse version, you can run:
   >
   > ```shell
   > kubectl get deckhousereleases | grep Deployed
   > ```

1. Wait for the pod to become `Running` and then run the following commands:

   * Retrieve the value of `SE_SANDBOX_IMAGE`:

     ```shell
     SE_SANDBOX_IMAGE=$(kubectl exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.pause")
     ```

     Check the result of the command to make sure it was successful:

     ```shell
     echo $SE_SANDBOX_IMAGE
     ```

     Example output:

     ```console
     sha256:2a909cb9df4d0207f1fe5bd9660a0529991ba18ce6ce7b389dc008c05d9022d1
     ```

   * Retrieve the value of `SE_K8S_API_PROXY`:

     ```shell
     SE_K8S_API_PROXY=$(kubectl exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".controlPlaneManager.kubernetesApiProxy")
     ```

     Check the result of the command to make sure it was successful:

     ```shell
     echo $SE_K8S_API_PROXY
     ```

     Example output:

     ```console
     sha256:af92506a36f4bd032a6459295069f9478021ccf67d37557a664878bc467dd9fd
     ```

   * Retrieve the value of `SE_REGISTRY_PACKAGE_PROXY`:

     ```shell
     SE_REGISTRY_PACKAGE_PROXY=$(kubectl exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".registryPackagesProxy.registryPackagesProxy")
     ```

     Run the command:

     ```shell
     sudo /opt/deckhouse/bin/crictl pull registry.deckhouse.io/deckhouse/se@$SE_REGISTRY_PACKAGE_PROXY
     ```

     Example output:

     ```console
     Image is up to date for sha256:7e9908d47580ed8a9de481f579299ccb7040d5c7fade4689cb1bff1be74a95de
     ```

   * Retrieve the value of `SE_MODULES`:

     ```shell
     SE_MODULES=$(kubectl exec se-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
     ```

     Check the result of the command to make sure it was successful:

     ```shell
     echo $SE_MODULES
     ```

     Example output:

     ```console
     common priority-class deckhouse external-module-manager ...
     ```

   * Retrieve the value of `USED_MODULES`:

     ```shell
     USED_MODULES=$(kubectl get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
     ```

     Check the result of the command to make sure it was successful:

     ```shell
     echo $USED_MODULES
     ```

     Example output:

     ```console
     admission-policy-engine cert-manager chrony ...
     ```

   * Form the list of modules that must be disabled:

     ```shell
     MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $SE_MODULES | tr ' ' '\n'))
     ```

1. Verify that the modules in use are supported by the SE edition.
   Run the following command to display modules that are not supported by SE and will be disabled:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Check the list and make sure the functionality of these modules is not required in your cluster or that you are prepared to disable them.

   Disable the unsupported modules:

   ```shell
   echo $MODULES_WILL_DISABLE | 
     tr ' ' '\n' | awk {'print "kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module disable",$1'} | bash
   ```

   Wait until the Deckhouse pod is in the `Ready` state and [all tasks in the queue are completed](#how-to-check-the-job-queue-in-deckhouse).

1. Create a NodeGroupConfiguration resource:

   ```shell
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: se-set-sha-images.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 30
     content: |
       _on_containerd_config_changed() {
         bb-flag-set containerd-need-restart
       }
       bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

       bb-sync-file /etc/containerd/conf.d/se-sandbox.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           sandbox_image = "registry.deckhouse.io/deckhouse/se@$SE_SANDBOX_IMAGE"
       EOF_TOML

       sed -i 's|image: .*|image: registry.deckhouse.io/deckhouse/se@$SE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
       sed -i 's|crictl pull .*|crictl pull registry.deckhouse.io/deckhouse/se@$SE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh

   EOF
   ```

   Wait for the `/etc/containerd/conf.d/se-sandbox.toml` file to propagate to the nodes and for bashible synchronization to complete.

   You can track the synchronization status via the `UPTODATE` value (the displayed number of nodes having this status must match the total number of nodes (`NODES`) in the group):

   ```shell
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Example output:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   Also, the `Configuration is in sync, nothing to do` message must show up in the bashible systemd service log after you run the following command:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Annotate node master-ee-to-se-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master ee-to-se-0 bashible.sh[53407]: Successful annotate node master-ee-to-se-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-se-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Update the secret to access the Deckhouse registry by running the following command:

   ```shell
   kubectl -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry.deckhouse.io\": { \"username\": \"license-token\", \"password\": \"$LICENSE_TOKEN\", \"auth\":    \"$AUTH_STRING\" }}}" \
     --from-literal="address"=registry.deckhouse.io   --from-literal="path"=/deckhouse/se \
     --from-literal="scheme"=https   --type=kubernetes.io/dockerconfigjson \
     --dry-run=client \
     -o yaml | kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- kubectl replace -f -
   ```

1. Apply the webhook-handler image:

  ```shell
  HANDLER=$(kubectl exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".deckhouse.webhookHandler")
  kubectl --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/webhook-handler handler=registry.deckhouse.io/deckhouse/se@$HANDLER
  ```

1. Apply the Deckhouse SE image:

   ```shell
   DECKHOUSE_KUBE_RBAC_PROXY=$(kubectl exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   DECKHOUSE_INIT_CONTAINER=$(kubectl exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   kubectl --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/deckhouse init-downloaded-modules=registry.deckhouse.io/deckhouse/se@$DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry.deckhouse.io/deckhouse/se@$DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry.deckhouse.io/deckhouse/se:$DECKHOUSE_VERSION
   ```

   > To find the latest Deckhouse version, you can run:
   >
   > ```shell
   > kubectl get deckhousereleases | grep Deployed
   > ```

1. Wait for the Deckhouse pod to become `Ready` and for [all the queued jobs to complete](#how-to-check-the-job-queue-in-deckhouse). If an `ImagePullBackOff` error is generated in the process, wait for the pod to be restarted automatically.

   Use the following command to check the Deckhouse pod's status:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

   Use the following command to check the Deckhouse queue:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller queue list
   ```

1. Check if there are any pods with the Deckhouse EE registry address left in the cluster:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.status.phase=="Running" or .status.phase=="Pending" or .status.phase=="PodInitializing") | select(.spec.containers[] | select(.image | contains("deckhouse.io/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

1. Purge temporary files, `NodeGroupConfiguration` resource, and variables:

   ```shell
   kubectl delete ngc containerd-se-config.sh se-set-sha-images.sh
   kubectl delete pod se-image
   kubectl apply -f - <<EOF
       apiVersion: deckhouse.io/v1alpha1
       kind: NodeGroupConfiguration
       metadata:
         name: del-temp-config.sh
       spec:
         nodeGroups:
         - '*'
         bundles:
         - '*'
         weight: 90
         content: |
           if [ -f /etc/containerd/conf.d/se-registry.toml ]; then
             rm -f /etc/containerd/conf.d/se-registry.toml
           fi
           if [ -f /etc/containerd/conf.d/se-sandbox.toml ]; then
             rm -f /etc/containerd/conf.d/se-sandbox.toml
           fi
   EOF
   ```

   Once bashible synchronization is complete (you can track the synchronization status on nodes via the `UPTODATE` value of the NodeGroup), delete the NodeGroupConfiguration resource you created earlier:

   ```shell
   kubectl delete ngc del-temp-config.sh
   ```

### How do I get access to Deckhouse controller in multimaster cluster?

In clusters with multiple master nodes Deckhouse runs in high availability mode (in several instances). To access the active Deckhouse controller, you can use the following command (as an example of the command `deckhouse-controller queue list`):

```shell
kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
```

## How do I upgrade the Kubernetes version in a cluster?

To upgrade the Kubernetes version in a cluster change the [kubernetesVersion](installing/configuration.html#clusterconfiguration-kubernetesversion) parameter in the [ClusterConfiguration](installing/configuration.html#clusterconfiguration) structure by making the following steps:

1. Run the command:

   ```shell
   kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit cluster-configuration
   ```

1. Change the `kubernetesVersion` field.
1. Save the changes. Cluster nodes will start updating sequentially.
1. Wait for the update to finish. You can track the progress of the update using the `kubectl get no` command. The update is completed when the new version appears in the command's output for each cluster node in the `VERSION` column.

### How do I run Deckhouse on a particular node?

Set the `nodeSelector` [parameter](modules/deckhouse/configuration.html) of the `deckhouse` module and avoid setting `tolerations`. The necessary values will be assigned to the `tolerations` parameter automatically.

{% alert level="warning" %}
Use only nodes with the **CloudStatic** or **Static** type to run Deckhouse. Also, avoid using a `NodeGroup` containing only one node to run Deckhouse.
{% endalert %}

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
