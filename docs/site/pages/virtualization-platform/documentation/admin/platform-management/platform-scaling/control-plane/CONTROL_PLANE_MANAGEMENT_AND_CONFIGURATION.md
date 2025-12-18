---
title: "General management and configuration of the control plane"
permalink: en/virtualization-platform/documentation/admin/platform-management/platform-scaling/control-plane/control-plane-management-and-configuration.html
---

## Main features

Deckhouse Virtualization Platform (DVP) manages control plane components using the [`control-plane-manager`](/modules/control-plane-manager/) module, which runs on all master nodes (nodes with the label `node-role.kubernetes.io/control-plane: ""`).

The control plane management functionality includes:

- Managing certificates required for the control plane to function, including their renewal and issuance when the configuration changes. Secure configuration is maintained automatically, with the ability to quickly add additional SANs for secure access to the Kubernetes API.

- Component configuration. DVP generates all necessary configurations and manifests (kube-apiserver, etcd, etc.), reducing the risk of human error.

- Upgrade/downgrade of components. DVP supports consistent version upgrades or downgrades of the control plane, helping maintain version uniformity across the cluster.

- Managing the etcd cluster configuration and its members. DVP scales master nodes and performs migrations between single-master and multi-master modes.

- Configuring kubeconfig. DVP generates an up-to-date configuration file (with `cluster-admin` privileges), handles automatic renewal and updates, and creates a `symlink` for the `root` user.

> Some parameters affecting control plane behavior are taken from the [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) resource.

## Enabling, disabling, and configuring the module

### Enabling / disabling

You can enable or disable the [`control-plane-manager`](/modules/control-plane-manager/) module in the following ways:

1. Create (or modify) the ModuleConfig/control-plane-manager resource by setting `spec.enabled` to `true` or `false`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     enabled: true
   ```

1. Using the command:

   ```bash
   d8 system module enable control-plane-manager
   ```

   or to disable:

   ```bash
   d8 system module disable control-plane-manager
   ```  
  
1. Via the [Deckhouse web interface](/modules/console/):

   - Go to the “Deckhouse → Modules” section.
   - Find the `control-plane-manager` module and click on it.
   - Toggle the “Module enabled” switch.

### Configuration

To configure the module, use the ModuleConfig/control-plane-manager resource and specify the required parameters in `spec.settings`.

Example with the schema version, enabled module, and a few settings:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  enabled: true
  settings:
    apiserver:
      bindToWildcard: true
      certSANs:
      - bakery.infra
      - devs.infra
      loadBalancer: {}
```

### Checking DVP status and queues

How to verify that [`control-plane-manager`](/modules/control-plane-manager/) is running correctly and is not in a pending state, and how to check active DVP tasks (queues):

1. Make sure the module is enabled:

   ```shell
   d8 k get modules control-plane-manager
   ```

1. Check the status of `control-plane-manager` pods (they run in the `kube-system` namespace and have the label `app=d8-control-plane-manager`):

   ```shell
   d8 k -n kube-system get pods -l app=d8-control-plane-manager -o wide
   ```

   Ensure that all pods are in the `Running` or `Completed` state.

1. Verify that master nodes are in the `Ready` state:

   ```shell
   d8 k get nodes -l node-role.kubernetes.io/control-plane
   ```

   To view detailed information:

   ```shell
   d8 k describe node <node-name>
   ```

1. Get the list of queues and active tasks:

   ```shell
   d8 system queue list
   ```

   Example output:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

{% alert level="warning" %}
Before performing heavy operations (e.g., transitioning from single-master to multi-master or upgrading the Kubernetes version), it is recommended to wait until all tasks in the queues are completed.
{% endalert %}

## Certificate management

In DVP, the [`control-plane-manager`](/modules/control-plane-manager/) module is responsible for issuing and renewing all SSL certificates for control plane components. It manages:

1. **Server certificates** for kube-apiserver and etcd, stored in the `d8-pki` secret (namespace: `kube-system`):
   - Kubernetes root CA (`ca.crt`, `ca.key`).
   - etcd root CA (`etcd/ca.crt`, `etcd/ca.key`).
   - RSA certificate and key for signing Service Accounts (`sa.pub`, `sa.key`).
   - Root CA for extension API servers (`front-proxy-ca.key`, `front-proxy-ca.crt`).

1. **Client certificates** required for mutual communication between control plane components (e.g., `apiserver.crt`, `apiserver-etcd-client.crt`, etc.). These files are stored only on the nodes. If any changes occur (e.g., new SANs are added), certificates are automatically reissued, and the kubeconfig is synchronized.

### PKI management

DVP also manages the Public Key Infrastructure (PKI) used for encryption and authentication throughout the Kubernetes cluster:

- PKI for control plane components (kube-apiserver, kube-controller-manager, kube-scheduler, etc.).
- PKI for the etcd cluster (etcd certificates and inter-node communication).

DVP assumes control of this PKI after the initial cluster installation and once its pods are running. As a result, all key issuance, renewal, and rotation operations (both for control plane and etcd) are performed automatically and centrally, without requiring manual intervention.

### Additional SANs and auto-update

Deckhouse simplifies the addition of new Subject Alternative Names (SANs) for the Kubernetes API endpoint: you only need to specify them in the configuration. After any SAN change, the module automatically regenerates the certificates and updates the kubeconfig.

To add additional SANs (extra DNS names or IP addresses) for the Kubernetes API, specify the new SANs in the `spec.settings.apiserver.certSANs` field of your ModuleConfig/control-plane-manager resource.

DVP will automatically generate new certificates and update all required configuration files (including kubeconfig).

### Kubelet certificate rotation

In Deckhouse Virtualization Platform, kubelet certificate rotation is automatic.
The `--tls-cert-file` and `--tls-private-key-file` parameters for kubelet are not set directly. Instead, a dynamic TLS certificate mechanism is used: kubelet applies the client certificate located at `/var/lib/kubelet/pki/kubelet-client-current.pem`, which it uses to request a new client or server certificate (file `/var/lib/kubelet/pki/kubelet-server-current.pem`) from kube-apiserver. Also, the CIS benchmark `AVD-KCV-0088` and `AVD-KCV-0089` checks, which track whether the `--tls-cert-file` and `--tls-private-key-file` arguments were passed to kubelet, are disabled in the `operator-trivy` module.

Features of kubelet certificate rotation in Deckhouse Virtualization Platform:

- By default, kubelet generates its own keys in the `/var/lib/kubelet/pki/` directory and, if necessary, independently requests certificate renewal from kube-apiserver.
- lifetime of certificates is 1 year (8760 hours). When there are between 5 and 10% of the time remaining before expiration (the exact value is randomly selected from this range), kubelet automatically initiates a request for a new certificate. For more details, see the [official documentation Kubernetes](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/#bootstrap-initialization). If necessary, lifetime of certificates can be changed using `--cluster-signing-duration` parameter in `/etc/kubernetes/manifests/kube-controller-manager.yaml` manifest. However, in order for kubelet to obtain and install a new certificate before the current one expires, it is recommended to set the validity period of certificates to at least 1 hour.
- If the kubelet fails to renew the certificate before it expires, it will lose the ability to make requests to the kube-apiserver and, accordingly, renew certificates. As a result, the node will be marked as `NotReady` and automatically recreated.

### Manual renewal of control plane certificates

If the master nodes were offline for an extended period (e.g., the servers were shut down), some control plane certificates may become outdated. In such cases, automatic renewal will not occur upon reboot — manual intervention is required.

To manually renew the control plane certificates, use the `kubeadm` utility on each master node:

1. Locate the `kubeadm` binary on the master node and create a symbolic link:

   ```shell
   ln -s  $(find /var/lib/containerd  -name kubeadm -type f -executable -print) /usr/bin/kubeadm
   ```

1. Execute the following command:

   ```shell
   kubeadm certs renew all
   ```

   This command will regenerate the necessary certificates (for kube-apiserver, kube-controller-manager, kube-scheduler, etcd, and others).

## Speeding up pod restarts after losing connection to a node

By default, if a node does not report its status within 40 seconds, it is marked as `Unreachable`. Then, after another 5 minutes, the pods on that node begin to be restarted on other nodes. As a result, the total application downtime can reach approximately 6 minutes.

In specific cases where an application cannot run in multiple instances, there is a way to reduce the downtime period:

1. Reduce the time before a node is marked as `Unreachable` by configuring the `nodeMonitorGracePeriodSeconds` parameter.
1. Set a shorter timeout for evicting pods from the unreachable node using the `failedNodePodEvictionTimeoutSeconds` parameter.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    nodeMonitorGracePeriodSeconds: 10
    failedNodePodEvictionTimeoutSeconds: 50
```

{% alert level="warning" %}
The shorter the timeouts, the more frequently system components check node status and plan pod rescheduling. This increases the load on the control plane, so choose values that match your requirements for high availability and performance.
{% endalert %}

## Forcibly disabling IPv6 on cluster nodes

Internal communication between Deckhouse cluster components is performed via IPv4 protocol. However, at the operating system level of the cluster nodes, IPv6 is usually active by default. This leads to automatic assignment of IPv6 addresses to all network interfaces, including Pod interfaces. This results in unwanted network traffic - for example, redundant DNS queries like `AAAAA` - which can affect performance and make debugging network communications more difficult.

To correctly disable IPv6 at the node level in a Deckhouse-managed cluster, it is sufficient to set the necessary parameters via the [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) resource:

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
