---
title: Installing Deckhouse Platform in an existing Talos cluster
permalink: en/guides/talos-existing-cluster.html
description: A guide to installing Deckhouse Platform in an existing Talos cluster.
lang: en
layout: sidebar-guides
relatedLinks:
  - title: "Installing Deckhouse Platform in an existing cluster"
    url: /products/kubernetes-platform/gs/existing/step3.html
  - title: "deckhouse module configuration"
    url: /modules/deckhouse/configuration.html
  - title: "Bundles and module management"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/
  - title: "Patching Talos MachineConfig"
    url: "https://docs.siderolabs.com/talos/v1.13/configure-your-talos-cluster/system-configuration/patching"
---

This guide applies to an existing, operational Talos cluster: the control plane is running, worker nodes have joined the cluster, a CNI is installed, and the Kubernetes API is accessible with `d8 k`.

Deckhouse Platform (DP) is installed on top of the existing Kubernetes cluster in existing-cluster mode. This example uses Deckhouse Platform Open, the [`EarlyAccess` release channel](/modules/deckhouse/configuration.html#parameters-releasechannel), and the [`Managed` bundle](/modules/deckhouse/configuration.html#parameters-bundle).

In this setup:

- Talos continues to manage the operating system, MachineConfig, kubelet, containerd, etcd, the control plane, Kubernetes PKI, and Kubernetes updates.
- The existing CNI continues to provide Pod networking.
- The external provisioner or the user continues to create and delete machines.
- DP installs and updates platform modules but does not manage Talos or the node lifecycle.

{% alert level="warning" %}
The `bundle` value is selected during installation and cannot be changed afterwards. You cannot install `Managed` and then switch it to `Minimal` or `Default` with a regular patch.
{% endalert %}

## 1. Prerequisites

The following tools and access are required on the computer from which the installation will be performed:

- Docker
- `d8`
- `yq` for validating YAML
- An administrative Kubernetes kubeconfig for the Talos cluster
- Access to the Kubernetes API
- HTTPS access to `registry.deckhouse.ru` from both the computer and the cluster nodes
- `talosctl` and `talosconfig` if an administrative Kubernetes kubeconfig has not yet been obtained

SSH access to Talos nodes is not required: the installer communicates with the cluster through the Kubernetes API.

Before installation, it is recommended to create an etcd snapshot using Talos and save the original `talosconfig` and Kubernetes kubeconfig.

## 2. Set the working paths

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
| `ADMIN_KUBECONFIG` | Administrative Kubernetes kubeconfig for running `d8 k` on the computer |
| `INSTALLER_KUBECONFIG` | Portable copy of the administrative kubeconfig for the Docker container |
| `CONFIG_FILE` | DP installation configuration |

If you open a new terminal, return to this directory and repeat the block that defines the four variables.

## 3. Prepare an administrative Kubernetes kubeconfig

Choose how to obtain the administrative kubeconfig depending on whether you already have one.

### If you already have a kubeconfig

Copy it to the working directory:

```bash
cp <ADMIN_KUBECONFIG_PATH> "$ADMIN_KUBECONFIG"
chmod 600 "$ADMIN_KUBECONFIG"
```

This must be a Kubernetes kubeconfig for `d8 k`, not a `talosconfig` file for `talosctl`.

### If you need to obtain a kubeconfig through Talos

First, copy the existing `talosconfig` to the working directory:

```bash
cp <TALOSCONFIG_PATH> "$TALOSCONFIG"
chmod 600 "$TALOSCONFIG"
```

Specify the address of a control-plane node:

```bash
CONTROL_PLANE_ADDRESS=<CONTROL_PLANE_ADDRESS>
```

Obtain an administrative Kubernetes kubeconfig:

```bash
talosctl kubeconfig "$ADMIN_KUBECONFIG" \
  --talosconfig="$TALOSCONFIG" \
  --nodes="$CONTROL_PLANE_ADDRESS" \
  --merge=false

chmod 600 "$ADMIN_KUBECONFIG"
```

By default, `talosctl` uses the Talos API endpoints from the current `talosconfig` context. If a different endpoint is required, add:

```text
--endpoints=<TALOS_API_ENDPOINT>
```

Use `--force` only when you intentionally want to overwrite an existing file.

### Verify permissions

Check which identity Kubernetes sees:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" auth whoami
```

Installation requires stable administrative access. A Talos administrative kubeconfig usually uses the `system:masters` group. Using an OIDC user kubeconfig for the installer is not recommended: after the DP `user-authz` module is enabled, that user's access to system namespaces may change.

Verify the required permissions:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  auth can-i '*' '*' --all-namespaces

d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  auth can-i create customresourcedefinitions.apiextensions.k8s.io

d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  auth can-i create clusterroles.rbac.authorization.k8s.io
```

All three commands must return `yes`.

## 4. Verify the existing cluster

Check the nodes:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get nodes -o wide
```

All nodes must be in the `Ready` state.

Check the Kubernetes API:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get --raw='/readyz?verbose'
```

The response must end with `readyz check passed`.

Check the system Pods:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  -n kube-system get pods -o wide
```

Before installing DP, the following components must already be running:

- CNI
- CoreDNS
- kube-proxy, if it is used by the selected network setup
- Control-plane components

Also make sure that the Kubernetes version is supported by the selected DP version.

## 5. Verify component ownership

DP must not manage the same components as Talos or an external provisioner.

| Component | Owner after installation |
| --- | --- |
| Talos OS and MachineConfig | Talos |
| etcd and the Kubernetes control plane | Talos |
| Kubernetes PKI | Talos |
| kubelet and containerd | Talos |
| CNI | The existing external CNI |
| CoreDNS and kube-proxy, if used | The existing cluster |
| Machine creation and deletion | The external provisioner or the user |
| Platform modules | DP |

The following DP modules must remain disabled:

- `control-plane-manager`
- `node-manager`
- `terraform-manager`
- `cni-cilium`
- `kube-dns`
- `kube-proxy`
- `cloud-provider-*` modules
- `registry-packages-proxy`

If Cilium is already installed in the Talos cluster, do not enable the DP `cni-cilium` module: two operators must not manage the same CNI at the same time.

The `Managed` bundle includes ingress, cert-manager, local-path-provisioner, VPA, monitoring, and the `user-authz` module. Before installation, check whether external equivalents are already present in the cluster:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get storageclass
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get ingressclass
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get deployments -A
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get crd
```

If a component is already installed, choose a single owner before proceeding. Do not run two ingress controllers, two cert-manager installations, or two VPA installations at the same time.
If an external solution remains responsible for the component, explicitly disable the corresponding DP module using a ModuleConfig with `spec.enabled: false`. If DP is to manage the component, disable or remove the external counterpart before installation.

## 6. Prepare a kubeconfig for the installer container

The installer runs inside Docker. It requires a portable kubeconfig that does not reference certificate and key files available only on the user's computer.

Create a portable copy:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
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
d8 k --kubeconfig="$INSTALLER_KUBECONFIG" auth whoami

d8 k --kubeconfig="$INSTALLER_KUBECONFIG" \
  auth can-i '*' '*' --all-namespaces
```

The second command must return `yes`.

Check the Kubernetes API address:

```bash
d8 k --kubeconfig="$INSTALLER_KUBECONFIG" \
  config view --minify \
  -o jsonpath='{.clusters[0].cluster.server}{"\n"}'
```

This address must be reachable from the Docker container. A reachable Kubernetes API address, a VPN address, or a load balancer address is preferred.

If the address is `https://127.0.0.1:6443`, the installer cannot use it directly: inside the container, `127.0.0.1` refers to the container itself. Make the Kubernetes API accessible from the container before continuing with the installation.

## 7. Create the configuration file

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

The domain in [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) must not match or be a subdomain of the domain specified in [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain). Before using the template, configure DNS services in both the networks where cluster nodes are located and the networks from which clients access the platform service web interfaces.

If the nodes have custom taints and DP components must run on them, add the corresponding values to [`global.spec.settings.modules.placement.customTolerationKeys`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-placement-customtolerationkeys). Do not add an example taint unless it exists in the cluster.

Validate the file:

```bash
yq eval-all '.' "$CONFIG_FILE" >/dev/null && echo "YAML OK"
grep -n $'\t' "$CONFIG_FILE"
```

The first command must print `YAML OK`; the second command must not print anything.

## 8. Run the Deckhouse Platform Open installer

The installer tag must match the `releaseChannel` in the configuration. The `early-access` tag is used for `EarlyAccess`.

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

Inside the installer container, run:

```bash
dhctl bootstrap-phase install-deckhouse \
  --kubeconfig=/kubeconfig \
  --config=/config.yml
```

Do not close the terminal until the bootstrap process completes. Installation usually takes between 5 and 30 minutes.

## 9. Monitor the installation

In a separate terminal, change to the same working directory and define the variables from section 2 again. Then monitor DP:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  -n d8-system get deployment,replicaset,pods -w
```

If a Pod is not created, check the events:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  -n d8-system get events \
  --sort-by=.metadata.creationTimestamp
```

Errors such as `ImagePullBackOff`, `ErrImagePull`, `401 Unauthorized`, or `403 Forbidden` usually indicate a problem with the registry address, registry access, DNS, or routing.

To diagnose a specific Pod, run:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  -n <NAMESPACE> describe pod <POD_NAME>
```

## 10. Verify the installation

Wait for the main Deployment to become ready:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  -n d8-system rollout status deployment/deckhouse \
  --timeout=10m

d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  -n d8-system get deployment,pods -o wide
```

Check the modules:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get modules -o wide
```

Enabled modules are expected to have `PHASE: Ready`, `ENABLED: True`, and `READY: True`.

The Module status alone is not sufficient: a module may be `Ready` even if one of its workloads was not created or is restarting. Check the actual resources:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  get deployment,statefulset,daemonset -A
```

For each DaemonSet, the `DESIRED`, `CURRENT`, and `READY` values must match. For Deployments and StatefulSets, the expected number of replicas must be ready.

Find Pods that are not in the `Running` or `Succeeded` phase:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  get pods -A \
  --field-selector='status.phase!=Running,status.phase!=Succeeded'
```

Check recent warnings:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  get events -A \
  --field-selector=type=Warning \
  --sort-by=.metadata.creationTimestamp
```

An old warning does not necessarily indicate a current problem. Consider the event timestamp, repetition count, and the current state of the related resource.

### Verify that DP does not manage Talos components

Verify that DP modules that can manage Talos components remain disabled.

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get modules \
  control-plane-manager \
  node-manager \
  terraform-manager \
  cni-cilium \
  kube-dns \
  kube-proxy \
  registry-packages-proxy \
  -o wide
```

All listed modules must have `ENABLED: False`.

Check the cloud provider modules:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" get modules -o wide \
  | grep -E '(^NAME|^cloud-provider-)'
```

All `cloud-provider-*` modules found by the command must have `ENABLED: False`.

Check the original cluster components again:

```bash
d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  -n kube-system get pods -o wide

d8 k --kubeconfig="$ADMIN_KUBECONFIG" \
  get nodes -o wide
```

All Talos nodes must remain `Ready`. The original CNI, CoreDNS, and control-plane components must continue to run; kube-proxy must also continue to run if it was used before the DP installation.

## 11. Successful installation criteria

The installation is considered successful when all of the following conditions are met:

- All Talos nodes remain `Ready`.
- The original CNI and CoreDNS continue to run; kube-proxy also continues to run if it was used before the DP installation.
- Deployment `deckhouse` is ready.
- Enabled modules have `READY: True`.
- The actual module Deployments, StatefulSets, and DaemonSets are ready.
- DP lifecycle modules remain disabled.
- Administrative access through the Talos administrative kubeconfig is preserved.

