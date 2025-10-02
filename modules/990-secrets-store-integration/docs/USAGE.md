---
title: "The secrets-store-integration module: usage"
description: Usage of the secrets-store-integration Deckhouse module.
---

## Configuring the module to work with Deckhouse Stronghold

[Enable](/modules/stronghold/stable/usage.html#how-to-enable-the-module) the Stronghold module beforehand to automatically configure the secrets-store-integration module to work with [Deckhouse Stronghold](/modules/stronghold/stable/).

Next, apply the `ModuleConfig`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: secrets-store-integration
spec:
  enabled: true
  version: 1
```

The [connectionConfiguration](configuration.html#parameters-connectionconfiguration) paramater is optional and set to `DiscoverLocalStronghold` value by default.

## Configuring the module to work with the external secret store

The module requires a pre-configured secret vault compatible with HashiCorp Vault. An authentication path must be preconfigured in the vault. An example of how to configure the secret vault is provided in [Setting up the test environment](#setting-up-the-test-environment).

To ensure that each API request is encrypted, sent to, and replied by the correct recipient, a valid public Certificate Authority certificate used by the secret store is required. A `caCert` variable in the module configuration must refer to such a CA certificate in PEM format.

The following is an example module configuration for using a Vault-compliant secret store running at "secretstoreexample.com" on a regular port (443 TLS). Note that you will need to replace the parameters values in the configuration with the values that match your environment.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: secrets-store-integration
spec:
  version: 1
  enabled: true
  settings:
    connection:
      url: "https://secretstoreexample.com"
      authPath: "main-kube"
      caCert: |
        -----BEGIN CERTIFICATE-----
        kD8MMYv5NHHko/3jlBJCjVG6cI+5HaVekOqRN9l3D9ZXsdg2RdXLU8CecQAD7yYa
        ................................................................
        C2ZTJJonuI8dA4qUadvCXrsQqJEa2nw1rql4LfPP5ztJz1SwNCSYH7EmwqW+Q7WR
        bZ6GhOj=
        -----END CERTIFICATE-----
    connectionConfiguration: Manual
```

**It is strongly recommended to set the `caCert` variable. Otherwise, the module will use system ca-certificates.**

## Setting up the test environment

{% alert level="info" %}
First of all, you'll need a root or similiar token and the Stronghold address.
You can get such a root token while initializing a new secrets store.

All subsequent commands will assume that these settings are specified in environment variables.
```bash
export VAULT_TOKEN=xxxxxxxxxxx
export VAULT_ADDR=https://secretstoreexample.com
```
{% endalert %}

> This guide will cover two ways to do this:
>   * using the d8 multitool with integrated Stronghold CLI [[Download the d8 multitool](#download-the-d8-multitool-for-stronghold-commands)]
>   * using curl to make direct requests to the secrets store API

Before proceeding with the secret injection instructions in the examples below, do the following:

1. Create a kv2 type secret in Stronghold in `demo-kv/myapp-secret` and copy `DB_USER` and `DB_PASS` there.
2. If necessary, add an authentication path (authPath) for authentication and authorization to Stronghold using the Kubernetes API of the remote cluster
3. Create a policy named `myapp-ro-policy` in Stronghold that allows reading secrets from `demo-kv/myapp-secret`.
4. Create a `myapp-role` role in Stronghold for the `myapp-sa` service account in the `myapp-namespace` namespace and bind the policy you created earlier to it.
5. Create a `myapp-namespace` namespace in the cluster.
6. Create a `myapp-sa` service account in the created namespace.

Example commands to set up the environment:

* Enable and create the Key-Value store:

  ```bash
  d8 stronghold secrets enable -path=demo-kv -version=2 kv
  ```
  The same command as a curl HTTP request:

  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request POST \
    --data '{"type":"kv","options":{"version":"2"}}' \
    ${VAULT_ADDR}/v1/sys/mounts/demo-kv
  ```

* Set the database username and password as the value of the secret:

  ```bash
  d8 stronghold kv put demo-kv/myapp-secret DB_USER="username" DB_PASS="secret-password"
  ```
  The curl equivalent of the above command:

  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request PUT \
    --data '{"data":{"DB_USER":"username","DB_PASS":"secret-password"}}' \
    ${VAULT_ADDR}/v1/demo-kv/data/myapp-secret
  ```

* Double-check that the password has been saved successfully:

  ```bash
  d8 stronghold kv get demo-kv/myapp-secret
  ```

  The curl equivalent of the above command:
  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    ${VAULT_ADDR}/v1/demo-kv/data/myapp-secret
  ```

* By default, the method of authentication in Stronghold via Kubernetes API of the cluster on which Stronghold itself is running is enabled and configured under the name `kubernetes_local`. If you want to configure access via remote clusters, set the authentication path (`authPath`) and enable authentication and authorization in Stronghold via Kubernetes API for each cluster:

  ```bash
  d8 stronghold auth enable -path=remote-kube-1 kubernetes
  ```
  The curl equivalent of the above command:
  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request POST \
    --data '{"type":"kubernetes"}' \
    ${VAULT_ADDR}/v1/sys/auth/remote-kube-1
  ```

* Set the Kubernetes API address for each cluster (in this case, it is the K8s's API server service):

  ```bash
  d8 stronghold write auth/remote-kube-1/config \
    kubernetes_host="https://api.kube.my-deckhouse.com"
    disable_local_ca_jwt=true
  ```
  The curl equivalent of the above command:
  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request PUT \
    --data '{"kubernetes_host":"https://api.kube.my-deckhouse.com","disable_local_ca_jwt":true}' \
    ${VAULT_ADDR}/v1/auth/remote-kube-1/config
  ```

* Create a policy in Stronghold called `myapp-ro-policy` that allows reading of the `myapp-secret` secret:

  ```bash
  d8 stronghold policy write myapp-ro-policy - <<EOF
  path "demo-kv/data/myapp-secret" {
    capabilities = ["read"]
  }
  EOF
  ```
  The curl equivalent of the above command:
  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request PUT \
    --data '{"policy":"path \"demo-kv/data/myapp-secret\" {\n capabilities = [\"read\"]\n}\n"}' \
    ${VAULT_ADDR}/v1/sys/policies/acl/myapp-ro-policy
  ```


* Create a database role and bind it to the `myapp-sa` ServiceAccount in the `myapp-namespace` namespace and the `myapp-ro-policy` policy:

  {% alert level="danger" %}
  **Important!**
  In addition to the Stronghold side settings, you must configure the authorization permissions of the `serviceAccount` used in the kubernetes cluster.
  See the [paragraph below](#how-to-allow-a-serviceaccount-to-log-in-to-stronghold) section for details.
  {% endalert %}

  ```bash
  d8 stronghold write auth/kubernetes_local/role/myapp-role \
      bound_service_account_names=myapp-sa \
      bound_service_account_namespaces=myapp-namespace \
      policies=myapp-ro-policy \
      ttl=10m
  ```
  The curl equivalent of the above command:
  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request PUT \
    --data '{"bound_service_account_names":"myapp-sa","bound_service_account_namespaces":"myapp-namespace","policies":"myapp-ro-policy","ttl":"10m"}' \
    ${VAULT_ADDR}/v1/auth/kubernetes_local/role/myapp-role
  ```


* Repeat the same for the rest of the clusters, specifying a different authentication path:

  ```bash
  d8 stronghold write auth/remote-kube-1/role/myapp-role \
      bound_service_account_names=myapp-sa \
      bound_service_account_namespaces=myapp-namespace \
      policies=myapp-ro-policy \
      ttl=10m
  ```
  The curl equivalent of the above command:
  ```bash
  curl \
    --header "X-Vault-Token: ${VAULT_TOKEN}" \
    --request PUT \
    --data '{"bound_service_account_names":"myapp-sa","bound_service_account_namespaces":"myapp-namespace","policies":"myapp-ro-policy","ttl":"10m"}' \
    ${VAULT_ADDR}/v1/auth/remote-kube-1/role/myapp-role
  ```


{% alert level="info" %}
**Important!**
The recommended TTL value of the Kubernetes token is 10m.
{% endalert %}

These settings allow any pod within the `myapp-namespace` namespace in both K8s clusters that uses the `myapp-sa` ServiceAccount to authenticate, authorize, and read secrets in the Stronghold according to the `myapp-ro-policy` policy.

* Create namespace and then ServiceAccount in the specified namespace:
  ```bash
  kubectl create namespace myapp-namespace
  kubectl -n myapp-namespace create serviceaccount myapp-sa
  ```

## How to allow a ServiceAccount to log in to Stronghold?

To log in to Stronghold, a k8s pod uses a token generated for its ServiceAccount. In order for Stronghold to be able to check the validity of the ServiceAccount data provided by the service, Stronghold must have permission to `get`, `list`, and `watch` for the `tokenreviews.authentication.k8s.io` and `subjectaccessreviews.authorization.k8s.io` endpoints. You can also use the `system:auth-delegator` clusterRole for this.

Stronghold can use different credentials to make requests to the Kubernetes API:
1. Use the token of the application that is trying to log in to Stronghold. In this case, each service that logs in to Stronghold must have the `system:auth-delegator` clusterRole (or the API rights listed above) in the ServiceAccount it uses. [Check an example in Stronghold documentation](https://deckhouse.io/products/stronghold/documentation/user/auth/kubernetes.html#use-the-stronghold-clients-jwt-as-the-reviewer-jwt).
2. Use a static token created specifically for Stronghold `ServiceAccount` that has the necessary rights. Setting up Stronghold for this case is described in detail in [Stronghold documentation](https://deckhouse.io/products/stronghold/documentation/user/auth/kubernetes.html#continue-using-long-lived-tokens).

## Injecting environment variables

### How it works

When the module is enabled, a mutating-webhook becomes available in the cluster. It modifies the pod manifest, adding an injector, if the pod has the `secrets-store.deckhouse.io/role` annotation an init container is added to the modified pod. Its mission is to copy a statically compiled binary injector file from a service image into a temporary directory shared by all containers in the pod. In the other containers, the original startup commands are replaced with a command that starts the injector. It then fetches the required data from a Vault-compatible storage using the application's service account, sets these variables in the process ENV, and then issues an execve system call, invoking the original command.

If the container does not have a startup command in the pod manifest, the image manifest is retrieved from the image registry,
and the command is retrieved from it.
The credentials from `imagePullSecrets` specified in the pod manifest are used to retrieve the manifest from the private image registry.


The following are the available annotations to modify the injector behavior:
| Annotation                                       | Default value |  Function |
|--------------------------------------------------|-------------|-------------|
|secrets-store.deckhouse.io/addr                   | from module | The address of the secrets store in the format https://stronghold.mycompany.tld:8200 |
|secrets-store.deckhouse.io/auth-path              | from module | The path to use for authentication |
|secrets-store.deckhouse.io/namespace              | from module | The namespace that will be used to connect to the store |
|secrets-store.deckhouse.io/role                   |             | Sets the role to be used to connect to the secret store |
|secrets-store.deckhouse.io/env-from-path          |             | A string containing a comma-delimited list of paths to secrets in the repository, from which all keys will be extracted and placed in the environment. Priority is given to keys that are closer to the end of the list. |
|secrets-store.deckhouse.io/ignore-missing-secrets | false.      | Runs the original application if an attempt to retrieve a secret from the store fails |
|secrets-store.deckhouse.io/client-timeout         | 10s         | Timeout to use for secrets retrieval |
|secrets-store.deckhouse.io/mutate-probes          | false       | Injects environment variables into the probes |
|secrets-store.deckhouse.io/log-level              | info        | Logging level |
|secrets-store.deckhouse.io/enable-json-log        | false       | Log format (string or JSON) |

The injector allows you to specify env templates instead of values in the pod manifests. They will be replaced at the container startup stage with the values from the store.

{% alert level="info" %}
**Note**
Including variables from a store branch has a higher priority than including explicitly defined variables from the store. This means that when using both the `secrets-store.deckhouse.io/env-from-path` annotation with a path to a secret that contains, for example, the `MY_SECRET` key, and an environment variable in the manifest with the same name:
```yaml
env:
  - name: MY_SECRET
    value: secrets-store:demo-kv/data/myapp-secret#password
```
the `MY_SECRET` environment variable inside the container will be set to the value of the secret from the **annotation**.
{% endalert %}

For example, here's how you can retrieve the `DB_PASS` key from the kv2-secret at `demo-kv/myapp-secret` from the Vault-compatible store:

```yaml
env:
  - name: PASSWORD
    value: secrets-store:demo-kv/data/myapp-secret#DB_PASS
```

The example below retrieves the `DB_PASS` key version `4` from the kv2 secret at `demo-kv/myapp-secret` from the Vault-compatible store:

```yaml
env:
  - name: PASSWORD
    value: secrets-store:demo-kv/data/myapp-secret#DB_PASS#4
```

The template can also be stored in the ConfigMap or in the Secret and can be hooked up using `envFrom`:

```yaml
envFrom:
  - secretRef:
      name: app-secret-env
  - configMapRef:
      name: app-env

```
The actual secrets from the Vault-compatible store will be injected at the application startup; the Secret and ConfigMap will only contain the templates.

### Setting environment variables by specifying the path to the secret in the vault to retrieve all keys from

The following is the specification of a pod named `myapp1`. In it, all the values are retrieved from the store at the `demo-kv/data/myapp-secret` path and stored as environment variables:

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: myapp1
  namespace: myapp-namespace
  annotations:
    secrets-store.deckhouse.io/role: "myapp-role"
    secrets-store.deckhouse.io/env-from-path: demo-kv/data/common-secret,demo-kv/data/myapp-secret
spec:
  serviceAccountName: myapp-sa
  containers:
  - image: alpine:3.20
    name: myapp
    command:
    - sh
    - -c
    - while printenv; do sleep 5; done
```

Let's apply it:

```bash
kubectl create --filename myapp1.yaml
```

Check the pod logs after it has been successfully started. You should see all the values from `demo-kv/data/myapp-secret`:

```bash
kubectl -n myapp-namespace logs myapp1
```

Delete the pod:

```bash
kubectl -n myapp-namespace delete pod myapp1 --force
```

### Explicitly specifying the values to be retrieved from the vault and used as environment variables

Below is the spec of a test pod named `myapp2`. The pod will retrieve the required values from the vault according to the template and turn them into environment variables:

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: myapp2
  namespace: myapp-namespace
  annotations:
    secrets-store.deckhouse.io/role: "myapp-role"
spec:
  serviceAccountName: myapp-sa
  containers:
  - image: alpine:3.20
    env:
    - name: DB_USER
      value: secrets-store:demo-kv/data/myapp-secret#DB_USER
    - name: DB_PASS
      value: secrets-store:demo-kv/data/myapp-secret#DB_PASS
    name: myapp
    command:
    - sh
    - -c
    - while printenv; do sleep 5; done
```

Apply it:

```bash
kubectl create --filename myapp2.yaml
```

Check the pod logs after it has been successfully started. You should see the values from `demo-kv/data/myapp-secret` matching those in the pod specification:

```bash
kubectl -n myapp-namespace logs myapp2
```

Delete the pod:

```bash
kubectl -n myapp-namespace delete pod myapp2 --force
```

## Retrieving a secret from the vault and mounting it as a file in a container

Use the `SecretStoreImport` CustomResource to deliver secrets to the application.

In this example, we use the already created ServiceAccount `myapp-sa` and namespace `myapp-namespace` from step [Setting up the test environment](#setting-up-the-test-environment)

Create a _SecretsStoreImport_ CustomResource named `myapp-ssi` in the cluster:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecretsStoreImport
metadata:
  name: myapp-ssi
  namespace: myapp-namespace
spec:
  type: CSI
  role: myapp-role
  files:
    - name: "db-password"
      source:
        path: "demo-kv/data/myapp-secret"
        key: "DB_PASS"
```

Create a test pod in the cluster named `myapp3`. It will retrieve the required values from the vault and mount them as a file:

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: myapp3
  namespace: myapp-namespace
spec:
  serviceAccountName: myapp-sa
  containers:
  - image: alpine:3.20
    name: myapp
    command:
    - sh
    - -c
    - while cat /mnt/secrets/db-password; do echo; sleep 5; done
    name: backend
    volumeMounts:
    - name: secrets
      mountPath: "/mnt/secrets"
  volumes:
  - name: secrets
    csi:
      driver: secrets-store.csi.deckhouse.io
      volumeAttributes:
        secretsStoreImport: "myapp-ssi"
```

Once these resources have been applied, a pod will be created, inside which a container named `backend` will then be started. This container's filesystem will have a directory `/mnt/secrets`, with the `secrets` volume mounted to it. The directory will contain a `db-password` file with the password for database (`DB_PASS`) from the Stronghold key-value store.

Check the pod logs after it has been successfully started (you should see the contents of the `/mnt/secrets/db-password` file):

```bash
kubectl -n myapp-namespace logs myapp3
```

Delete the pod:

```bash
kubectl -n myapp-namespace delete pod myapp3 --force
```
### Delivering Binary Files to a Container

There are situations when you need to deliver a binary file to a container. This could be a JKS container with keys,
or a keytab for Kerberos authentication.
In this case, you can encode the binary file using base64 and place it in the secrets store. When you retrieve it,
the CSI driver will decode your data and place the binary file in the container. To do this, set the `decodeBase64`
parameter to `true` for the corresponding file.
If decoding fails (for example, the storage contains an invalid base64), the container will not be created.

Example:

Putting a file into storage

```bash
d8 stronghold kv put demo-kv/myapp-secret keytab=$(cat /path/to/keytab_file | base64 -w0)
```

SecretsStoreImport Manifest

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecretsStoreImport
metadata:
  name: myapp-ssi
  namespace: myapp-namespace
spec:
  type: CSI
  role: myapp-role
  files:
    - name: "keytab"
      decodeBase64: true
      source:
        path: "demo-kv/data/myapp-secret"
        key: "keytab"
```

In this case, a binary file named `keytab` will be created in the container

### The autorotation feature

The autorotation feature of the secret-store-integration module is enabled by default. Every two minutes, the module polls Stronghold and synchronizes the secrets in the mounted file if it has been changed.

There are two ways to keep track of changes to the secret file in the pod. The first is to keep track of when the mounted file changes (mtime), reacting to changes in the file. The second is to use the inotify API, which provides a mechanism for subscribing to file system events. Inotify is part of the Linux kernel. Once a change is detected, there are a large number of options for responding to the change event, depending on the application architecture and programming language used. The most simple one is to force K8s to restart the pod by failing the liveness probe.

Here is how you can use inotify in a Python application leveraging the `inotify` Python package:

```python
#!/usr/bin/python3

import inotify.adapters

def _main():
    i = inotify.adapters.Inotify()
    i.add_watch('/mnt/secrets-store/db-password')

    for event in i.event_gen(yield_nones=False):
        (_, type_names, path, filename) = event

        if 'IN_MODIFY' in type_names:
            print("file modified")

if __name__ == '__main__':
    _main()
```

Sample code to detect whether a password has been changed within a Go application using inotify and the `inotify` Go package:

```python
watcher, err := inotify.NewWatcher()
if err != nil {
    log.Fatal(err)
}
err = watcher.Watch("/mnt/secrets-store/db-password")
if err != nil {
    log.Fatal(err)
}
for {
    select {
    case ev := <-watcher.Event:
        if ev == 'InModify' {
        	log.Println("file modified")}
    case err := <-watcher.Error:
        log.Println("error:", err)
    }
}
```

#### Secret rotation limitations

A container that uses the `subPath` volume mount will not get secret updates when the latter is rotated.

```yaml
   volumeMounts:
   - mountPath: /app/settings.ini
     name: app-config
     subPath: settings.ini
...
 volumes:
 - name: app-config
   csi:
     driver: secrets-store.csi.deckhouse.io
     volumeAttributes:
       secretsStoreImport: "python-backend"
```

## Download the d8 multitool for stronghold commands

### Official website of Deckhouse Kubernetes Platform

Go to the official website and follow the [instructions](/products/kubernetes-platform/documentation/v1/deckhouse-cli/#how-do-i-install-the-deckhouse-cli).

### The subdomain of your Deckhouse Kubernetes Platform

To download the multitool:
1. Go to the page `tools.<cluster_domain>`, where `<cluster_domain>` is the DNS name that matches the template defined in the [modules.publicDomainTemplate](/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) parameter.

2. Select *Deckhouse CLI* for your operating system.
1. **For Linux and MacOS:**
  - Add execution rights to `d8` via `chmod +x d8`.
  - Move the executable file to the `/usr/local/bin/` folder.

   **For Windows:**
  - Extract the archive, move the `d8.exe` file to a directory of your choice, and add the directory to PATH variable of the operating system.
  - Unblock the `d8.exe file`, for example, in the following way:
    - Right-click on the file and select *Properties* from the context menu.
    - In the *Properties* window, ensure you are on the *General* tab.
    - At the bottom of the *General* tab, you may see a *Security* section with a message about the file being blocked.
    - Check the *Unblock* box or click the *Unblock* button, then click *Apply* and *OK* to save the changes.
1. Check that the utility works:
    ```
    d8 help
    ```
Congrats, you have successfully installed the `d8 stronhold`.
