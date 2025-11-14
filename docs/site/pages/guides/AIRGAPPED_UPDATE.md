---
title: Updating DKP in air-gapped environment
permalink: en/guides/airgapped-update.html
description: A guide for updating Deckhouse Kubernetes Platform in an air-gapped environment.
lang: en
layout: sidebar-guides
---

{% alert level="warning" %}
This guide is intended for DKP Enterprise Edition,
but the mechanism is the same [for other editions](../documentation/v1/reference/revision-comparison.html).
{% endalert %}

{% alert level="info" %}
This guide was tested on [d8 v0.17.1](../documentation/v1/cli/d8/).

The guide uses the third-party utility [crane](https://github.com/google/go-containerregistry?tab=readme-ov-file#crane) to analyze the container registry.
Install it first following the [official instructions](https://github.com/google/go-containerregistry/blob/main/cmd/crane/README.md#installation).
{% endalert %}

## Platform update mechanism using release channels

Deckhouse Kubernetes Platform (DKP) updates are based on [release channels](../documentation/v1/architecture/updating.html#release-channels).
You can check which release channel is configured for your DKP installation
in the [`deckhouse`](/modules/deckhouse/configuration.html) ModuleConfig by running the following command:

```bash
d8 k get mc deckhouse -o jsonpath='{.spec.settings.releaseChannel}'
```

Example output:

```console
Stable
```

Technically, a DKP update works as follows: the registry contains an image with a fixed name `release-channel`
and a tag corresponding to the release channel.
This tag points to an image of a specific DKP version (when a new version is released,
the tag is updated to point to the new image).

Let's examine the contents of a DKP Enterprise Edition image with the `alpha` release channel.

To do that, run the following command:

```bash
crane export registry.deckhouse.io/deckhouse/ee/release-channel:alpha | tar -tf -
```

The output contains a list of files and directories in the image:

```console
changelog.yaml
version.json
.werf
.werf/stapel
.werf/tmp
.werf/tmp/ssh-auth-sock
```

The image contains two key files:

- `changelog.yaml`: Describes changes.
- `version.json`: Contains data about the release canary deployment (`canary`), requirements (`requirements`), disruptions (`disruptions`) ([deprecated field](../documentation/v1/reference/api/cr.html#deckhouserelease-v1alpha1-spec-disruptions)), and the release version itself in the `version` field.

  To view the contents of `version.json`, run the following command:

  ```bash
  crane export registry.deckhouse.io/deckhouse/ee/release-channel:alpha | tar -xOf - version.json | jq
  ```

  Example contents of `version.json`:

  ```json
  {
    "canary": {
      "alpha": {
        "enabled": true,
        "waves": 2,
        "interval": "5m"
      },
      "beta": {
        "enabled": false,
        "waves": 1,
        "interval": "1m"
      },
      "early-access": {
        "enabled": true,
        "waves": 6,
        "interval": "30m"
      },
      "stable": {
        "enabled": true,
        "waves": 6,
        "interval": "30m"
      },
      "rock-solid": {
        "enabled": false,
        "waves": 5,
        "interval": "5m"
      }
    },
    "requirements": {
      "k8s": "1.29",
      "disabledModules": "delivery,l2-load-balancer,ceph-csi",
      "migratedModules": "",
      "autoK8sVersion": "1.31",
      "ingressNginx": "1.9",
      "nodesMinimalOSVersionUbuntu": "18.04",
      "nodesMinimalOSVersionDebian": "10",
      "istioMinimalVersion": "1.19",
      "metallbHasStandardConfiguration": "true",
      "unmetCloudConditions": "true",
      "nodesMinimalLinuxKernelVersion": "5.8.0"
    },
    "disruptions": {
      "1.36": [
        "ingressNginx"
      ]
    },
    "version": "v1.71.5"
  }
  ```

When the `version` field changes in `version.json` in the registry, DKP applies a new release in the cluster:
a `deckhouserelease` object is created and the update process begins.

{% alert level="info" %}
If the `deckhouse` module is set to manual update mode (via the [`settings.update.mode`](/modules/deckhouse/configuration.html#parameters-update-mode) field),
you need to manually approve the new version before it is applied.
{% endalert %}

If there is a gap between the minor version running in the cluster and the one in the `release-channel` image,
DKP will automatically attempt to reconstruct intermediate `deckhouserelease` objects to perform a sequential update.

{% alert level="warning" %}
Note that DKP can't be updated non-sequentially by skipping minor releases (patch releases are not affected).
Minor releases often include migrations that must be applied in order.
These migrations may be removed over time.
Skipping minor releases may result in:

- Leftover "garbage"
- Cluster malfunction due to missed migrations
{% endalert %}

## Module update mechanism

Modules have a similar update mechanism, but their release cycle is decoupled from the platform and fully independent.

The cluster contains [ModuleSource](../documentation/v1/reference/api/cr.html#modulesource) resources tracked by DKP,
which determine the list of available modules.

To see from which repository modules will be installed, run the following command:

```bash
d8 k get ms deckhouse -o jsonpath='{.spec.registry.repo}'
```

Example output:

```console
registry.deckhouse.io/deckhouse/ee/modules
```

You can view repository contents with the following command:

```bash
crane ls registry.deckhouse.io/deckhouse/ee/modules
```

Example output:

```console
commander-agent
console
csi-ceph
csi-nfs
observability
operator-postgres
pod-reloader
prompp
sds-local-volume
sds-node-configurator
sds-replicated-volume
secrets-store-integration
snapshot-controller
stronghold
virtualization
```

As an example, let's look at the contents of the `console` module image.
The registry contains an image with the fixed name `release` and a tag for the channel,
pointing to a specific version of the `console` module (when a new module version is released, this tag is updated).

To view the contents of this image, use the following command:

```bash
crane export registry.deckhouse.io/deckhouse/ee/modules/console/release:alpha | tar -tf -
```

Example output:

```console
changelog.yaml
version.json
```

Similarly to the DKP image, the module image contains the `changelog.yaml` and `version.json` files.

To view the contents of `version.json`, run the following command:

```bash
crane export registry.deckhouse.io/deckhouse/ee/modules/console/release:alpha | tar -xOf - version.json | jq
```

Example contents of `version.json`:

```json
{
  "version": "v1.39.4"
}
```

The `version` field contains the module version.
When it changes, DKP applies a new release (a `modulerelease` object is created and the update process begins).

{% alert level="warning" %}
Note that modules can't be updated non-sequentially by skipping minor releases (patch releases are not affected).
Minor releases often include migrations that must be applied in order.
These migrations may be removed over time.
Skipping minor releases may result in:

- Leftover "garbage"
- Cluster malfunction due to missed migrations
{% endalert %}

If required minor versions are missing,
DKP will report an error that may look as follows: `minor version is greater than deployed $version by one`.

If there is a gap between the minor version running in the cluster and the one in the `release` image,
DKP will automatically attempt to reconstruct intermediate `modulerelease` objects to perform a sequential update.

## Vulnerability scanner database update mechanism

{% alert level="warning" %}
Available in DKP Enterprise Edition.
{% endalert %}

Vulnerability databases are updated every 6 hours.
The `operator-trivy` module in the cluster downloads them from the registry once during this period.

Vulnerability database images in [DKP EE](/modules/operator-trivy/) have fixed names and tags and are available at:

```bash
registry.deckhouse.io/deckhouse/ee/security/trivy-db:2
registry.deckhouse.io/deckhouse/ee/security/trivy-java-db:1
registry.deckhouse.io/deckhouse/ee/security/trivy-checks:0
registry.deckhouse.io/deckhouse/ee/security/trivy-bdu:1
```

To configure periodic updates of vulnerability database images, run the command in the following format:

```bash
d8 mirror pull --source='registry.deckhouse.io/deckhouse/ee' --license='YOUR_LICENSE_TOKEN' --no-platform --no-modules $(pwd)/d8-bundle-security-db && d8 mirror push $(pwd)/d8-bundle-security-db YOUR_PRIVATE_REGISTRY_HOSTNAME:5050/dkp/ee --registry-login='YOUR_REGISTRY_LOGIN' --registry-password='YOUR_REGISTRY_PASSWORD' --tls-skip-verify
```

## Example workflow for updating the platform, modules, and vulnerability databases

To update DKP, its modules, and vulnerability databases to the latest versions in an air-gapped environment,
download the latest patch releases of all required platform minor versions and modules, then upload them to your registry.

Running `d8 mirror pull --source='registry.deckhouse.io/deckhouse/ee' --license='YOUR_LICENSE_TOKEN' $(pwd)/d8-bundle`
downloads all release channel images and all modules (over 30 in DKP EE).
This results in a very large `d8-bundle` (at the time of writing, the volume of the `d8-bundle` directory is more than 50 GB).

To avoid this, download only the images relevant to your version following these guidelines:

1. Get the DKP version in your cluster using the following command:

   ```bash
   d8 k -n d8-system get deployment deckhouse -o json | jq -r '.metadata.annotations | {"core.deckhouse.io/edition","core.deckhouse.io/version"}'
   ```

   Example output:

   ```console
   {
     "core.deckhouse.io/edition": "EE",
     "core.deckhouse.io/version": "v1.68.13"
   }
   ```

1. Get the list of modules deployed in the cluster:

   ```bash
   d8 k get mr | grep Deployed
   ```

   Example output:

   ```console
   commander-agent-v1.2.4             Deployed                     13d
   console-v1.35.1                    Deployed                     7d4h
   ```

   Add this list to the `d8 mirror pull` command as flags: `--include-module='commander-agent@v1.2.4' --include-module='console@v1.35.1'`

   Alternatively, use a one-liner:

   ```bash
   d8 k get mr -o json | jq -r '.items[] | select(.status.phase == "Deployed") | "--include-module='\''\(.spec.moduleName)@\(.spec.version)'\''"' | paste -sd " " -
   ```

1. Create the final command for pulling images with the obtained parameters:

   ```bash
   d8 mirror pull --source='registry.deckhouse.io/deckhouse/ee' --license='YOUR_LICENSE_TOKEN' --since-version='v1.68.13' --include-module='commander-agent@1.2.4' --include-module='console@1.35.1' $(pwd)/d8-bundle
   ```

   > If you have already set up periodic downloading and pushing of vulnerability databases to your registry,
   > you can add the flag `--no-security-db` to skip them during this process.

   This will download the latest patch releases of all required platform minor versions and modules,
   starting from the latest patch versions up to the current ones on [release channels](https://releases.deckhouse.io/ee).

1. Push the artifacts to your registry using the following command:

   ```bash
   d8 mirror push $(pwd)/d8-bundle YOUR_PRIVATE_REGISTRY_HOSTNAME:5050/dkp/ee --registry-login='YOUR_REGISTRY_LOGIN' --registry-password='YOUR_REGISTRY_PASSWORD' --tls-skip-verify
   ```

1. Check the update status in the cluster by running the following commands:

   ```bash
   d8 k get deckhousereleases.deckhouse.io
   d8 k get modulereleases.deckhouse.io
   d8 system queue list
   ```

## Possible issues

### Release is suspended

When trying to download platform images with `d8 mirror pull d8-bundle/ --license='YOUR_LICENSE_KEY'`,
you may encounter the following error:

```console
Sep  9 00:10:57.145 INFO  ╔ Pull Deckhouse Kubernetes Platform
Sep  9 00:11:01.532 ERROR Pull Deckhouse Kubernetes Platform failed error="Find tags to mirror: Find versions to mirror: get stable release version from registry: Cannot mirror Deckhouse: source registry contains suspended release channel \"stable\", try again later"
Error: pull failed, see the log for details
```

This means that release deployment on a channel has been suspended.
This happens when a new version is pushed to the release channel but then something happened that paused its rollout.
The channel image is patched with a `suspend` flag.

Nevertheless, you can still download the platform version by specifying the `--deckhouse-tag` flag in `d8 mirror pull`.
For example:

```bash
d8 mirror pull d8-bundle/ --license='YOUR_LICENSE_KEY' --deckhouse-tag='v1.71.3'
```

Example output:

```console
Sep 16 12:56:25.074 INFO  ╔ Pull Deckhouse Kubernetes Platform
Sep 16 12:56:25.713 INFO  ║ Skipped releases lookup as tag "v1.71.3" is specifically requested with --deckhouse-tag
Sep 16 12:56:25.714 INFO  ║ Creating OCI Image Layouts
Sep 16 12:56:25.720 INFO  ║ Resolving tags
Sep 16 12:56:26.715 INFO  ║╔ Pull release channels and installers
Sep 16 12:56:26.716 INFO  ║║ Beginning to pull Deckhouse release channels information
Sep 16 12:56:26.717 INFO  ║║ [1 / 1] Pulling registry.deckhouse.io/deckhouse/ee/release-channel:v1.71.3
Sep 16 12:56:27.087 INFO  ║║ Deckhouse release channels are pulled!
Sep 16 12:56:27.087 INFO  ║║ Beginning to pull installers
Sep 16 12:56:27.087 INFO  ║║ [1 / 1] Pulling registry.deckhouse.io/deckhouse/ee/install:v1.71.3
...
```
