---
title: FAQ
permalink: en/deckhouse-faq.html
---

## How do I find out all Deckhouse parameters?

Deckhouse is configured using global settings, module settings, and various custom resources. Read more in the [documentation](./).

1. Display global Deckhouse settings:

   ```shell
   d8 k get mc global -o yaml
   ```

1. List the status of all modules (available for Deckhouse version 1.47+):

   ```shell
   d8 k get modules
   ```

1. Display the settings of the `user-authn` module configuration:

   ```shell
   d8 k get moduleconfigs user-authn -o yaml
   ```

## How do I find the documentation for the version installed?

The documentation for the Deckhouse version running in the cluster is available at `documentation.<cluster_domain>`, where `<cluster_domain>` is the DNS name that matches the template defined in the [modules.publicDomainTemplate](deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) parameter.

{% alert level="warning" %}
Documentation is available when the [`documentation`](modules/documentation/) module is enabled. It is enabled by default except the `Minimal` [bundle](modules/deckhouse/configuration.html#parameters-bundle).
{% endalert %}

## Deckhouse update

### How to find out in which mode the cluster is being updated?

You can view the cluster update mode in the [configuration](modules/deckhouse/configuration.html) of the `deckhouse` module. To do this, run the following command:

```shell
d8 k get mc deckhouse -oyaml
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
d8 k annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
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
d8 k get deckhouserelease
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
d8 k -n d8-system get pods -l app=deckhouse
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
   d8 k -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
   ```

1. [Collect debugging information](modules/deckhouse/faq.html#how-to-collect-debug-info) and contact technical support.
1. Ask for help from the [community](https://deckhouse.io/community/about.html).
{% endalert %}

### How do I know that a new version is available for the cluster?

As soon as a new version of Deckhouse appears on the release channel installed in the cluster:

- The alert `DeckhouseReleaseIsWaitingManualApproval` fires, if the cluster uses manual update mode (the [update.mode](modules/deckhouse/configuration.html#parameters-update-mode) parameter is set to `Manual`).
- There is a new custom resource [DeckhouseRelease](cr.html#deckhouserelease). Use the `d8 k get deckhousereleases` command, to view the list of releases. If the `DeckhouseRelease` is in the `Pending` state, the specified version has not yet been installed. Possible reasons why `DeckhouseRelease` may be in `Pending`:
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
d8 k get deckhousereleases
```

{% alert level="warning" %}
Starting from DKP 1.70 patch releases (e.g., an update from version `1.70.1` to version `1.70.2`) are installed taking into account the update windows. Prior to DKP 1.70, patch version updates ignore update windows settings and apply as soon as they are available.
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
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- getent ahosts registry.deckhouse.io
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
d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
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
d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
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
This feature is available in the following editions: BE, SE, SE+, EE.
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

1. Create a docker **proxy** repository (*Administration* -> *Repository* -> *Repositories*) pointing to the Deckhouse registry `https://registry.deckhouse.io`:
  ![Create docker proxy repository](images/registry/nexus/nexus-repository.png)

1. Fill in the fields on the Create page as follows:
   * `Name` must contain the name of the repository you created earlier, e.g., `d8-proxy`.
   * `Repository Connectors / HTTP` or `Repository Connectors / HTTPS` must contain a dedicated port for the created repository, e.g., `8123` or other.
   * `Remote storage` must be set to `https://registry.deckhouse.io/`.
   * You can disable `Auto blocking enabled` and `Not found cache enabled` for debugging purposes, otherwise they must be enabled.
   * `Maximum Metadata Age` must be set to `0`.
   * `Authentication` must be enabled if you plan to use a commercial edition of Deckhouse Kubernetes Platform, and the related fields must be set as follows:
     * `Authentication Type` must be set to `Username`.
     * `Username` must be set to `license-token`.
     * `Password` must contain your Deckhouse Kubernetes Platform license key.

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
  * Specify the `Access ID` and `Access Secret` (the Deckhouse Kubernetes Platform license key).

  ![Create a Registry](images/registry/harbor/harbor1.png)

* Create a new Project:
  * `Projects -> New Project`.
  * `Project Name` will be used in the URL. You can choose any name, for example, `d8s`.
  * `Access Level`: `Public`.
  * `Proxy Cache` — enable and choose the Registry, created in the previous step.

  ![Create a new Project](images/registry/harbor/harbor2.png)

Thus, Deckhouse images will be available at `https://your-harbor.com/d8s/deckhouse/ee:{d8s-version}`.

### How to generate a self-signed certificate?

When generating certificates manually, it is important to fill out all fields of the certificate request correctly to ensure that the final certificate is issued properly and can be validated across various services.  

It is important to follow these guidelines:

1. Specify domain names in the `SAN` (Subject Alternative Name) field.

   The `SAN` field is a more modern and commonly used method for specifying the domain names covered by the certificate.
   Some services no longer consider the `CN` (Common Name) field as the source for domain names.

2. Correctly fill out the `keyUsage`, `basicConstraints`, `extendedKeyUsage` fields, specifically:
   - `basicConstraints = CA:FALSE`  

     This field determines whether the certificate is an end-entity certificate or a certification authority (CA) certificate. CA certificates cannot be used as service certificates.

   - `keyUsage = digitalSignature, keyEncipherment`  

     The `keyUsage` field limits the permissible usage scenarios of this key:

     - `digitalSignature`: Allows the key to be used for signing digital messages and ensuring data integrity.
     - `keyEncipherment`: Allows the key to be used for encrypting other keys, which is necessary for secure data exchange using TLS (Transport Layer Security).

   - `extendedKeyUsage = serverAuth`  

     The `extendedKeyUsage` field specifies additional key usage scenarios required by specific protocols or applications:

     - `serverAuth`: Indicates that the certificate is intended for server use, authenticating the server to the client during the establishment of a secure connection.

It is also recommended to:

1. Issue the certificate for no more than 1 year (365 days).

   The validity period of the certificate affects its security. A one-year validity ensures the cryptographic methods remain current and allows for timely certificate updates in case of threats. Furthermore, some modern browsers now reject certificates with a validity period longer than 1 year.

2. Use robust cryptographic algorithms, such as elliptic curve algorithms (including `prime256v1`).

   Elliptic curve algorithms (ECC) provide a high level of security with a smaller key size compared to traditional methods like RSA. This makes the certificates more efficient in terms of performance and secure in the long term.

3. Do not specify domains in the `CN` (Common Name) field.
  
   Historically, the `CN` field was used to specify the primary domain name for which the certificate was issued. However, modern standards, such as [RFC 2818](https://datatracker.ietf.org/doc/html/rfc2818), recommend using the `SAN` (Subject Alternative Name) field for this purpose.
   If the certificate is intended for multiple domain names listed in the `SAN` field, specifying one of the domains additionally in `CN` can cause a validation error in some services when accessing domains not listed in `CN`.
   If non-domain-related information is specified in `CN` (for example, an identifier or service name), the certificate will also extend to these names, which could be exploited for malicious purposes.

#### Certificate generation example

To generate a certificate, we'll use the `openssl` utility.

1. Fill in the `cert.cnf` configuration file:

   ```ini
   [ req ]
   default_bits       = 2048
   default_md         = sha256
   prompt             = no
   distinguished_name = dn
   req_extensions     = req_ext

   [ dn ]
   C = GB
   ST = London
   L = London
   O = Example Company
   OU = IT Department
   # CN = Do not specify the CN field.

   [ req_ext ]
   subjectAltName = @alt_names

   [ alt_names ]
   # Specify all domain names.
   DNS.1 = example.co.uk
   DNS.2 = www.example.co.uk
   DNS.3 = api.example.co.uk
   # Specify IP addresses (if required).
   IP.1 = 192.0.2.1
   IP.2 = 192.0.4.1

   [ v3_ca ]
   basicConstraints = CA:FALSE
   keyUsage = digitalSignature, keyEncipherment
   extendedKeyUsage = serverAuth

   [ v3_req ]
   basicConstraints = CA:FALSE
   keyUsage = digitalSignature, keyEncipherment
   extendedKeyUsage = serverAuth
   subjectAltName = @alt_names

   # Elliptic curve parameters.
   [ ec_params ]
   name = prime256v1
   ```

2. Generate an elliptic curve key:

   ```shell
   openssl ecparam -genkey -name prime256v1 -noout -out ec_private_key.pem
   ```

3. Create a certificate signing request:

   ```shell
   openssl req -new -key ec_private_key.pem -out example.csr -config cert.cnf
   ```

4. Generate a self-signed certificate:

   ```shell
   openssl x509 -req -in example.csr -signkey ec_private_key.pem -out example.crt -days 365 -extensions v3_req -extfile cert.cnf
   ```

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
    - `NO_PROXY` — comma-separated list of hosts to exclude from proxying. Supported value formats include IP addresses (`1.2.3.4`), CIDR notations (`1.2.3.4/8`), domains, and the asterisk character (`*`). The IP addresses and domain names can also include a literal port number (`1.2.3.4:80`). The domain name matches that name and all the subdomains. The domain name with a leading `.` matches subdomains only. For example, `foo.com` matches `foo.com` and `bar.foo.com`; `.y.com` matches `x.y.com` but does not match `y.com`. A single asterisk `*` indicates that no proxying should be done;
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

### How do I switch a running Deckhouse cluster to use a third-party registry?

{% alert level="warning" %}
When using the [registry](modules/registry/) module, change the address and parameters of the registry in the [registry](modules/deckhouse/configuration.html#parameters-registry) section of the `deckhouse` module configuration. An example of configuration is provided in the [`registry`](modules/registry/examples.html) module documentation.
{% endalert %}

{% alert level="warning" %}
Using a registry other than `registry.deckhouse.io` is only available in a commercial edition of Deckhouse Kubernetes Platform.
{% endalert %}

To switch the Deckhouse cluster to using a third-party registry, follow these steps:

* Run `deckhouse-controller helper change-registry` inside the Deckhouse Pod with the new registry settings.
  * Example:

    ```shell
    d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee
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
    d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- bash -c "echo '$CA_CONTENT' > /tmp/ca.crt && deckhouse-controller helper change-registry --ca-file /tmp/ca.crt --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee"
    ```

  * To view the list of available keys of the `deckhouse-controller helper change-registry` command, run the following command:

    ```shell
    d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --help
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
  d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
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

### How do I use a proxy server?

{% alert level="warning" %}
This feature is available in the following editions: BE, SE, SE+, EE.
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

### How do I set up proxy variable autoloading in the CLI?

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
d8 platform edit cluster-configuration
```

After saving the changes, Deckhouse will bring the cluster configuration to the state according to the changed configuration. Depending on the size of the cluster, this may take some time.

### How do I change the configuration of a cloud provider in a cluster?

Cloud provider setting of a cloud of hybrid cluster are stored in the `<PROVIDER_NAME>ClusterConfiguration` structure, where `<PROVIDER_NAME>` — name/code of the cloud provider. E.g., for an OpenStack provider, the structure will be called [OpenStackClusterConfiguration]({% if site.mode == 'module' and site.d8Revision == 'CE' %}{{ site.urls[page.lang] }}/products/kubernetes-platform/documentation/v1/{% endif %}modules/cloud-provider-openstack/cluster_configuration.html).

Regardless of the cloud provider used, its settings can be changed using the following command:

```shell
d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

### How do I change the configuration of a static cluster?

Settings of a static cluster are stored in the [StaticClusterConfiguration](installing/configuration.html#staticclusterconfiguration) structure.

To change the settings of a static cluster, run the command:

```shell
d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit static-cluster-configuration
```

### How to switch Deckhouse edition to CE/BE/SE/SE+/EE?

{% alert level="warning" %}
- The functionality of this guide is validated for Deckhouse versions starting from `v1.70`. If your version is older, use the corresponding documentation.
- For commercial editions, you need a valid license key that supports the desired edition. If necessary, you can [request a temporary key](https://deckhouse.ru/products/enterprise_edition.html).
- The guide assumes the use of the public container registry address: `registry.deckhouse.io`. If you are using a different container registry address, modify the commands accordingly or refer to the [guide on switching Deckhouse to use a different registry](#how-do-i-configure-deckhouse-to-use-a-third-party-registry).
- The Deckhouse CE/BE/SE/SE+ editions do not support the cloud providers `dynamix`, `openstack`, `VCD`, and `vSphere` (vSphere is supported in SE+) and a number of modules. A detailed comparison is available in the [documentation](revision-comparison.html).
- All commands are executed on the master node of the existing cluster with `root` user.
{% endalert %}

#### How to switch using the registry module?

1. Make sure the cluster has been migrated to be managed by the [`registry` module](modules/registry/faq.html#how-to-migrate-to-the-registry-module).  
If the cluster is not managed by the `registry` module, proceed to the [instruction](#how-to-switch-without-using-the-registry-module).

1. Prepare variables for the license token and new edition name:

    > It is not necessary to fill the `NEW_EDITION` variable when switching to Deckhouse CE edition.
    > The `NEW_EDITION` variable should match your desired Deckhouse edition. For example, to switch to:
    - CE, the variable should be `ce`;
    - BE, the variable should be `be`;
    - SE, the variable should be `se`;
    - SE+, the variable should be `se-plus`;
    - EE, the variable should be `ee`.

    ```shell
    NEW_EDITION=<PUT_YOUR_EDITION_HERE>
    LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
    ```

1. Ensure the [Deckhouse queue](#how-to-check-the-job-queue-in-deckhouse) is empty and error-free.

1. Start a temporary pod for the new Deckhouse edition to obtain current digests and a list of modules:

   for the CE edition:

   ```shell
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   d8 k run $NEW_EDITION-image --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   for other editions:

   ```shell
   d8 k create secret docker-registry $NEW_EDITION-image-pull-secret \
    --docker-server=registry.deckhouse.ru \
    --docker-username=license-token \
    --docker-password=${LICENSE_TOKEN}

   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   d8 k run $NEW_EDITION-image \
    --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION \
    --overrides="{\"spec\": {\"imagePullSecrets\":[{\"name\": \"$NEW_EDITION-image-pull-secret\"}]}}" \
    --command sleep -- infinity
   ```

   Once the pod is in `Running` state, execute the following commands:

   ```shell
   NEW_EDITION_MODULES=$(d8 k exec $NEW_EDITION-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
   USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $NEW_EDITION_MODULES | tr ' ' '\n'))
   ```

1. Verify that the modules used in the cluster are supported in the desired edition. To see the list of modules not supported in the new edition and will be disabled:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Check the list to ensure the functionality of these modules is not in use in your cluster and you are ready to disable them.

   Disable the modules not supported by the new edition:

   ```shell
   echo $MODULES_WILL_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   Wait for the Deckhouse pod to reach `Ready` state and [ensure all tasks in the queue are completed](#how-to-check-the-job-queue-in-deckhouse).

1. Delete the created Secret and Pod:

   ```shell
   d8 k delete pod/$NEW_EDITION-image
   d8 k delete secret/$NEW_EDITION-image-pull-secret
   ```

1. Perform the switch to the new edition. To do this, specify the following parameter in the `deckhouse` ModuleConfig. For detailed configuration, refer to the [`deckhouse`](modules/deckhouse/) module documentation.

   ```yaml
   ---
   # Example for Direct mode
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Direct
         direct:
           # Relax mode is used to check for the presence of the current Deckhouse version in the specified registry.
           # This mode must be used to switch between editions.
           checkMode: Relax
           # Specify your value for <NEW_EDITION>.
           imagesRepo: registry.deckhouse.ru/deckhouse/<NEW_EDITION>
           scheme: HTTPS
           # Specify your value for <LICENSE_TOKEN>.
           # If switching to the CE edition, remove this parameter.
           license: <LICENSE_TOKEN>
   ---
   # Example for Unmanaged mode.
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           # Relax mode is used to check for the presence of the current Deckhouse version in the specified registry.
           # This mode must be used to switch between editions.
           checkMode: Relax
           # Specify your value for <NEW_EDITION>.
           imagesRepo: registry.deckhouse.ru/deckhouse/<NEW_EDITION>
           scheme: HTTPS
           # Specify your value for <LICENSE_TOKEN>.
           # If switching to the CE edition, remove this parameter.
           license: <LICENSE_TOKEN>
   ```

1. Wait for the registry to switch. To verify the switch progress, follow the [instruction](modules/registry/faq.html#how-to-check-the-registry-mode-switch-status).

   Example output:

   ```yaml
   conditions:
     - lastTransitionTime: "..."
       message: |-
         Mode: Relax
         registry.deckhouse.ru: all 1 items are checked
       reason: Ready
       status: "True"
       type: RegistryContainsRequiredImages
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. After the switch, remove the `checkMode: Relax` parameter from the `deckhouse` ModuleConfig to revert to the default check mode.  
Removing this parameter will trigger a check for the presence of critical components in the registry.

1. Wait for the check to complete by following the [instruction](modules/registry/faq.html#how-to-check-the-registry-mode-switch-status).

   Example output:

   ```yaml
   conditions:
     - lastTransitionTime: "..."
       message: |-
         Mode: Default
         registry.deckhouse.ru: all 155 items are checked
       reason: Ready
       status: "True"
       type: RegistryContainsRequiredImages
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. Check if there are any pods with the Deckhouse old edition address left in the cluster, where `<YOUR-PREVIOUS-EDITION>` your previous edition name:

   for Unmanaged mode:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.ru/deckhouse/<YOUR-PREVIOUS-EDITION>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   For other modes that use a fixed registry address.  
   This check does not take external modules into account:  

   ```shell
   # Get the list of valid digest values from the images_digests.json file inside Deckhouse.
   IMAGES_DIGESTS=$(d8 k -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- cat /deckhouse/modules/images_digests.json | jq -r '.[][]' | sort -u)

   # Check for Pods using Deckhouse images from `registry.d8-system.svc:5001/system/deckhouse`
   # with a digest that is NOT present in the list of valid digest values (IMAGES_DIGESTS).
   d8 k get pods -A -o json |
   jq -r --argjson digests "$(printf '%s\n' $IMAGES_DIGESTS | jq -R . | jq -s .)" '
     .items[]
     | {name: .metadata.name, namespace: .metadata.namespace, containers: .spec.containers}
     | select(.containers != null)
     | select(
         .containers[]
         | select(.image | test("registry.d8-system.svc:5001/system/deckhouse") and test("@sha256:"))
         | .image as $img
         | ($img | split("@") | last) as $digest
         | ($digest | IN($digests[]) | not)
       )
     | .namespace + "\t" + .name
   ' | sort -u
   ```

#### How to switch without using the registry module?  

1. Before you begin, disable the `registry` module by following [instruction](modules/registry/faq.html#how-to-migrate-back-from-the-registry-module).

1. Prepare variables for the license token and new edition name:

    > It is not necessary to fill the `NEW_EDITION` and `AUTH_STRING` variables when switching to Deckhouse CE edition.
    The `NEW_EDITION` variable should match your desired Deckhouse edition. For example, to switch to:
    - CE, the variable should be `ce`;
    - BE, the variable should be `be`;
    - SE, the variable should be `se`;
    - SE+, the variable should be `se-plus`;
    - EE, the variable should be `ee`.

    ```shell
    NEW_EDITION=<PUT_YOUR_EDITION_HERE>
    LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
    AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
    ```

1. Ensure the [Deckhouse queue](#how-to-check-the-job-queue-in-deckhouse) is empty and error-free.

1. Create a `NodeGroupConfiguration` resource for temporary authorization in `registry.deckhouse.io`:

   > Before creating a resource, refer to the section [How to add configuration for an additional registry](modules/node-manager/faq.html#how-to-add-configuration-for-an-additional-registry)
   >
   > Skip this step if switching to Deckhouse CE.

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-$NEW_EDITION-config.sh
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
       bb-sync-file /etc/containerd/conf.d/$NEW_EDITION-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry.deckhouse.io".auth]
               auth = "$AUTH_STRING"
       EOF_TOML
   EOF
   ```

   Wait for the `/etc/containerd/conf.d/$NEW_EDITION-registry.toml` file to appear on the nodes and for bashible synchronization to complete. To track the synchronization status, check the `UPTODATE` value (the number of nodes in this status should match the total number of nodes (`NODES`) in the group):

   ```shell
   d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Example output:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   Also, a message stating `Configuration is in sync, nothing to do` should appear in the systemd service log for bashible by executing the following command:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Annotate node master-ee-to-se-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master ee-to-se-0 bashible.sh[53407]: Successful annotate node master-ee-to-se-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-se-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Start a temporary pod for the new Deckhouse edition to obtain current digests and a list of modules:

   ```shell
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   d8 k run $NEW_EDITION-image --image=registry.deckhouse.io/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION --command sleep --infinity
   ```

1. Once the pod is in `Running` state, execute the following commands:

   ```shell
   NEW_EDITION_MODULES=$(d8 k exec $NEW_EDITION-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
   USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $NEW_EDITION_MODULES | tr ' ' '\n'))
   ```

1. Verify that the modules used in the cluster are supported in the desired edition. To see the list of modules not supported in the new edition and will be disabled:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Check the list to ensure the functionality of these modules is not in use in your cluster and you are ready to disable them.

   Disable the modules not supported by the new edition:

   ```shell
   echo $MODULES_WILL_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   Wait for the Deckhouse pod to reach `Ready` state and [ensure all tasks in the queue are completed](#how-to-check-the-job-queue-in-deckhouse).

1. Execute the `deckhouse-controller helper change-registry` command from the Deckhouse pod with the new edition parameters:

   To switch to BE/SE/SE+/EE editions:

   ```shell
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user=license-token --password=$LICENSE_TOKEN --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.io/deckhouse/$NEW_EDITION
   ```

   To switch to CE edition:

   ```shell
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.io/deckhouse/ce
   ```

1. Check if there are any pods with the Deckhouse old edition address left in the cluster, where `<YOUR-PREVIOUS-EDITION>` your previous edition name:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.io/deckhouse/<YOUR-PREVIOUS-EDITION>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

1. Delete temporary files, the `NodeGroupConfiguration` resource, and variables:

   > Skip this step if switching to Deckhouse CE.

   ```shell
   d8 k delete ngc containerd-$NEW_EDITION-config.sh
   d8 k delete pod $NEW_EDITION-image
   d8 k apply -f - <<EOF
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
           if [ -f /etc/containerd/conf.d/$NEW_EDITION-registry.toml ]; then
             rm -f /etc/containerd/conf.d/$NEW_EDITION-registry.toml
           fi
   EOF
   ```

   After the bashible synchronization completes (synchronization status on the nodes is shown by the `UPTODATE` value in NodeGroup), delete the created NodeGroupConfiguration resource:

   ```shell
   d8 k delete ngc del-temp-config.sh
   ```

### How to switch Deckhouse EE to CSE?

{% alert level="warning" %}
- The instructions assume the use of the public address of the container registry: `registry-cse.deckhouse.ru`. If you use a different address of the container registry, change the commands or use the [instructions for switching Deckhouse to use a third-party registry](#how-do-i-switch-a-running-deckhouse-cluster-to-use-a-third-party-registry).
- Deckhouse CSE does not support cloud clusters and some modules. For more information about supported modules, see the [comparison of editions](revision-comparison.html) page.
- Migration to Deckhouse CSE is only possible from Deckhouse EE 1.58, 1.64 or 1.67.
- Current versions of Deckhouse CSE: 1.58.2 for release 1.58, 1.64.1 for release 1.64 and 1.67.0 for release 1.67. These versions will need to be used later to specify the `DECKHOUSE_VERSION` variable.
- The transition is only supported between the same minor versions, for example, from Deckhouse EE 1.64 to Deckhouse CSE 1.64. The transition from EE 1.58 to CSE 1.67 will require an intermediate migration: first to EE 1.64, then to EE 1.67, and only then to CSE 1.67. Attempts to upgrade the version to several releases at once may lead to cluster inoperability.
- Deckhouse CSE 1.58 and 1.64 support Kubernetes version 1.27, Deckhouse CSE 1.67 supports Kubernetes versions 1.27 and 1.29.
- When switching to Deckhouse CSE, temporary unavailability of cluster components is possible.
  {% endalert %}

To switch a Deckhouse Enterprise Edition cluster to Certified Security Edition, follow these steps (all commands are executed on the cluster master node as a user with the `kubectl` context configured, or as the superuser):

#### How to switch using the registry module?

1. Make sure that the cluster has been switched to use the [`registry`](modules/registry/faq.html#how-to-migrate-to-the-registry-module) module. If the module is not used, go to the [instructions](#how-to-switch-without-using-registry-module).
1. Configure the cluster to use the desired Kubernetes version (information on versioning is provided in the [How to switch Deckhouse EE to CSE](#how-to-switch-deckhouse-ee-to-cse) section). To do this:
   1. Run the command:

      ```shell
      d8 platform edit cluster-configuration
      ```

   1. Change the `kubernetesVersion` parameter to the required value, for example, `"1.27"` (in quotation marks) for Kubernetes 1.27.
   1. Save the changes. The cluster nodes will start updating sequentially.
   1. Wait for the update to complete. You can track the update progress with the `d8 k get no` command. The update can be considered complete when the updated version appears in the `VERSION` column of each cluster node in the command output.

1. Prepare variables with license token:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   ```

1. Run a temporary pod of the new edition of Deckhouse to get the latest digests and a list of modules:

   ```shell
   d8 k create secret docker-registry cse-image-pull-secret \
    --docker-server=registry-cse.deckhouse.ru \
    --docker-username=license-token \
    --docker-password=${LICENSE_TOKEN}

   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   d8 k run cse-image \
    --image=registry-cse.deckhouse.ru/deckhouse/cse/install:$DECKHOUSE_VERSION \
    --overrides="{\"spec\": {\"imagePullSecrets\":[{\"name\": \"cse-image-pull-secret\"}]}}" \
    --command sleep -- infinity
   ```

   Once the pod has entered the `Running` status, run the following commands:

   ```shell
   CSE_MODULES=$(d8 k exec cse-image -- ls -l deckhouse/modules/ | awk {'print $9'} |grep -oP "\d.*-\w*" | cut -c5-)
   USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $CSE_MODULES | tr ' ' '\n'))
   ```

1. Make sure that the modules used in the cluster are supported in Deckhouse CSE.
   For example, Deckhouse CSE 1.58 and 1.64 do not have the `cert-manager` module. Therefore, before disabling the `cert-manager` module, it is necessary to switch the HTTPS mode of some components (for example, [`user-authn`](https://deckhouse.io/products/kubernetes-platform/documentation/v1.58/modules/user-authn/configuration.html#parameters-https-mode) or [prometheus](https://deckhouse.io/products/kubernetes-platform/documentation/v1.58/modules/prometheus/configuration.html#parameters-https-mode)) to alternative operating options, or change the [global parameter](deckhouse-configure-global.html#parameters-modules-https-mode) responsible for the HTTPS mode in the cluster.

   You can display a list of modules that are not supported in Deckhouse CSE and will be disabled using the following command:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Check the list and make sure that the functionality of the specified modules is not used in the cluster and you are ready to disable them.

   Disable modules not supported in Deckhouse CSE:

   ```shell
   echo $MODULES_WILL_DISABLE | 
     tr ' ' '\n' | awk {'print "d8 k -n d8-system exec deploy/deckhouse -- deckhouse-controller module disable",$1'} | bash
   ```

   Deckhouse CSE does not support the earlyOOM feature. Disable it using [setting](modules/node-manager/configuration.html#parameters-earlyoomenabled).

   Wait for the Deckhouse pod to become `Ready` and all queued tasks to complete.

   ```shell
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

   Check that disabled modules have moved to the `Disabled` state.

   ```shell
   d8 k get modules
   ```

1. Delete the created secret and pod:

   ```shell
   d8 k delete pod/cse-image
   d8 k delete secret/cse-image-pull-secret
   ```

1. Switch to the new edition. To do this, specify the following parameters in ModuleConfig `deckhouse` (for detailed settings, see the [`deckhouse`](modules/deckhouse/) module configuration):

   ```yaml
   ---
   # Example for Direct mode.
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Direct
         direct:
           # Relax mode is used to check if the current version of Deckhouse is in the specified registry.
           # To switch between editions, you must use this registry check mode.
           checkMode: Relax
           imagesRepo: registry-cse.deckhouse.ru/deckhouse/cse
           scheme: HTTPS
           # Specify your <LICENSE_TOKEN> parameter.
           license: <LICENSE_TOKEN>
   ---
   # Example for Unmanaged mode.
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           # Relax mode is used to check if the current version of Deckhouse is in the specified registry.
           # To switch between editions, you must use this check mode.
           checkMode: Relax
           imagesRepo: registry-cse.deckhouse.ru/deckhouse/cse
           scheme: HTTPS
           # Specify your <LICENSE_TOKEN> parameter.
           license: <LICENSE_TOKEN>
   ```

1. Wait for the registry to switch. To check if the switch has been completed, use the [instructions](modules/registry/faq.html#how-to-check-the-registry-mode-switch-status).

   Example output:

   ```yaml
   conditions:
     - lastTransitionTime: "..."
       message: |-
         Mode: Relax
         registry-cse.deckhouse.ru: all 1 items are checked
       reason: Ready
       status: "True"
       type: RegistryContainsRequiredImages
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. After switching, remove the `checkMode: Relax` parameter from the `deckhouse` ModuleConfig to enable the default check. Removing it will start checking for critical components in the registry.

1. Wait until the check is completed. The registry mode switching status can be obtained using [instructions](modules/registry/faq.html#how-to-check-the-registry-mode-switch-status).

   Example output:

   ```yaml
   conditions:
     - lastTransitionTime: "..."
       message: |-
         Mode: Default
         registry-cse.deckhouse.ru: all 155 items are checked
       reason: Ready
       status: "True"
       type: RegistryContainsRequiredImages
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. Check if there are any pods left in the cluster with the registry address for Deckhouse EE:

   For Unmanaged mode:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   For other modes using a fixed address (this check does not take into account external modules):

   ```shell
   # Get a list of current digests from the images_digests.json file inside Deckhouse.
   IMAGES_DIGESTS=$(d8 k -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- cat /deckhouse/modules/images_digests.json | jq -r '.[][]' | sort -u)

   # Check if there are Pods using Deckhouse images at `registry.d8-system.svc:5001/system/deckhouse`
   # with a digest that is not in the list of current digests from IMAGES_DIGESTS.
   d8 k get pods -A -o json |
   jq -r --argjson digests "$(printf '%s\n' $IMAGES_DIGESTS | jq -R . | jq -s .)" '
     .items[]
     | {name: .metadata.name, namespace: .metadata.namespace, containers: .spec.containers}
     | select(.containers != null)
     | select(
         .containers[]
         | select(.image | test("registry.d8-system.svc:5001/system/deckhouse") and test("@sha256:"))
         | .image as $img
         | ($img | split("@") | last) as $digest
         | ($digest | IN($digests[]) | not)
       )
     | .namespace + "\t" + .name
   ' | sort -u
   ```

   If the output contains `chrony` module pods, re-enable this module (in Deckhouse CSE this module is disabled by default):

   ```shell
   d8 k -n d8-system exec deploy/deckhouse -- deckhouse-controller module enable chrony
   ```

#### How to switch without using the registry module?

1. Before you begin, disable the `registry` module using the [instructions](modules/registry/faq.html#how-to-migrate-back-from-the-registry-module).

1. Configure the cluster to use the required Kubernetes version (information on versioning is provided in the [instruction](#how-to-switch-deckhouse-ee-to-cse)). To do this:
  1. Run the command:

     ```shell
     d8 platform edit cluster-configuration
     ```

  1. Change the `kubernetesVersion` parameter to the desired value, for example `"1.27"` (in quotes) for Kubernetes 1.27.
  1. Save the changes. The cluster nodes will begin to be updated sequentially.
  1. Wait for the update to complete. You can track the update progress using the `d8 k get no` command. The update is complete when the updated version appears in the `VERSION` column of each cluster node in the command output.

1. Prepare variables with the license token and create a NodeGroupConfiguration for transient authorization in `registry-cse.deckhouse.ru`:

   > Before creating a resource, please read the section [How to add a configuration for an additional registry](modules/node-manager/faq.html#how-to-add-configuration-for-an-additional-registry)

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
   d8 k apply -f - <<EOF
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-cse-config.sh
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
       bb-sync-file /etc/containerd/conf.d/cse-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry]
             [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
           [plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry-cse.deckhouse.ru"]
             endpoint = ["https://registry-cse.deckhouse.ru"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry-cse.deckhouse.ru".auth]
               auth = "$AUTH_STRING"
       EOF_TOML
   EOF
   ```

   Wait for synchronization to complete and for the `/etc/containerd/conf.d/cse-registry.toml` file to appear on the nodes.

   The synchronization status can be tracked by the `UPTODATE` value (the displayed number of nodes in this status should match the total number of nodes (`NODES`) in the group):

   ```shell
   d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Example output:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   The bashible systemd service log should show the message `Configuration is in sync, nothing to do` as a result of running the following command:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 bashible.sh[53407]: Successful annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Run the following commands to start a temporary Deckhouse CSE pod to get up-to-date digests and a list of modules:

   ```shell
   DECKHOUSE_VERSION=v<ВЕРСИЯ_DECKHOUSE_CSE>
   # For example, DECKHOUSE_VERSION=v1.58.2.
   d8 k run cse-image --image=registry-cse.deckhouse.ru/deckhouse/cse/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   Once the pod has entered the `Running` status, run the following commands:

   ```shell
   CSE_SANDBOX_IMAGE=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | grep pause | grep -oE 'sha256:\w*')
   CSE_K8S_API_PROXY=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | grep kubernetesApiProxy | grep -oE 'sha256:\w*')
   CSE_MODULES=$(d8 k exec cse-image -- ls -l deckhouse/modules/ | awk {'print $9'} |grep -oP "\d.*-\w*" | cut -c5-)
   USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $CSE_MODULES | tr ' ' '\n'))
   CSE_DECKHOUSE_KUBE_RBAC_PROXY=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   ```

   > An additional command that is only needed when switching to Deckhouse CSE version 1.64:
   >
   > ```shell
   > CSE_DECKHOUSE_INIT_CONTAINER=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   > ```

1. Make sure that the modules used in the cluster are supported by Deckhouse CSE.
   For example, Deckhouse CSE 1.58 and 1.64 do not have the `cert-manager` module. Therefore, before disabling the `cert-manager` module, it is necessary to switch the HTTPS mode of some components (for example, [user-authn](https://deckhouse.io/products/kubernetes-platform/documentation/v1.58/modules/user-authn/configuration.html#parameters-https-mode) or [prometheus](https://deckhouse.io/products/kubernetes-platform/documentation/v1.58/modules/prometheus/configuration.html#parameters-https-mode)) to alternative operating options, or change the [global parameter](deckhouse-configure-global.html#parameters-modules-https-mode) responsible for the HTTPS mode in the cluster.

   You can display a list of modules that are not supported in Deckhouse CSE and will be disabled using the following command:
   
   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Check the list and make sure that the functionality of the specified modules is not used in the cluster and you are ready to disable them.

   Disable modules not supported in Deckhouse CSE:

   ```shell
   echo $MODULES_WILL_DISABLE | 
     tr ' ' '\n' | awk {'print "d8 k -n d8-system exec deploy/deckhouse -- deckhouse-controller module disable",$1'} | bash
   ```

   Deckhouse CSE does not support the earlyOOM feature. Disable it using [setting](modules/node-manager/configuration.html#parameters-earlyoomenabled).

   Wait until the Deckhouse pod becomes `Ready` and all queued tasks are completed.

   ```shell
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

   Check that disabled modules have moved to the `Disabled` state.

   ```shell
   d8 k get modules
   ```

1. Create a NodeGroupConfiguration:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: cse-set-sha-images.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 50
     content: |
        _on_containerd_config_changed() {
          bb-flag-set containerd-need-restart
        }
        bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

        bb-sync-file /etc/containerd/conf.d/cse-sandbox.toml - containerd-config-file-changed << "EOF_TOML"
        [plugins]
          [plugins."io.containerd.grpc.v1.cri"]
            sandbox_image = "registry-cse.deckhouse.ru/deckhouse/cse@$CSE_SANDBOX_IMAGE"
        EOF_TOML

        sed -i 's|image: .*|image: registry-cse.deckhouse.ru/deckhouse/cse@$CSE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
        sed -i 's|crictl pull .*|crictl pull registry-cse.deckhouse.ru/deckhouse/cse@$CSE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
   EOF
   ```

   Wait for bashible to complete synchronization on all nodes.

   The synchronization status can be tracked by the `UPTODATE` status value (the displayed number of nodes in this status should match the total number of nodes (`NODES`) in the group):

   ```shell
   d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   The bashible systemd service log on the nodes should show the message `Configuration is in sync, nothing to do` as a result of running the following command:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 bashible.sh[53407]: Successful annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Update the Deckhouse CSE registry access secret by running the following command:

   ```shell
   d8 k -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry-cse.deckhouse.ru\": { \"username\": \"license-token\", \"password\": \"$LICENSE_TOKEN\", \"auth\": \"$AUTH_STRING\" }}}" \
     --from-literal="address"=registry-cse.deckhouse.ru \
     --from-literal="path"=/deckhouse/cse \
     --from-literal="scheme"=https \
     --type=kubernetes.io/dockerconfigjson \
     --dry-run='client' \
     -o yaml | d8 k -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- d8 k replace -f -
   ```

1. Change the Deckhouse image to the Deckhouse CSE image:

   Command for Deckhouse CSE version 1.58:

   ```shell
   d8 k -n d8-system set image deployment/deckhouse kube-rbac-proxy=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry-cse.deckhouse.ru/deckhouse/cse:$DECKHOUSE_VERSION
   ```

   Command for Deckhouse CSE version 1.64 and 1.67:

   ```shell
   d8 k -n d8-system set image deployment/deckhouse init-downloaded-modules=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry-cse.deckhouse.ru/deckhouse/cse:$DECKHOUSE_VERSION
   ```

1. Wait for the Deckhouse pod to become `Ready` and [all tasks in the queue have completed](#how-to-check-the-job-queue-in-deckhouse). If an `ImagePullBackOff` error occurs during the process, wait for the pod to automatically restart.

   View Deckhouse pod status:

   ```shell
   d8 k -n d8-system get po -l app=deckhouse
   ```

   Check the status of the Deckhouse queue:

   ```shell
   d8 k -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller queue list
   ```

1. Check if there are any pods left in the cluster with the registry address for Deckhouse EE:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   If the output contains `chrony` module pods, re-enable this module (in Deckhouse CSE this module is disabled by default):

   ```shell
   d8 k -n d8-system exec deploy/deckhouse -- deckhouse-controller module enable chrony
   ```

1. Clean up temporary files, `NodeGroupConfiguration` resource and variables:

   ```shell
   rm /tmp/cse-deckhouse-registry.yaml

   d8 k delete ngc containerd-cse-config.sh cse-set-sha-images.sh

   d8 k delete pod cse-image

   d8 k apply -f - <<EOF
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
       if [ -f /etc/containerd/conf.d/cse-registry.toml ]; then
         rm -f /etc/containerd/conf.d/cse-registry.toml
       fi
       if [ -f /etc/containerd/conf.d/cse-sandbox.toml ]; then
         rm -f /etc/containerd/conf.d/cse-sandbox.toml
       fi
   EOF
   ```

   After synchronization (the synchronization status on the nodes can be tracked by the `UPTODATE` value of the NodeGroup), delete the created NodeGroupConfiguration resource:

   ```shell
   d8 k delete ngc del-temp-config.sh
   ```

### How do I get access to Deckhouse controller in multimaster cluster?

In clusters with multiple master nodes Deckhouse runs in high availability mode (in several instances). To access the active Deckhouse controller, you can use the following command (as an example of the command `deckhouse-controller queue list`):

```shell
d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
```

### How do I upgrade the Kubernetes version in a cluster?

To upgrade the Kubernetes version in a cluster change the [kubernetesVersion](installing/configuration.html#clusterconfiguration-kubernetesversion) parameter in the [ClusterConfiguration](installing/configuration.html#clusterconfiguration) structure by making the following steps:

1. Run the command:

   ```shell
   d8 platform edit cluster-configuration
   ```

1. Change the `kubernetesVersion` field.
1. Save the changes. Cluster nodes will start updating sequentially.
1. Wait for the update to finish. You can track the progress of the update using the `d8 k get no` command. The update is completed when the new version appears in the command's output for each cluster node in the `VERSION` column.

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

### How do I force IPv6 to be disabled on Deckhouse cluster nodes?

> Internal communication between Deckhouse cluster components is performed via IPv4 protocol. However, at the operating system level of the cluster nodes, IPv6 is usually active by default. This leads to automatic assignment of IPv6 addresses to all network interfaces, including Pod interfaces. This results in unwanted network traffic - for example, redundant DNS queries like `AAAAA` - which can affect performance and make debugging network communications more difficult.

To correctly disable IPv6 at the node level in a Deckhouse-managed cluster, it is sufficient to set the necessary parameters via the [NodeGroupConfiguration](./modules/node-manager/cr.html#nodegroupconfiguration) resource:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: disable-ipv6.sh
spec:
  nodeGroups:
  - '*'
  bundles:
  - '*'
  weight: 50
  content: |
    GRUB_FILE_PATH="/etc/default/grub"
    
    if ! grep -q "ipv6.disable" "$GRUB_FILE_PATH"; then
      sed -E -e 's/^(GRUB_CMDLINE_LINUX_DEFAULT="[^"]*)"/\1 ipv6.disable=1"/' -i "$GRUB_FILE_PATH"
      update-grub
      
      bb-flag-set reboot
    fi
```

{% alert level="warning" %}
After applying the resource, the GRUB settings will be updated and the cluster nodes will begin a sequential reboot to apply the changes.
{% endalert %}

### How do I change container runtime to containerd v2 on nodes?

You can migrate to containerd v2 in one of the following ways:

* By specifying the value `ContainerdV2` for the [`defaultCRI`](./installing/configuration.html#clusterconfiguration-defaultcri) parameter in the general cluster parameters. In this case, the container runtime will be changed in all node groups, unless where explicitly defined using the [`spec.cri.type`](./modules/node-manager/cr.html#nodegroup-v1-spec-cri-type) parameter.
* By specifying the value `ContainerdV2` for the [`spec.cri.type`](./modules/node-manager/cr.html#nodegroup-v1-spec-cri-type) parameter for a specific node group.

{% alert level="info" %}
Migration to containerd v2 is possible if the following conditions are met:

* Nodes meet the requirements described [in general cluster parameters](./installing/configuration.html#clusterconfiguration-defaultcri).
* The server has no custom configurations in `/etc/containerd/conf.d` ([example custom configuration](./modules/node-manager/faq.html#how-to-use-containerd-with-nvidia-gpu-support)).
{% endalert %}

Migrating to containerd v2 clears the `/var/lib/containerd` folder. For containerd, the `/etc/containerd/conf.d` folder is used. For containerd v2, `/etc/containerd/conf2.d` is used.
