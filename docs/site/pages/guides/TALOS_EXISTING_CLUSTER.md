---
title: Installing DKP in an existing Talos cluster
permalink: en/guides/talos-existing-cluster.html
description: A guide to installing Deckhouse Kubernetes Platform in an existing Talos cluster.
lang: en
layout: sidebar-guides
---

This guide applies to an existing, operational Kubernetes cluster based on [Talos Linux](https://www.siderolabs.com/talos-linux): the control plane is running, worker nodes have joined the cluster, a CNI is installed, and the Kubernetes API is accessible with `kubectl`.

Deckhouse is installed on top of the existing Kubernetes cluster. This example uses [Community Edition](/products/kubernetes-platform/documentation/v1/reference/revision-comparison.html), [release channel](/products/kubernetes-platform/documentation/v1/reference/release-channels.html) `EarlyAccess`, and [module bundle](/products/kubernetes-platform/documentation/v1/admin/configuration/#module-bundles) `Managed`.

In this setup:

- Talos continues to manage the operating system, MachineConfig, kubelet, containerd, etcd, the control plane, Kubernetes PKI, and Kubernetes updates;
- the existing CNI continues to provide Pod networking;
- the external provisioner or the user continues to create and delete machines;
- Deckhouse installs and updates platform modules but does not manage Talos or the node lifecycle.

{% alert level="warning" %}
The module bundle is selected during installation and cannot be changed afterwards. You cannot install `Managed` and then switch it to `Minimal` or `Default` with a regular patch.
{% endalert %}

## Prerequisites

The following tools and access are required on the computer from which the installation will be performed:

- Docker;
- `kubectl`;
- `yq` for validating YAML;
- an administrative kubeconfig for the Talos cluster;
- access to the Kubernetes API;
- HTTPS access to `registry.deckhouse.ru`;
- `talosctl` and `talosconfig` if an administrative kubeconfig has not yet been obtained.

The cluster nodes also require HTTPS access to `registry.deckhouse.ru`.

SSH access to the cluster nodes is not required: the installation is performed through the Kubernetes API.

Before installation, it is recommended to create an etcd snapshot using Talos and save the original talosconfig and kubeconfig.

## Setting the working paths

Create a separate directory for the installation files and change to it:

```bash
mkdir -p "$PWD/talos-deckhouse-install"
cd "$PWD/talos-deckhouse-install"
```

All subsequent commands assume that this remains the current directory. Set the paths once:

```bash
TALOSCONFIG="$PWD/talosconfig"
ADMIN_KUBECONFIG="$PWD/kubeconfig-admin"
INSTALLER_KUBECONFIG="$PWD/kubeconfig-installer"
CONFIG_FILE="$PWD/config.yml"
```

The files are used as follows:

| Variable | Purpose |
| --- | --- |
| `TALOSCONFIG` | `talosctl` configuration for accessing the Talos API |
| `ADMIN_KUBECONFIG` | Administrative Kubernetes kubeconfig for running `kubectl` on the computer |
| `INSTALLER_KUBECONFIG` | Portable copy of the administrative kubeconfig for the Docker container |
| `CONFIG_FILE` | Deckhouse installation configuration |

If you open a new terminal, return to this directory and set the configuration environment variables again.

## Preparing an administrative kubeconfig

### If you already have a kubeconfig

Copy it to the working directory:

```bash
cp /path/to/existing/admin-kubeconfig "$ADMIN_KUBECONFIG"
chmod 600 "$ADMIN_KUBECONFIG"
```

{% alert level="info" %}
This refers to the kubeconfig for `kubectl`, not the talosconfig for `talosctl`.
{% endalert %}

### If you need to obtain a kubeconfig through Talos

First, copy the existing talosconfig to the working directory:

```bash
cp /path/to/existing/talosconfig "$TALOSCONFIG"
chmod 600 "$TALOSCONFIG"
```

Specify the address of a control-plane node:

```bash
CONTROL_PLANE_ADDRESS=<CONTROL_PLANE_IP_OR_DNS>
```

Obtain an administrative kubeconfig:

```bash
talosctl kubeconfig "$ADMIN_KUBECONFIG" \
  --talosconfig="$TALOSCONFIG" \
  --nodes="$CONTROL_PLANE_ADDRESS" \
  --merge=false

chmod 600 "$ADMIN_KUBECONFIG"
```

By default, `talosctl` uses the Talos API endpoints from the current talosconfig context. If a different endpoint is required, add:

```text
--endpoints=<TALOS_API_ENDPOINT>
```

Use `--force` only when you intentionally want to overwrite an existing file.

### Verifying permissions

Check how Kubernetes identifies the user:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" auth whoami
```

Installation requires stable administrative access. A Talos administrative kubeconfig usually uses the `system:masters` group. Using an OIDC user kubeconfig for the installation is not recommended, since after Deckhouse authorization modules are enabled, that user's access to system namespaces may change.

Verify the required permissions:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  auth can-i '*' '*' --all-namespaces

kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  auth can-i create customresourcedefinitions.apiextensions.k8s.io

kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  auth can-i create clusterroles.rbac.authorization.k8s.io
```

All three commands must return `yes`.

## Verifying the existing cluster

Check the nodes:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get nodes -o wide
```

All nodes must be in the `Ready` state.

Check the Kubernetes API:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get --raw='/readyz?verbose'
```

The response must end with `readyz check passed`.

Check the system Pods:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n kube-system get pods -o wide
```

Before installing Deckhouse, the following components must already be running:

- CNI;
- CoreDNS;
- kube-proxy, if it is used by the selected network setup;
- control-plane components.

Also make sure that the Kubernetes version is [supported](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#kubernetes) by the selected DKP version.

## Verifying component ownership

Deckhouse must not manage the same components as Talos or an external provisioner.

The table below lists the components and their owners after installation:

| Component | Owner after installation |
| --- | --- |
| Talos OS and MachineConfig | Talos |
| etcd and the Kubernetes control plane | Talos |
| Kubernetes PKI | Talos |
| kubelet and containerd | Talos |
| CNI | The existing external CNI |
| CoreDNS and kube-proxy | The existing cluster |
| Machine creation and deletion | The external provisioner or the user |
| Platform modules | Deckhouse |

In this setup, the following Deckhouse modules must remain disabled:

- `control-plane-manager`;
- `node-manager`;
- `terraform-manager`;
- `cni-cilium`;
- `kube-dns`;
- `kube-proxy`;
- cloud provider modules;
- `registry-packages-proxy`.

{% alert level="warning" %}
If Cilium is already installed in the Talos cluster, do not enable the `cni-cilium` module: two operators must not manage the same CNI at the same time.
{% endalert %}

The `Managed` bundle includes `ingress-nginx`, `cert-manager`, `local-path-provisioner`, VPA, monitoring, and authorization modules. Before installation, check whether external equivalents are already present in the cluster:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get storageclass
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get ingressclass
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get deployments -A
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get crd
```

If a component is already installed, choose a single owner before proceeding. Do not run two `ingress-controller` instances, two `cert-manager` installations, or two VPA installations at the same time.

## Preparing a kubeconfig

The installer runs inside Docker. It requires a portable kubeconfig that does not reference certificate and key files available only on the user's computer.

Create a portable copy:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  config view \
  --raw \
  --flatten \
  --minify \
  > "$INSTALLER_KUBECONFIG"

chmod 600 "$INSTALLER_KUBECONFIG"
```

This command does not create new certificates. The `--flatten` option reads the CA, client certificate, and key from the paths specified in the source kubeconfig and embeds them in the new file.

Verify the copy:

```bash
kubectl --kubeconfig="$INSTALLER_KUBECONFIG" auth whoami

kubectl --kubeconfig="$INSTALLER_KUBECONFIG" \
  auth can-i '*' '*' --all-namespaces
```

The second command must return `yes`.

Check the Kubernetes API address:

```bash
kubectl --kubeconfig="$INSTALLER_KUBECONFIG" \
  config view --minify \
  -o jsonpath='{.clusters[0].cluster.server}{"\n"}'
```

This address must be reachable from the Docker container. A reachable Kubernetes API address, a VPN address, or a load balancer address is preferred.

If the address is `https://127.0.0.1:6443`, the installer cannot use it directly: inside the container, `127.0.0.1` refers to the container itself. Make the Kubernetes API accessible from the container before continuing with the installation.

## Creating `config.yml`

Create the `$CONFIG_FILE` file with the following content and replace `example.com` with your domain:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Managed
    releaseChannel: EarlyAccess
    logLevel: Info
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings:
    modules:
      publicDomainTemplate: "%s.example.com"
```

`publicDomainTemplate` must not match the Kubernetes `clusterDomain`.

If the nodes have custom taints and Deckhouse components must run on them, add the corresponding values to `global.spec.settings.modules.placement.customTolerationKeys`. Do not add an example taint unless it exists in the cluster.

Validate the file:

```bash
yq eval-all '.' "$CONFIG_FILE" >/dev/null && echo "YAML OK"
grep -n $'\t' "$CONFIG_FILE"
```

The first command must print `YAML OK`; the second command must not print anything.

## Running the installer

Check that the files exist:

```bash
ls -l "$CONFIG_FILE" "$INSTALLER_KUBECONFIG"
```

Run the installer:

```bash
docker run --pull=always -it \
  -v "$CONFIG_FILE:/config.yml:ro" \
  -v "$INSTALLER_KUBECONFIG:/kubeconfig:ro" \
  registry.deckhouse.ru/deckhouse/ce/install:early-access \
  bash
```

Note the `ce` edition and the `early-access` release channel specified in the installer image reference.

Inside the installer container, run:

```bash
dhctl bootstrap-phase install-deckhouse \
  --kubeconfig=/kubeconfig \
  --config=/config.yml
```

Do not close the terminal until the bootstrap process completes. Installation can take anywhere from 5 to 30 minutes.

## Monitoring the installation

In a separate terminal, change to the same working directory and define the variables from the "Setting the working paths" section again. Run the command:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n d8-system get deployment,replicaset,pods -w
```

If the deckhouse Pod is not created, check the events:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n d8-system get events \
  --sort-by=.metadata.creationTimestamp
```

Errors such as `ImagePullBackOff`, `ErrImagePull`, `401 Unauthorized`, or `403 Forbidden` usually indicate a problem with the registry address, registry access, DNS, or routing.

To diagnose a specific Pod, run:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n <NAMESPACE> describe pod <POD_NAME>
```

## Verifying the installation

Wait for the main Deployment to become ready:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n d8-system rollout status deployment/deckhouse \
  --timeout=10m

kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n d8-system get deployment,pods -o wide
```

Check the modules:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get modules -o wide
```

Enabled modules are expected to have `PHASE: Ready`, `ENABLED: True`, and `READY: True`.

The `Module` status alone is not sufficient: a module may be `Ready` even if one of its workloads was not created or is restarting. Check the actual resources:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  get deployment,statefulset,daemonset -A
```

For each DaemonSet, the `DESIRED`, `CURRENT`, and `READY` values must match. For Deployments and StatefulSets, the expected number of replicas must be in the `Ready` state.

Find any other problematic Pods:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  get pods -A \
  --field-selector='status.phase!=Running,status.phase!=Succeeded'
```

Check recent warnings:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  get events -A \
  --field-selector=type=Warning \
  --sort-by=.metadata.creationTimestamp
```

An old warning does not necessarily indicate a current problem. Consider the event timestamp, repetition count, and the current state of the related resource.

### Verifying that Deckhouse has not taken over Talos-managed components

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" get modules \
  control-plane-manager \
  node-manager \
  terraform-manager \
  cni-cilium \
  kube-dns \
  kube-proxy \
  registry-packages-proxy \
  -o wide
```

In this setup, these modules must have `ENABLED: False`.

Check the original cluster components again:

```bash
kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  -n kube-system get pods -o wide

kubectl --kubeconfig="$ADMIN_KUBECONFIG" \
  get nodes -o wide
```

All Talos nodes must remain `Ready`, and the original CNI, CoreDNS, kube-proxy, and control-plane components must continue to run.

## Successful installation criteria

The installation is considered successful when all of the following conditions are met:

- all Talos nodes remain `Ready`;
- the original CNI, CoreDNS, and kube-proxy continue to run;
- the Deckhouse Deployment is ready;
- enabled modules have `READY: True`;
- the actual module Deployments, StatefulSets, and DaemonSets are ready;
- Deckhouse lifecycle modules remain disabled;
- administrative access through the Talos administrative kubeconfig is preserved.

## Useful links

- [Installing Deckhouse in an existing cluster](https://deckhouse.io/products/kubernetes-platform/gs/existing/step2.html)
- [Deckhouse module configuration](https://deckhouse.io/modules/deckhouse/configuration.html)
- [Bundles and module management](https://deckhouse.io/products/kubernetes-platform/documentation/v1/admin/configuration/)
- [Patching Talos MachineConfig](https://docs.siderolabs.com/talos/v1.13/configure-your-talos-cluster/system-configuration/patching)
