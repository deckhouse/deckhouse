---
title: "The secrets-store-integration module"
description: "The secrets-store-integration module integrates secret stores and applications in K8s clusters"
---

The secrets-store-integration module delivers secrets to the application pods in the Kubernetes
cluster by mounting multiple secrets, keys, and certificates stored in external secret stores.

Secrets are mounted into pods as volumes using the CSI driver implementation.
Note that secret stores must be compatible with the HashiCorp Vault API.

## Delivering secrets to applications

There are several ways to deliver secrets to an application from a vault-compatible storage:

1. Your application itself can access the vault.

   > This is the most secure option, but it requires an application to be modified.

2. An intermediate application retrieves secrets from the vault, and your application retrieves secrets from files created in the container.

   > Use this option if you cannot modify the application for some reason. It is less secure because the secrets are stored in files in the container. However, it is easier to implement.

3. An intermediate application retrieves secrets from the vault, and your application accesses the secrets as the environment variables.

   > If reading from files is unavailable, you can opt for this alternative. Keep in mind, however, that it is NOT secure, because the secret data is stored in Kubernetes (and etcd, so it can potentially be read on any node in the cluster).

<table>
<thead>
<tr>
<th>How secrets are being delivered</th>
<th>Resources consumption</th>
<th>How your application gets the data</th>
<th>Where the secret is stored in the Kubernetes</th>
<th>Status</th>
</tr>
</thead>
<tbody>
<tr>
<td><a style="color: ##0066FF;" href="#option-1-get-the-secrets-from-the-app-itself">App</a></td>
<td>No changes</td>
<td>Directly from the secrets store</td>
<td>Not stored</td>
<td>Implemented</td>
</tr>
<tr>
<td><a style="color: ##0066FF;" href="#csi-interface">CSI Interface</a></td>
<td>Two pods per node (daemonset)</td>
<td><ul><li>From the disk volume (as a file)</li><li>From the environment variable</li></ul></td>
<td>Not stored</td>
<td>Implemented</td>
</tr>
<tr>
<td><a style="color: ##0066FF;" href="#option-3-entrypoint-injection">Entrypoint injection</a></td>
<td>One app for the whole cluster (deployment)</td>
<td>Secrets are delivered as environment variables at application startup</td>
<td>Not stored</td>
<td>Implemented</td>
</tr>
<tr>
<td><a style="color: ##0066FF;" href="#option-4-delivering-secrets-through-kubernetes-mechanisms">Kubernetes Secrets</a></td>
<td>One app for the whole cluster (deployment)</td>
<td><ul><li>From the disk volume (as a file)</li><li>From the environment variable</li></ul></td>
<td>Stored as a Kubernetes Secret</td>
<td>Planned for implementation and release</td>
</tr>
<tr>
<td><a style="color: #A9A9A9; font-style: italic;" href="#for-reference-vault-agent-injector">Vault-agent Injector</a></td>
<td style="color: #A9A9A9; font-style: italic;">One agent per pod (sidecar)</td>
<td style="color: #A9A9A9; font-style: italic;">From the disk volume (as a file)</td>
<td style="color: #A9A9A9; font-style: italic;">Not stored</td>
<td style="color: #A9A9A9; font-style: italic;"><sup><b>*</b></sup>No implementation plans</td>
</tr>
</tbody>
</table>

<i><sup>*</sup>No implementation plans. There are no advantages over the CSI interface.</i>

### Option #1: Getting the secrets using the app itself

> *Status:* the most secure option. Recommended if you can access the application and modify it.

The application accesses the Stronghold API and retrieves the secret over HTTPS using the SA authorization token.

#### Pros:

- The secret received by the application is not stored anywhere other than the application itself. There is no danger that it will be compromised during the transmission.

#### Cons:

- The application will need to be modified for it to work with Stronghold.
- You would have to re-implement secret access in each application, and if the library is updated, you would have to rebuild all the applications.
- The application must support TLS and certificate validation.
- No caching is available. When the application restarts, it will have to re-request the secret straight from the storage.

### Option #2: Delivering secrets using files

#### CSI interface

> *Status:* secure option. Recommended if you cannot make changes to the application.

When creating pods that request CSI volumes, the CSI secret vault driver sends a request to the Vault CSI. The Vault CSI then uses the specified SecretProviderClass and ServiceAccount of the pod to retrieve the secrets from the vault and mount them in the pod volume.

#### Environment variable injection:

If there is no way to change the application code, you can implement secure secret injection as an environment variable that the application can use. To do so, read all the files mounted by the CSI into the container and define the environment variables so that their names correspond to the file names and values correspond to the file contents. After that, run the application. Refer to the Bash example below.

```bash
bash -c "for file in $(ls /mnt/secrets); do export  $file=$(cat /mnt/secrets/$file); done ; exec my_original_file_to_startup"
```

#### Pros:

- Only two containers, whose resource requirements are known in advance, are required on each node to deliver secrets to applications.
- Creating SecretsStore/SecretProviderClass resources reduces the amount of repetitive code compared to other vault agent implementations.
- If necessary, you can create a Kubernetes secret that is a copy of the secret retrieved from the vault.
- The secret is retrieved from the vault by the CSI driver at the container creation stage. This means that pods will be started only after the secrets are read from the vault and written to the container volume.

### Option №3: Entrypoint injection

#### Delivering environment variables into the container through entrypoint injection

> *Status:* secure option. This option is currently being developed.

Environment variables are propagated into the container at application startup. They are stored in RAM only. At first, variables will be delivered via the entrypoint injection into the container. In the future, we plan to integrate the secrets delivery mechanism into containerd.

### Option #4: Delivering secrets using Kubernetes mechanisms

> *Status:* not secure; not recommended for use. No support is available. It may be implemented in the future..

This integration method relies on the Kubernetes secrets operator with a set of CRDs for synchronizing secrets from Vault to the Kubernetes secrets.

#### Pros:

- This is the traditional way of passing a secret to an application via environment variables - all you have to do is to hook up the Kubernetes secret.

#### Cons:

- The secret is stored in both the secret store and the Kubernetes secret (which can be accessed via the Kubernetes API). The secret is also stored in etcd and can potentially be read on any cluster node or retrieved from an etcd backup. No option to avoid storing data in Kubernetes secrets is available.

### For reference: vault-agent injector

> *Status:* no pros compared to the CSI mechanism. No support/implementation is available or in plans.

When a pod is created, a mutation is run that adds a vault-agent container. The vault-agent retrieves the secrets from the secret store and puts them in a shared volume on a disk that can be accessed by the application.

#### Cons:

- Each pod requires running a sidecar container, which consumes resources. Suppose there are 50 applications running in a cluster, each having 3 to 15 replicas. While the resource requirements of each sidecar are low, the numbers get noticeable when multiplied by the total number of containers: 50mcpu + 100Mi per sidecar means dozens of CPU cores and dozens of gigabytes of RAM.


- Since metrics are collected from each container, this approach will produce twice as many container metrics. 
