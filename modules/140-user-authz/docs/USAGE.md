---
title: "The user-authz module: usage"
---

## Example of assigning rights to a cluster administrator

{% alert level="info" %}
The example uses the [experimental role-based](./#experimental-role-based-model).
{% endalert %}

To grant access to a cluster administrator, use the role `d8:manage:all:manager` in `ClusterRoleBinding`.

Example of assigning rights to a cluster administrator (User `jane`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-admin-jane
subjects:
- kind: User
  name: jane.doe@example.com
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:manage:all:manager
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="The rights that the user will get" %}
The rights that the user will get will be limited to namespaces starting with `d8-` or `kube-`.

The user will be able to:
- View, modify, delete, and create Kubernetes resources and DKP modules;
- Modify module configurations (view, modify, delete, and create `moduleConfig` resources);
- Execute the following commands on pods and services:
  - `kubectl attach`
  - `kubectl exec`
  - `kubectl port-forward`
  - `kubectl proxy`
{% endofftopic %}

## Example of assigning rights to a network administrator

{% alert level="info" %}
The example uses the [experimental role-based](./#experimental-role-based-model).
{% endalert %}

To grant a network administrator access to manage the network subsystem of the cluster, use the role `d8:manage:networking:manager` in `ClusterRoleBinding`.

Example of assigning rights to a network administrator (User `jane`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: network-admin-jane
subjects:
- kind: User
  name: jane.doe@example.com
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:manage:networking:manager
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="The rights that the user will get" %}
The rights that the user will get will be limited to the following list of DKP module namespaces from the `networking` subsystem (the actual list depends on the list of modules included in the cluster):
- `d8-cni-cilium`
- `d8-cni-flannel`
- `d8-cni-simple-bridge`
- `d8-ingress-nginx`
- `d8-istio`
- `d8-metallb`
- `d8-network-gateway`
- `d8-openvpn`
- `d8-static-routing-manager`
- `d8-system`
- `kube-system`

The user will be able to:
- View, modify, delete, and create *standard* Kubernetes resources in the module namespace from the `networking` subsystem.

  Example of resources that the user will be able to manage (the list is not exhaustive):
  - `Certificate`
  - `CertificateRequest`
  - `ConfigMap`
  - `ControllerRevision`
  - `CronJob`
  - `DaemonSet`
  - `Deployment`
  - `Event`
  - `HorizontalPodAutoscaler`
  - `Ingress`
  - `Issuer`
  - `Job`
  - `Lease`
  - `LimitRange`
  - `NetworkPolicy`
  - `PersistentVolumeClaim`
  - `Pod`
  - `PodDisruptionBudget`
  - `ReplicaSet`
  - `ReplicationController`
  - `ResourceQuota`
  - `Role`
  - `RoleBinding`
  - `Secret`
  - `Service`
  - `ServiceAccount`
  - `StatefulSet`
  - `VerticalPodAutoscaler`
  - `VolumeSnapshot`

- View, modify, delete, and create the following resources in the modules namespace from the `networking` subsystem:

  A list of resources that the user will be able to manage:
  - `EgressGateway`
  - `EgressGatewayPolicy`
  - `FlowSchema`
  - `IngressClass`
  - `IngressIstioController`
  - `IngressNginxController`
  - `IPRuleSet`
  - `IstioFederation`
  - `IstioMulticluster`
  - `RoutingTable`

- Modify the configuration of modules (view, change, delete, and create moduleConfig resources) from the `networking` subsystem.

  List of modules that the user will be able to manage:
  - `cilium-hubble`
  - `cni-cilium`
  - `cni-flannel`
  - `cni-simple-bridge`
  - `flow-schema`
  - `ingress-nginx`
  - `istio`
  - `kube-dns`
  - `kube-proxy`
  - `metallb`
  - `network-gateway`
  - `network-policy-engine`
  - `node-local-dns`
  - `openvpn`
  - `static-routing-manager`
  
- Execute the following commands with pods and services in the modules namespace from the `networking` subsystem:
  - `kubectl attach`
  - `kubectl exec`
  - `kubectl port-forward`
  - `kubectl proxy`
{% endofftopic %}

## Example of assigning administrative rights to a user within a namespace

{% alert level="info" %}
The example uses the [experimental role-based](./#experimental-role-based-model).
{% endalert %}

To assign rights to a user manage application resources within a namespace, but without the ability to configure DKP modules, use the role `d8:use:role:admin` in `RoleBinding` in the corresponding namespace.

Example of assigning rights to an application developer (User `app-developer`) in namespace `myapp`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: myapp-developer
  namespace: myapp
subjects:
- kind: User
  name: app-developer@example.com
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:use:role:admin
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="The rights that the user will get" %}
In the `myapp` namespace, the user will be able to:
- View, modify, delete, and create Kubernetes resources. For example, the following resources (the list is not exhaustive):
  - `Certificate`
  - `CertificateRequest`
  - `ConfigMap`
  - `ControllerRevision`
  - `CronJob`
  - `DaemonSet`
  - `Deployment`
  - `Event`
  - `HorizontalPodAutoscaler`
  - `Ingress`
  - `Issuer`
  - `Job`
  - `Lease`
  - `LimitRange`
  - `NetworkPolicy`
  - `PersistentVolumeClaim`
  - `Pod`
  - `PodDisruptionBudget`
  - `ReplicaSet`
  - `ReplicationController`
  - `ResourceQuota`
  - `Role`
  - `RoleBinding`
  - `Secret`
  - `Service`
  - `ServiceAccount`
  - `StatefulSet`
  - `VerticalPodAutoscaler`
  - `VolumeSnapshot`
- View, edit, delete, and create the following DKP module resources:
  - `DexAuthenticator`
  - `DexClient`
  - `PodLogginConfig`
- Execute the following commands for pods and services:
  - `kubectl attach`
  - `kubectl exec`
  - `kubectl port-forward`
  - `kubectl proxy`
{% endofftopic %}

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
  # This option is only available if the enableMultiTenancy parameter is set.
  allowAccessToSystemNamespaces: false
  # This option is only available if the enableMultiTenancy parameter is set.
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
* Regular users and groups managed by some external tool that the cluster administrator configures. There are many authentication mechanisms and, accordingly, many ways to create users. Currently, two authentication methods are supported:
  * Via the `user-authn` module. The module supports the following external providers and authentication protocols: GitHub, GitLab, Atlassian Crowd, BitBucket Cloud, Crowd, LDAP, OIDC. More details — in the documentation of the [`user-authn`](../../modules/user-authn/) module.
  * Via the [certificates](#how-to-create-a-user-using-a-client-certificate).

When issuing the authentication certificate, you need to specify the name (`CN=<name>`), the required number of groups (`O=<group>`), and sign it using the root CA of the cluster. It is this mechanism that authenticates you in the cluster when, for example, you use kubectl on a bastion node.

### Creating a ServiceAccount for a machine and granting it access

You may need to create a ServiceAccount with access to the Kubernetes API when, for example, an application is deployed using a CI system.

1. Create a ServiceAccount, e.g., in the `d8-service-accounts` namespace:

   ```shell
   d8 k create -f - <<EOF
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
   d8 k create -f - <<EOF
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
     # This option is only available if the enableMultiTenancy parameter is set.
     allowAccessToSystemNamespaces: true      
   EOF
   ```

   If multitenancy is enabled in the Deckhouse configuration (via the [`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy) parameter), configure the namespaces the ServiceAccount has access to (via the [`namespaceSelector`](cr.html#clusterauthorizationrule-v1-spec-namespaceselector) parameter).

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
        d8 k get cm kube-root-ca.crt -o jsonpath='{ .data.ca\.crt }' > /tmp/ca.crt
        ```

     1. Generate the `cluster` section (the API server's IP address is used for access):

        ```shell
        d8 k config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(d8 k get ep kubernetes -o json | jq -rc '.subsets[0] | "\(.addresses[0].ip):\(.ports[0].port)"') \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * If there is no direct access to the API server, use one of the following options:
      * enable access to the API-server over the Ingress controller (the [publishAPI](../user-authn/configuration.html#parameters-publishapi) parameter) and specify the addresses from which requests originate (the [whitelistSourceRanges](../user-authn/configuration.html#parameters-publishapi-whitelistsourceranges) parameter);
      * specify addresses from which requests will originate in a separate Ingress controller (the [acceptRequestsFrom](../ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) parameter).

   * If a non-public CA is used:

     1. Get the CA certificate from the Secret with the certificate that is used for the `api.%s` domain:

        ```shell
        d8 k -n d8-user-authn get secrets -o json \
          $(d8 k -n d8-user-authn get ing kubernetes-api -o jsonpath="{.spec.tls[0].secretName}") \
          | jq -rc '.data."ca.crt" // .data."tls.crt"' \
          | base64 -d > /tmp/ca.crt
        ```

     2. Generate the `cluster` section (an external domain and a CA for access are used):

        ```shell
        d8 k config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(d8 k -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * If a public CA is used. Generate the `cluster` section (an external domain is used for access):

     ```shell
     d8 k config set-cluster $CLUSTER_NAME \
       --server=https://$(d8 k -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
       --kubeconfig=$FILE_NAME
     ```

1. Generate the `user` section using the token from the Secret's ServiceAccount in the kubectl configuration file:

   ```shell
   d8 k config set-credentials $USER_NAME \
     --token=$(d8 k -n d8-service-accounts get secret gitlab-runner-deploy-token -o json |jq -r '.data["token"]' | base64 -d) \
     --kubeconfig=$FILE_NAME
   ```

1. Generate the context in the kubectl configuration file:

   ```shell
   d8 k config set-context $CONTEXT_NAME \
     --cluster=$CLUSTER_NAME --user=$USER_NAME \
     --kubeconfig=$FILE_NAME
   ```

1. Set the generated context as the default one in the kubectl configuration file:

   ```shell
   d8 k config use-context $CONTEXT_NAME --kubeconfig=$FILE_NAME
   ```

### How to create a user using a client certificate

{% alert level="info" %}
This method is recommended for system needs (authentication of kubelets, control plane components, etc.). If you need to create a "normal" user (e.g. with console access, `kubectl`, etc.), use [kubeconfig generation](../user-authn/faq.html#how-to-generate-a-kubeconfig-and-access-kubernetes-api).
{% endalert %}

To create a user with a client certificate, you can use either [OpenSSL](#creating-a-user-using-a-certificate-issued-via-openssl) or the [Kubernetes API (via the CertificateSigningRequest object)](#creating-a-user-by-issuing-a-certificate-via-the-kubernetes-api).

{% alert level="warning" %}
Certificates issued by any of these methods cannot be revoked.
If a certificate is compromised, you will need to remove all permissions for that user (this can be difficult if the user is added to any groups: you will also need to remove all relevant groups).
{% endalert %}

#### Creating a user using a certificate issued via OpenSSL

{% alert level="warning" %}
Consider security risks when using this method.

The `ca.crt` and `ca.key` must not leave the master node: sign the CSR only on the master node.

Signing CSRs outside the master node risks compromising the cluster root certificate.
{% endalert %}

The features of this method are:

- The client certificate must be signed on the master node to prevent the cluster certificate from being compromised.
- Access to the cluster CA key (`ca.key`) is required. Only the cluster administrator can sign certificates.

To create a user using a client certificate issued through OpenSSL, follow these steps:

1. Get the cluster’s root certificate (`ca.crt` and `ca.key`).
1. Generate the user key:

    ```shell
    openssl genrsa -out myuser.key 2048
    ```

1. Create a CSR file and specify the username in it (`myuser`) and groups to which this user belongs (`mygroup1` and `mygroup2`):

    ```shell
    openssl req -new -key myuser.key -out myuser.csr -subj "/CN=myuser/O=mygroup1/O=mygroup2"
    ```

1. Upload the CSR created in the previous step (`myuser.csr` in this example) to the master node and sign it with the cluster root certificate. Example command to sign the CSR on the master node (make sure that the paths to `myuser.csr`, `ca.crt` and `ca.key` are correct for your case):

    ```shell
    openssl x509 -req -in myuser.csr -CA /etc/kubernetes/pki/ca.crt -CAkey /etc/kubernetes/pki/ca.key -CAcreateserial -out myuser.crt -days 10
    ```

Now the certificate can be specified in the config file:

```shell
cat << EOF
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: $(cat /etc/kubernetes/pki/ca.crt | base64 -w0)
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

#### Creating a user by issuing a certificate via the Kubernetes API

This is a more secure method because a special kubernetes API is used to sign the certificate.

The features of this method are:

- API-based certificate signing: CSRs are processed through the Kubernetes API without requiring access to the CA’s private key (`ca.key`).
- Not only the cluster administrator can issue client certificates. The right to create CSRs and sign them can be assigned to a specific user.

To create a user using a client certificate issued through the Kubernetes API, follow these steps:

1. Generate the user key:

    ```shell
    openssl genrsa -out myuser.key 2048
    ```

1. Create a CSR file and specify in it the username (`myuser`) and groups to which this user belongs (`mygroup1` and `mygroup2`):

    ```shell
    openssl req -new -key myuser.key -out myuser.csr -subj "/CN=myuser/O=mygroup1/O=mygroup2"
    ```

1. Create a manifest for the CertificateSigningRequest object and save it to a file (`csr.yaml` in this example):

    > In the `request` field, specify the contents of the CSR created in the previous step, encoded in Base64.

    ```yaml
    apiVersion: certificates.k8s.io/v1
    kind: CertificateSigningRequest
    metadata:
    name: demo-client-cert
    spec:
      request: # CSR in Base64
      signerName: "kubernetes.io/kube-apiserver-client"
      expirationSeconds: 7200
      usages:
      - "digital signature"
      - "client auth"
    ```
  
1. Apply the manifest to create a certificate signing request:
  
    ```shell
    d8 k apply -f csr.yaml
    ```

1. Check that the certificate has been approved and issued:

    ```shell
    d8 k get csr demo-client-cert
    ```

    If the certificate is issued, it will have the value `Approved,Issued` in the `CONDITION` column. Example output:

    ```shell
    NAME               AGE     SIGNERNAME                            REQUESTOR          REQUESTEDDURATION   CONDITION
    demo-client-cert   8m24s   kubernetes.io/kube-apiserver-client   kubernetes-admin   120m                Approved,Issued
    ```

    If the certificate is not automatically verified, verify it:

    ```shell
    d8 k certificate approve demo-client-cert
    ```

    Then, confirm that the certificate has been successfully approved.

1. Extract the encoded certificate from the CSR named `demo-client-cert`, decode it from Base64 and save it to the file (`myuser.crt` in this example) created in step 2:

    ```shell
    d8 k get csr demo-client-cert -ojsonpath="{.status.certificate}" | base64 -d > myuser.crt
    ```

Now the certificate can be specified in the config file:

```shell
cat << EOF
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: $(cat /etc/kubernetes/pki/ca.crt | base64 -w0)
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

Working in multi-tenancy mode requires enabling the [Webhook authorization plugin](https://kubernetes.io/docs/reference/access-authn-authz/webhook/) and configuring a `kube-apiserver.` All actions necessary for the multi-tenancy mode are performed **automatically** by the [control-plane-manager](../../modules/control-plane-manager/) module; no additional steps are required.

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

> You can use Dex logs to find out groups and a username if this module is used together with the `user-authn` module (`d8 k -n d8-user-authn logs -l app=dex`); logs available only if the user is authorized.

```shell
cat  <<EOF | 2>&1 d8 k  create --raw  /apis/authorization.k8s.io/v1/subjectaccessreviews -f - | jq .status
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
cat  <<EOF | 2>&1 d8 k --kubeconfig /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml create --raw / -f - | jq .status
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

If you want to grant more privileges to a specific [high-level role](./#current-role-based-model), you only need to create a ClusterRole with the `user-authz.deckhouse.io/access-level: <AccessLevel>` annotation.

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
