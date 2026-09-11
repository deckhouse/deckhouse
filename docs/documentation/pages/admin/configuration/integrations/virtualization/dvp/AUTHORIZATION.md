---
title: Connection and authorization in Deckhouse Virtualization Platform
permalink: en/admin/integrations/virtualization/dvp/authorization.html
---

To interact with virtualization resources, Deckhouse Platform components use the virtualization API. To configure access, create a user (ServiceAccount), assign the necessary permissions, and generate a kubeconfig.

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

{% alert level="warning" %}
If the `update-hostname` module of the `cloud-init` package is not disabled, it is recommended to change its run frequency from `always` to `once-per-instance`.

To do this, modify the `update-hostname` module configuration in the `/etc/cloud/cloud.cfg` file:

```yaml
cloud_init_modules:
  ...
  - [update-hostname, once-per-instance]
  ...
```

The `update-hostname` module can also be disabled completely by removing it from the `cloud_init_modules` module list in the `/etc/cloud/cloud.cfg` file.

{% endalert %}

## Creating a user

Create a new user in the virtualization cluster using the following command:

```bash
d8 k create -f -<<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa-demo
  namespace: default
---
apiVersion: v1
kind: Secret
metadata:
  name: sa-demo-token
  namespace: default
  annotations:
    kubernetes.io/service-account.name: sa-demo
type: kubernetes.io/service-account-token
EOF
```

## Assigning a role

Assign a role to the created user in the virtualization cluster using the following command:

```bash
d8 k create -f -<<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: sa-demo-rb
  namespace: default
subjects:
  - kind: ServiceAccount
    name: sa-demo
    namespace: default
roleRef:
  kind: ClusterRole
  name: d8:use:role:manager
  apiGroup: rbac.authorization.k8s.io
EOF
```

## Generating a kubeconfig

Generate a kubeconfig to be used in the cluster initial configuration file:

```bash
cat <<EOF > kubeconfig
apiVersion: v1
clusters:
- cluster:
    server: https://<KUBE-APISERVER-URL>   # Replace this with the actual API server address for the cluster.
  name: <CLUSTER-NAME>                     # Replace with the cluster name.
contexts:
- context:
    cluster: <CLUSTER-NAME>                # Replace with the cluster name.
    user: sa-demo
    namespace: default
  name: sa-demo-context
current-context: sa-demo-context
kind: Config
preferences: {}
users:
- name: sa-demo
  user:
    token: $(d8 k get secret sa-demo-token -n default -o json | jq -rc .data.token | base64 -d)
EOF
```

Encode the generated kubeconfig file using Base64 encoding (put it in the `d8-credentials` Secret in the `stringData.secret` field of the initial configuration file):

```bash
base64 kubeconfig | tr -d '\n'
```

## Credentials Secret

The credentials for accessing the parent cluster API are stored in a separate Secret rather than in the ModuleConfig. The provider reads it when starting the components that access the DVP API.

The Secret is created during cluster installation together with the other resources of the initial configuration. If the module is added to a running cluster, the Secret is applied after the module creates the namespace, and the order is covered in the [Hybrid cluster with DVP](../../hybrid/dvp-hybrid.html) section. The Secret must meet the following requirements:

- The name is `d8-credentials` and the namespace is `d8-cloud-provider-dvp`.
- The type is `cloud-provider.deckhouse.io/credentials`.
- The `authScheme` field is set to `kubeconfig`.
- The `secret` field contains the Base64-encoded kubeconfig.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-dvp
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: kubeconfig
  secret: <KUBE_CONFIG_BASE64>
```

Replace `<KUBE_CONFIG_BASE64>` with the Base64-encoded kubeconfig.

To change the credentials, update the `secret` field:

```shell
d8 k -n d8-cloud-provider-dvp edit secret d8-credentials
```

In clusters migrated to ModuleConfig, the kubeconfig is moved to this Secret from the `provider.kubeconfigDataBase64` parameter of the DVPClusterConfiguration resource.
