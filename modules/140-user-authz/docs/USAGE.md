---
title: "The user-authz module: usage"
---

## An example of `ClusterAuthorizationRule`

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: test-rule
spec:
  subjects:
  - kind: User
    name: some@example.com
  - kind: ServiceAccount
    name: gitlab-runner-deploy
    namespace: d8-service-accounts
  - kind: Group
    name: some-group-name
  accessLevel: PrivilegedUser
  portForwarding: true
  # This option is only available if the enableMultiTenancy parameter is set (Enterprise Edition version)
  allowAccessToSystemNamespaces: false
  # This option is only available if the enableMultiTenancy parameter is set (Enterprise Edition version)
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: stage
        operator: In
        values:
        - test
        - review
      matchLabels:
        team: frontend
```

## Creating a user

There are two types of users in Kubernetes:

* Service accounts managed by Kubernetes via the API;
* Regular users managed by some external tool that the cluster administrator configures. There are many authentication mechanisms and, accordingly, many ways to create users. Currently, two authentication methods are supported:
  * Via the [user-authn](../../modules/150-user-authn/) module.
  * Via the certificates.

When issuing the authentication certificate, you need to specify the name (`CN=<name>`), the required number of groups (`O=<group>`), and sign it using the root CA of the cluster. It is this mechanism that authenticates you in the cluster when, for example, you use kubectl on a bastion node.

### Creating a ServiceAccount for a machine and granting it access

You may need to create a ServiceAccount with access to the Kubernetes API when, for example, an application is deployed using a CI system.

1. Create a ServiceAccount, e.g., in the `d8-service-accounts` namespace:

   ```shell
   kubectl create -f - <<EOF
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: gitlab-runner-deploy
     namespace: d8-service-accounts
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: gitlab-runner-deploy-token
     namespace: d8-service-accounts
     annotations:
       kubernetes.io/service-account.name: gitlab-runner-deploy
   type: kubernetes.io/service-account-token
   EOF
   ```

1. Grant it the necessary privileges (using the [ClusterAuthorizationRule](cr.html#clusterauthorizationrule) custom resource):

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: ClusterAuthorizationRule
   metadata:
     name: gitlab-runner-deploy
   spec:
     subjects:
     - kind: ServiceAccount
       name: gitlab-runner-deploy
       namespace: d8-service-accounts
     accessLevel: SuperAdmin
     # This option is only available if the enableMultiTenancy parameter is set (Enterprise Edition version)
     allowAccessToSystemNamespaces: true      
   EOF
   ```

   If multitenancy is enabled in the Deckhouse configuration (the [enableMultiTenancy](configuration.html#parameters-enablemultitenancy) parameter; it is only available in Enterprise Edition), configure the namespaces the ServiceAccount has access to (the [namespaceSelector](cr.html#clusterauthorizationrule-v1-spec-namespaceselector) parameter).

1. Set the variable values (they will be used later) by running the following commands (**insert your own values**):

   ```shell
   export CLUSTER_NAME=my-cluster
   export USER_NAME=gitlab-runner-deploy.my-cluster
   export CONTEXT_NAME=${CLUSTER_NAME}-${USER_NAME}
   export FILE_NAME=kube.config
   ```

1. Generate the `cluster` section in the kubectl configuration file:

   Use one of the following options to access the cluster API server:

   * If there is direct access to the API server:
     1. Get a Kubernetes cluster CA certificate:

        ```shell
        kubectl get cm kube-root-ca.crt -o jsonpath='{ .data.ca\.crt }' > /tmp/ca.crt
        ```

     1. Generate the `cluster` section (the API server's IP address is used for access):

        ```shell
        kubectl config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(kubectl get ep kubernetes -o json | jq -rc '.subsets[0] | "\(.addresses[0].ip):\(.ports[0].port)"') \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * If there is no direct access to the API server, use one of the following options:
      * enable access to the API-server over the Ingress controller (the [publishAPI](../150-user-authn/configuration.html#parameters-publishapi) parameter) and specify the addresses from which requests originate (the [whitelistSourceRanges](../150-user-authn/configuration.html#parameters-publishapi-whitelistsourceranges) parameter);
      * specify addresses from which requests will originate in a separate Ingress controller (the [acceptRequestsFrom](../402-ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) parameter).

   * If a non-public CA is used:

     1. Get the CA certificate from the Secret with the certificate that is used for the `api.%s` domain:

        ```shell
        kubectl -n d8-user-authn get secrets -o json \
          $(kubectl -n d8-user-authn get ing kubernetes-api -o jsonpath="{.spec.tls[0].secretName}") \
          | jq -rc '.data."ca.crt" // .data."tls.crt"' \
          | base64 -d > /tmp/ca.crt
        ```

     2. Generate the `cluster` section (an external domain and a CA for access are used):

        ```shell
        kubectl config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(kubectl -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * If a public CA is used. Generate the `cluster` section (an external domain is used for access):

     ```shell
     kubectl config set-cluster $CLUSTER_NAME \
       --server=https://$(kubectl -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
       --kubeconfig=$FILE_NAME
     ```

1. Generate the `user` section using the token from the Secret's ServiceAccount in the kubectl configuration file:

   ```shell
   kubectl config set-credentials $USER_NAME \
     --token=$(kubectl -n d8-service-accounts get secret gitlab-runner-deploy-token -o json |jq -r '.data["token"]' | base64 -d) \
     --kubeconfig=$FILE_NAME
   ```

1. Generate the context in the kubectl configuration file:

   ```shell
   kubectl config set-context $CONTEXT_NAME \
     --cluster=$CLUSTER_NAME --user=$USER_NAME \
     --kubeconfig=$FILE_NAME
   ```

1. Set the generated context as the default one in the kubectl configuration file:

   ```shell
   kubectl config use-context $CONTEXT_NAME --kubeconfig=$FILE_NAME
   ```

### How to create a user using a client certificate

#### Creating a user

* Get the cluster's root certificate (ca.crt and ca.key).
* Generate the user key:

  ```shell
  openssl genrsa -out myuser.key 2048
  ```

* Create a CSR file and specify in it the username (`myuser`) and groups to which this user belongs (`mygroup1` & `mygroup2`):

  ```shell
  openssl req -new -key myuser.key -out myuser.csr -subj "/CN=myuser/O=mygroup1/O=mygroup2"
  ```

* Sign the CSR using the cluster root certificate:

  ```shell
  openssl x509 -req -in myuser.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out myuser.crt -days 10
  ```

* Now you can use the certificate issued in the config file:

  ```shell
  cat << EOF
  apiVersion: v1
  clusters:
  - cluster:
      certificate-authority-data: $(cat ca.crt | base64 -w0)
      server: https://<cluster_host>:6443
    name: kubernetes
  contexts:
  - context:
      cluster: kubernetes
      user: myuser
    name: myuser@kubernetes
  current-context: myuser@kubernetes
  kind: Config
  preferences: {}
  users:
  - name: myuser
    user:
      client-certificate-data: $(cat myuser.crt | base64 -w0)
      client-key-data: $(cat myuser.key | base64 -w0)
  EOF
  ```

#### Granting access to the created user

To grant access to the created user, create a `ClusterAuthorizationRule'.

Example of a `ClusterAuthorizationRule`:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: myuser
spec:
  subjects:
  - kind: User
    name: myuser
  accessLevel: PrivilegedUser
  portForwarding: true
```

## Configuring kube-apiserver for multi-tenancy mode

The multi-tenancy mode, which allows you to restrict access to namespaces, is enabled by the [enableMultiTenancy](configuration.html#parameters-enablemultitenancy) module's parameter.

Working in multi-tenancy mode requires enabling the [Webhook authorization plugin](https://kubernetes.io/docs/reference/access-authn-authz/webhook/) and configuring a `kube-apiserver.` All actions necessary for the multi-tenancy mode are performed **automatically** by the [control-plane-manager](../../modules/040-control-plane-manager/) module; no additional steps are required.

Changes to the `kube-apiserver` manifest that will occur after enabling multi-tenancy mode:

* The `--authorization-mode` argument will be modified: the Webhook method will be added in front of the RBAC method (e.g., `--authorization-mode=Node,Webhook,RBAC`);
* The `--authorization-webhook-config-file=/etc/kubernetes/authorization-webhook-config.yaml` will be added;
* The `volumeMounts` parameter will be added:

  ```yaml
  - name: authorization-webhook-config
    mountPath: /etc/kubernetes/authorization-webhook-config.yaml
    readOnly: true
  ```

* The `volumes` parameter will be added:

  ```yaml
  - name: authorization-webhook-config
    hostPath:
      path: /etc/kubernetes/authorization-webhook-config.yaml
      type: FileOrCreate
  ```

## How do I check that a user has access?

Execute the command below with the following parameters:

* `resourceAttributes` (the same as in RBAC) - target resources;
* `user` - the name of the user;
* `groups` - user groups;

> You can use Dex logs to find out groups and a username if this module is used together with the `user-authn` module (`kubectl -n d8-user-authn logs -l app=dex`); logs available only if the user is authorized).

```shell
cat  <<EOF | 2>&1 kubectl  create --raw  /apis/authorization.k8s.io/v1/subjectaccessreviews -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "system:kube-controller-manager",
    "groups": [
      "Admins"
    ]
  }
}
EOF
```

You will see if access is allowed and what role is used:

```json
{
  "allowed": true,
  "reason": "RBAC: allowed by ClusterRoleBinding \"system:kube-controller-manager\" of ClusterRole \"system:kube-controller-manager\" to User \"system:kube-controller-manager\""
}
```

If the **multitenancy** mode is enabled in your cluster, you need to perform another check to be sure that the user has access to the namespace:

```shell
cat  <<EOF | 2>&1 kubectl --kubeconfig /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml create --raw / -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "system:kube-controller-manager",
    "groups": [
      "Admins"
    ]
  }
}
EOF
```

```json
{
  "allowed": false
}
```

The `allowed: false` message means that the webhook doesn't block access. In case of webhook denying the request, you will see, e.g., the following message:

```json
{
  "allowed": false,
  "denied": true,
  "reason": "making cluster scoped requests for namespaced resources are not allowed"
}
```

## Customizing rights of high-level roles

If you want to grant more privileges to a specific [high-level role](./#role-model), you only need to create a ClusterRole with the `user-authz.deckhouse.io/access-level: <AccessLevel>` annotation.

An example:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: Editor
  name: user-editor
rules:
- apiGroups:
  - kuma.io
  resources:
  - trafficroutes
  - trafficroutes/finalizers
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - flagger.app
  resources:
  - canaries
  - canaries/status
  - metrictemplates
  - metrictemplates/status
  - alertproviders
  - alertproviders/status
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
```
