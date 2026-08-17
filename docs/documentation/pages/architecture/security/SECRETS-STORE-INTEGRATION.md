---
title: Secrets-store-integration module
permalink: en/architecture/security/secrets-store-integration.html
search: vault, secrets
description: Architecture of the secrets-store-integration module in Deckhouse Kubernetes Platform.
---

The [`secrets-store-integration`](/modules/secrets-store-integration/) module
delivers secrets to applications in Deckhouse Kubernetes Platform (DKP)
from an external store compatible with the [HashiCorp Vault](https://github.com/hashicorp/vault) API.

The module provides the following capabilities:

- Delivers secrets to pods as mounted files or environment variables
  without storing them in etcd.
- Automatically connects to an internal
  [Deckhouse Stronghold](/products/stronghold/documentation/) instance
  without manual configuration (`DiscoverLocalStronghold` mode).
- Works with any Vault-compatible secrets store in `Manual` mode.
- Automatically refreshes mounted secrets every two minutes
  when the value in the store changes.
- Injects an entrypoint for applications
  that cannot be modified to read secrets directly from the store.
- Delivers Base64-encoded binary secrets (for example, JKS keystores or Kerberos keytab files)
  with automatic decoding.

The operating mode (`Manual` or `DiscoverLocalStronghold`) is specified by the
[`settings.connectionConfiguration`](/modules/secrets-store-integration/configuration.html#parameters-connectionconfiguration)
module parameter of the [ModuleConfig](../../reference/api/cr.html#moduleconfig) custom resource.

The module works with the following custom resources:

- SecretProviderClass: Describes which secrets to deliver to a pod and from which external store.
  The resource specification also defines connection parameters for the secrets source
  and path mappings in the container.
- SecretProviderClassPodStatus: Contains the status of the secret mounting process in a pod
  and diagnostic information.
- [SecretsStoreImport](/modules/secrets-store-integration/alpha/cr.html#secretsstoreimport):
  Stores the mapping of secrets between a Vault-compatible store and files in containers.

For more details about the module, refer to the [corresponding documentation section](/modules/secrets-store-integration/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

- The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
- Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`secrets-store-integration`](/modules/secrets-store-integration/) module
and its interactions with other DKP components
are shown in the following diagram:

![Secrets-store-integration module architecture](../../images/architecture/security/c4-l2-secrets-store-integration.svg)

## Module components

The `secrets-store-integration` module consists of the following components:

1. **Csi-secrets-store** (DaemonSet): Runs on all cluster nodes
   and implements the [Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec/blob/master/spec.md) standard
   to deliver secrets to a pod as mounted files.

   Csi-secrets-store performs the following actions:

   - Registers the ephemeral CSI driver `secrets-store.csi.deckhouse.io` in [kubelet](../kubernetes-and-scheduling/kubelet.html).
   - Interacts with vault-csi-provider to retrieve data from the secrets store.
   - Mounts secrets as files in a pod.
   - Rotates secrets.
   - Watches the Pod and CSIDriver resources and the SecretProviderClass custom resource.
   - Works with the SecretProviderClassPodStatus custom resource.

   The component includes the following containers:

   - **injector-puller**: Init container that performs a one-time launch
     of the injector binary (an executable that securely injects secrets
     from an external store into the application environment inside the pod)
     from the `secrets-store-integration/env-injector` service image.
     This one-time launch is necessary to preload the image onto every cluster node.
     The webhook component uses this image to deliver secrets
     to user applications as environment variables.

   - **csi-node-driver-registrar**: Sidecar container that registers the CSI Node Plugin in kubelet.
     Calls the `GetPluginInfo` and `NodeGetInfo` RPCs in the secrets-store container to get plugin and node information.
   - **secrets-store**: Main container.
   - **csi-livenessprobe**: Sidecar container that monitors the CSI driver Unix socket
     and exposes the `/healthz` HTTP endpoint monitored by kubelet.
     If the `livenessProbe` check fails, kubelet restarts the csi-secrets-store pod.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to secrets-store container metrics.

1. **Vault-csi-provider** (DaemonSet): Runs on all cluster nodes
   and consists of a single **vault-csi-provider** container.
   Vault-csi-provider authenticates with the secrets store, retrieves data from it,
   and passes the data to csi-secrets-store.

1. **Ssi-controller** (Deployment): Consists of a single **ssi-controller** container.
   The controller watches SecretsStoreImport custom resources and creates SecretProviderClass custom resources based on them.

1. **Webhook** (Deployment): Implements mutating webhooks for Pod resources
   using [Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).

   The component mutates a pod if the pod has the `secrets-store.deckhouse.io/role` annotation.
   In this case, the component makes the following modifications to the pod manifest:

   - Adds an init container that copies a statically built injector binary
     from the `secrets-store-integration/env-injector` service image
     into a temporary directory shared by all containers in the pod.
   - If the pod manifest has the `secrets-store.deckhouse.io/env-from-path` annotation
     or a container uses secrets from the secrets store,
     then for each such container, including init containers,
     the component replaces the original startup command with a command that starts the injector binary.

     The injector receives the original startup command as an argument.
     If the container in the pod manifest has no startup command,
     the component retrieves the startup command from the image in the container registry.

   The component includes the following containers:

   - **vault-secrets-webhook**: Main container.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC
     that provides secure access to vault-secrets-webhook container metrics.

The following components are not part of the module, but the module affects them:

- **User-app**: Represents a typical user application that needs secrets delivered as environment variables.

   The component includes the following containers:

  - **copy-env-injector**: Init container added by the webhook component. It copies the injector binary from the     `secrets-store-integration/env-injector` service image.
  - **&lt;CONTAINER_NAME&gt;**: One or more containers
    (including init containers) of the original user application
    whose startup command was changed by the webhook component to start the injector binary.

   The injector authenticates with the secrets store, retrieves data,
   passes secrets to the application as environment variables,
   and starts the original container command.
   If the `secrets-store.deckhouse.io/restart-on-secret-change` annotation
   is set to `watch-for-lease` or `watch-for-data`,
   the injector rotates secrets when they change in the store.

## Module interactions

The `secrets-store-integration` module interacts with the following components:

1. **Kube-apiserver**:

   - Authorizes requests.
   - Retrieves CSIDriver, Secret, ConfigMap, and ServiceAccount.
   - Works with the SecretsStoreImport, SecretProviderClass,
     and SecretProviderClassPodStatus custom resources.

1. **Container registry**: Retrieves the startup command from the user container image.

1. **Secrets store**: Authorizes requests and retrieves secrets.

The following external components interact with the module:

1. **Kube-apiserver**: Invokes the mutating webhook when a Pod resource is created.

1. **Kubelet**:

   - Registers the Node Plugin.
   - Checks the CSI driver `livenessProbe`.
   - Calls the `NodePublishVolume` and `NodeUnpublishVolume` RPCs in the Node Plugin.

1. **Prometheus-main**: Collects metrics from the csi-secrets-store and webhook components.
