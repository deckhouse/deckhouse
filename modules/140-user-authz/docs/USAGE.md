---
title: "The user-authz module: usage"
description: "Examples of granting access with the roles and objects of the user-authz module: administrators, namespaces, projects, ClusterAuthorizationRule, ServiceAccounts, and client certificates."
---

## Example of assigning rights to a cluster administrator

{% alert level="info" %}
The example uses the [granular role-based model](./#granular-role-based-model).
{% endalert %}

To grant access to a cluster administrator, use the role `d8:system:manager` in ClusterRoleBinding.

Example of assigning rights to a cluster administrator (User `jane.doe@example.com`):

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
  name: d8:system:manager
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="The rights that the user will get" %}
The user gets access to cluster-wide resources, to the configuration of Deckhouse Platform modules, and to the namespaces of the enabled Deckhouse Platform modules. The user gets no access to the namespaces of user applications.

The user will be able to:
- View, modify, delete, and create Kubernetes resources and Deckhouse Platform modules;
- Modify module configurations (view, modify, delete, and create `moduleConfig` resources);
- Execute the following commands on pods and services:
  - `kubectl attach`
  - `kubectl exec`
  - `kubectl port-forward`
  - `kubectl proxy`
{% endofftopic %}

## Example of assigning rights to a network administrator

{% alert level="info" %}
The example uses the [granular role-based model](./#granular-role-based-model).
{% endalert %}

To grant a network administrator access to manage the network subsystem of the cluster, use the role `d8:subsystem:network:manager` in ClusterRoleBinding.

Example of assigning rights to a network administrator (User `jane.doe@example.com`):

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
  name: d8:subsystem:network:manager
  apiGroup: rbac.authorization.k8s.io
```

{% offtopic title="The rights that the user will get" %}
The rights that the user will get will be limited to the following list of Deckhouse Platform module namespaces from the `network` subsystem (the actual list depends on the list of modules included in the cluster):

- `d8-cni-cilium`
- `d8-cni-flannel`
- `d8-cni-simple-bridge`
- `d8-ingress-nginx`
- `d8-istio`
- `d8-keepalived`
- `d8-metallb`
- `d8-monitoring`
- `d8-network-gateway`
- `d8-openvpn`
- `d8-service-with-healthchecks`
- `d8-static-routing-manager`
- `d8-system`
- `kube-system`

The user will be able to:
- View, modify, delete, and create *standard* Kubernetes resources in the module namespace from the `network` subsystem.

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

- View, modify, delete, and create the following resources in the modules namespace from the `network` subsystem:

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
  - `KeepalivedInstance`
  - `RoutingTable`

- Modify the configuration of modules (view, change, delete, and create moduleConfig resources) from the `network` subsystem.

  List of modules that the user will be able to manage:
  - `cilium-hubble`
  - `cni-cilium`
  - `cni-flannel`
  - `cni-simple-bridge`
  - `ingress-nginx`
  - `istio`
  - `keepalived`
  - `kube-dns`
  - `kube-proxy`
  - `metallb`
  - `monitoring-ping`
  - `network-gateway`
  - `network-policy-engine`
  - `node-local-dns`
  - `openvpn`
  - `sdn`
  - `service-with-healthchecks`
  - `static-routing-manager`
  
- Execute the following commands with pods and services in the modules namespace from the `network` subsystem:
  - `kubectl attach`
  - `kubectl exec`
  - `kubectl port-forward`
  - `kubectl proxy`
{% endofftopic %}

## Example of assigning administrative rights to a user within a namespace

{% alert level="info" %}
The example uses the [granular role-based model](./#granular-role-based-model).
{% endalert %}

To assign rights to a user to manage application resources within a namespace (including some namespaced Deckhouse Platform module resources such as DexAuthenticator), use the role `d8:namespace:admin` in a RoleBinding in the corresponding namespace. Cluster-wide module configuration is not included.

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
  name: d8:namespace:admin
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
- View, edit, delete, and create the following Deckhouse Platform module resources:
  - `DexAuthenticator`
  - `DexClient`
  - `PodLoggingConfig`
- Execute the following commands for pods and services:
  - `kubectl attach`
  - `kubectl exec`
  - `kubectl port-forward`
  - `kubectl proxy`

The user will **not** get the operations reserved for the `superadmin` level (see the section [Admin level restrictions and superadmin rights](./#admin-level-restrictions-and-superadmin-rights)): minting ServiceAccount tokens, making requests on behalf of ServiceAccounts, modifying platform system resources in this namespace, and connecting to system pods.
{% endofftopic %}

## An example of assigning rights across all namespaces of a project

{% alert level="info" %}
The example uses the [granular role-based model](./#granular-role-based-model) and the [multitenancy-manager](/modules/multitenancy-manager/) module.
{% endalert %}

If the namespaces are grouped into a [project](/modules/multitenancy-manager/), a role can be granted for the whole project at once — it will automatically apply in all of its namespaces, including those created later. To do that, use a [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) in the main namespace of the project instead of RoleBinding:

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: ProjectRoleBinding
metadata:
  name: team-developers
  namespace: my-project
spec:
  subjects:
    - kind: Group
      name: developers
  roleRef:
    kind: ClusterRole
    name: d8:project:user
```

For more details on role bindings in projects, see [the multitenancy-manager module documentation](/modules/multitenancy-manager/usage.html#granting-access-within-a-project).

## An example of a ClusterAuthorizationRule

{% alert level="info" %}
The example uses the [basic role-based model](./#basic-role-based-model).
{% endalert %}

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
    namespace: ci
  - kind: Group
    name: some-group-name
  accessLevel: PrivilegedUser
  portForwarding: true
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

A `User` or `Group` subject is matched by name against the user identity in the token. A locally managed identity is indistinguishable from one asserted by an external authentication provider. Because of that, you can't create a [User](/modules/user-authn/cr.html#user) whose `spec.email` or a [Group](/modules/user-authn/cr.html#group) whose `spec.name` matches a subject of an existing ClusterAuthorizationRule or AuthorizationRule. This prevents any unnoticed attempt to grant the identity privileges.

If the match is intentional, for example, if the ClusterAuthorizationRule has been written in advance, set the `user-authz.deckhouse.io/allow-authorization-rule-collision: "true"` annotation on the User or Group. The annotation only acknowledges the name collision. It does not let a requester assign roles they cannot cover or that sit outside their can-assign range.

Deleting a ClusterAuthorizationRule uses the same can-assign check as creating or updating it. You can remove a grant only if you could assign those roles. An update is checked against both the previous and the new grants, so lowering `SuperAdmin` to `User` still requires being able to assign `SuperAdmin`.

Write an email for the `User` subject in lowercase. It's recorded to the token in lowercase so a subject spelled `Admin@Example.com` won't match `admin@example.com`. Names of `Group` subjects are matched exactly, because group names are not lowercased anywhere.

This restriction is applied when a User or Group is created and when its `spec.email` or `spec.name` is changed. Deleting a User or Group whose name a rule still grants privileges to is allowed with a warning: the name stays granted, and recreating the object restores the privileges. The restriction doesn't prevent adding existing users or groups to a ClusterAuthorizationRule or AuthorizationRule. If a matching User or Group already exists, it receives the privileges immediately after it's added to the rule.

## Managing permissions via CLI

{% alert level="info" %}
The `d8 iam access` command uses the [basic role-based model](./#basic-role-based-model).
{% endalert %}

The [`d8 iam access`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-iam-access) command provides a convenient way to manage authorization rules without writing YAML manifests. `d8 iam access grant` creates [ClusterAuthorizationRule](cr.html#clusterauthorizationrule) and [AuthorizationRule](cr.html#authorizationrule) objects, and `d8 iam access revoke` deletes only the objects created by `grant`.

The command works with the basic role-based model, and with users and groups only. Grant roles of the [granular model](./#granular-role-based-model) with RoleBinding and ClusterRoleBinding objects. Give access to a ServiceAccount with a manifest, see [Creating a ServiceAccount for a machine and granting it access](#creating-a-serviceaccount-for-a-machine-and-granting-it-access). The `d8 iam get user` and `d8 iam list users` commands count only ClusterAuthorizationRule and AuthorizationRule objects. To see all the access of a subject, use [SubjectAccessReport](faq.html#how-do-i-find-out-what-a-specific-user-group-or-serviceaccount-is-allowed-to-do).

Available access levels: `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`, `ClusterAdmin`, `SuperAdmin`.

Examples of granting permissions with the `d8 iam access` command:

Grant the Admin role to a user in the `dev` namespace:

```shell
d8 iam access grant user anton --access-level Admin -n dev
```

Grant the Admin role to a user in multiple namespaces:

```shell
d8 iam access grant user anton --access-level Admin -n dev -n stage
```

Grant a cluster-wide role to a user (excluding system namespaces):

```shell
d8 iam access grant user anton --access-level ClusterAdmin --scope cluster
```

Grant a cluster-wide role to a user in all namespaces (including system namespaces):

```shell
d8 iam access grant user anton --access-level ClusterAdmin --scope all-namespaces
```

Grant a role to group members by a namespace label selector:

```shell
d8 iam access grant group admins --access-level Editor --scope labels=team=platform,tier=prod
```

`--scope cluster` leaves out the system namespaces only in [multi-tenancy mode](configuration.html#parameters-enablemultitenancy); without it, the rule grants its access level in every namespace, system ones included. `--scope labels=...` and `--scope all-namespaces` need multi-tenancy mode: without it, the rule is refused. A rule created with `-n` is an AuthorizationRule and applies only in its namespace.

Grant a role to group members with additional capabilities:

```shell
d8 iam access grant group admins --access-level Editor -n dev --port-forwarding --allow-scale
```

Preview the manifest without applying it:

```shell
d8 iam access grant user anton --access-level Admin -n dev --dry-run -o yaml
```

Revoke a user's permissions in a namespace:

```shell
d8 iam access revoke user anton --access-level Admin -n dev
```

Revoke a user's cluster-wide permissions:

```shell
d8 iam access revoke user anton --access-level ClusterAdmin --scope cluster
```

The access level and the scope must match the original grant.

List all access rules in the cluster:

```shell
d8 iam list rules
```

Show detailed information about a specific rule. Name a ClusterAuthorizationRule as `CAR/<NAME>` and an AuthorizationRule as `AR/<NAMESPACE>/<NAME>`:

```shell
d8 iam get rule CAR/<NAME>
d8 iam get rule AR/<NAMESPACE>/<NAME>
```

## Example of granting access to all namespaces

{% alert level="info" %}
The example refers to the [basic role-based model](./#basic-role-based-model).
{% endalert %}

In [multi-tenancy](configuration.html#parameters-enablemultitenancy) mode (`userAuthz.enableMultiTenancy`), namespace-based restrictions are configured using the [ClusterAuthorizationRule](cr.html#clusterauthorizationrule-v1-spec-namespaceselector) resource fields.

Configuration options:

- **Allow access to all namespaces, including system namespaces** (`kube-*`, `d8-*`, `default`). To allow access to all namespaces, including system namespaces,  set `spec.namespaceSelector.matchAny: true`. If the `namespaceSelector` parameter is specified, the values of the deprecated `limitNamespaces` and `allowAccessToSystemNamespaces` parameters are **ignored**. Access to the system namespaces with the `Admin` or `ClusterAdmin` level is equivalent to a cluster administrator: those namespaces hold the ServiceAccounts of the platform (see [Access of system and subsystem roles to the namespaces of the modules](./#access-of-system-and-subsystem-roles-to-the-namespaces-of-the-modules)).

  Example:

  ```yaml
  apiVersion: deckhouse.io/v1
  kind: ClusterAuthorizationRule
  metadata:
    name: all-namespaces-including-system
  spec:
    subjects:
    - kind: User
      name: user@example.com
    accessLevel: Editor
    namespaceSelector:
      matchAny: true
  ```

- **Allow access to all namespaces except system namespaces**. To grant a user access to all namespaces except system namespaces, do not specify `namespaceSelector` or the deprecated `limitNamespaces` and `allowAccessToSystemNamespaces` parameters when creating a ClusterAuthorizationRule. For a list of system namespaces, see the [CR field description](cr.html#clusterauthorizationrule-v1-spec-namespaceselector).

{% alert level="info" %}
The `SuperAdmin` access level **does not lift** the namespace restrictions specified in the `namespaceSelector` and `limitNamespaces` parameters. If you need to grant access to all namespaces, specify the scope explicitly, including via [`namespaceSelector.matchAny`](cr.html#clusterauthorizationrule-v1-spec-namespaceselector).
{% endalert %}

If several `ClusterAuthorizationRule` resources match the same subject, the allowed namespaces are **combined**; the effective `accessLevel` is the **most powerful** among all matching rules. For details, refer to the [FAQ](faq.html#what-if-there-are-two-clusterauthorizationrules-matching-a-single-user).

{% alert level="warning" %}
Namespace restrictions from ClusterAuthorizationRule are enforced by the authorization webhook. The API server calls it before RBAC with `failurePolicy: Deny`, so if the webhook is unavailable, the requests it has to answer are **denied**. The webhook does not check control plane components, kubelets, and the ServiceAccounts of the `kube-system` namespace and of the namespaces whose names start with `d8-`. Namespace restrictions do not apply to them. For details, see the [module description](./#basic-role-based-model).
{% endalert %}

{% offtopic title="Granular role-based model" %}
In the granular model, bind `d8:namespace:*` roles with a RoleBinding in each namespace, or use a [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) to cover every namespace of a project. `d8:system:*` / `d8:subsystem:*` roles do not cover namespaces for user workloads; they only apply to system namespaces (`d8-*`, `kube-*`) within the subsystem.
{% endofftopic %}

## Creating a user

There are two types of users in Kubernetes:

* Service accounts managed by Kubernetes via the API;
* Regular users and groups managed by some external tool that the cluster administrator configures. There are many authentication mechanisms and, accordingly, many ways to create users. Currently, two authentication methods are supported:
  * Via the `user-authn` module. The module supports the following external providers and authentication protocols: GitHub, GitLab, Bitbucket Cloud, Atlassian Crowd, LDAP, OIDC, SAML, and local users (the [User](/modules/user-authn/cr.html#user) object). More details — in the documentation of the [`user-authn`](/modules/user-authn/) module.
  * Via the [certificates](#how-to-create-a-user-using-a-client-certificate).

When issuing the authentication certificate, you need to specify the name (`CN=<name>`), the required number of groups (`O=<group>`), and sign it using the root CA of the cluster. It is this mechanism that authenticates you in the cluster when, for example, you use kubectl on a master node.

### Creating a ServiceAccount for a machine and granting it access

{% alert level="info" %}
The example uses the [basic role-based model](./#basic-role-based-model).
{% endalert %}

You may need to create a ServiceAccount with access to the Kubernetes API when, for example, an application is deployed using a CI system.

1. Create a namespace for CI, for example `ci`, and a ServiceAccount in it:

   ```shell
   d8 k create -f - <<EOF
   apiVersion: v1
   kind: Namespace
   metadata:
     name: ci
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: gitlab-runner-deploy
     namespace: ci
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: gitlab-runner-deploy-token
     namespace: ci
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
       namespace: ci
     accessLevel: Editor
   EOF
   ```

   If multi-tenancy mode is enabled (the [`enableMultiTenancy`](configuration.html#parameters-enablemultitenancy) parameter), limit the namespaces the ServiceAccount has access to with the [`namespaceSelector`](cr.html#clusterauthorizationrule-v1-spec-namespaceselector) parameter.

   {% alert level="warning" %}
   Do not create ServiceAccounts for external systems in `kube-system` or in namespaces whose names start with `d8-`. The authorization webhook does not check the ServiceAccounts of these namespaces, so the namespace limits of a ClusterAuthorizationRule (`namespaceSelector`, `limitNamespaces`, `allowAccessToSystemNamespaces`) do not apply to them. Their access level applies in every namespace of the cluster, including system namespaces.
   {% endalert %}

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
          --server=https://$(d8 k get endpointslices -l kubernetes.io/service-name=kubernetes -o json | jq -rc '.items[0] | "\(.endpoints[0].addresses[0]):\(.ports[0].port)"') \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * If there is no direct access to the API server, use one of the following options:
      * enable access to the API-server over the Ingress controller (the [apiserver.publishAPI](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi) parameter of the `control-plane-manager` module) and specify the addresses from which requests originate (the [whitelistSourceRanges](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi-ingress-whitelistsourceranges) parameter);
      * specify addresses from which requests will originate in a separate Ingress controller (the [acceptRequestsFrom](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) parameter).

   * If a non-public CA is used:

     1. Get the CA certificate from the Secret of the `kubernetes-api` Ingress in the `kube-system` namespace. It is the certificate of the `api.%s` domain:

        ```shell
        d8 k -n kube-system get secrets -o json \
          $(d8 k -n kube-system get ing kubernetes-api -o jsonpath="{.spec.tls[0].secretName}") \
          | jq -rc '.data."ca.crt" // .data."tls.crt"' \
          | base64 -d > /tmp/ca.crt
        ```

     2. Generate the `cluster` section (an external domain and a CA for access are used):

        ```shell
        d8 k config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(d8 k -n kube-system get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * If a public CA is used. Generate the `cluster` section (an external domain is used for access):

     ```shell
     d8 k config set-cluster $CLUSTER_NAME \
       --server=https://$(d8 k -n kube-system get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
       --kubeconfig=$FILE_NAME
     ```

   * If the API server is published through Gateway API, the cluster has the `kubernetes-api` HTTPRoute in the `kube-system` namespace instead of the Ingress. Take the host from it instead of `.spec.rules[].host` in the commands above:

     ```shell
     d8 k -n kube-system get httproute kubernetes-api -o jsonpath='{.spec.hostnames[0]}'
     ```

     Take the name of the Secret with the CA certificate from the `control-plane-manager` ListenerSet instead of `.spec.tls[0].secretName`:

     ```shell
     d8 k -n kube-system get listenerset control-plane-manager -o jsonpath='{.spec.listeners[?(@.name=="api")].tls.certificateRefs[0].name}'
     ```

1. Generate the `user` section using the token from the Secret's ServiceAccount in the kubectl configuration file:

   ```shell
   d8 k config set-credentials $USER_NAME \
     --token=$(d8 k -n ci get secret gitlab-runner-deploy-token -o json |jq -r '.data["token"]' | base64 -d) \
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
This method is recommended for system needs (authentication of kubelets, control plane components, etc.). If you need to create a "normal" user (e.g. with console access, `kubectl`, etc.), use [kubeconfig generation](/modules/user-authn/faq.html#how-to-generate-a-kubeconfig-and-access-kubernetes-api).
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

1. Make sure you have access to a master node. The cluster CA files `ca.crt` and `ca.key` are in `/etc/kubernetes/pki/` on it.
1. Generate the user key:

    ```shell
    openssl ecparam -name prime256v1 -genkey -out myuser.key
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
    openssl ecparam -name prime256v1 -genkey -out myuser.key
    ```

1. Create a CSR file and specify in it the username (`myuser`) and groups to which this user belongs (`mygroup1` and `mygroup2`):

    ```shell
    openssl req -new -key myuser.key -out myuser.csr -subj "/CN=myuser/O=mygroup1/O=mygroup2"
    ```

1. Create a manifest for the CertificateSigningRequest object and save it to a file (`csr.yaml` in this example):

    {% alert level="info" %}
    In the `request` field, replace `<CSR_BASE64>` with the contents of `myuser.csr` encoded in Base64 without line breaks, for example, the output of `base64 < myuser.csr | tr -d '\n'`.
    {% endalert %}

    ```yaml
    apiVersion: certificates.k8s.io/v1
    kind: CertificateSigningRequest
    metadata:
      name: demo-client-cert
    spec:
      request: <CSR_BASE64>
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

    ```console
    NAME               AGE     SIGNERNAME                            REQUESTOR          REQUESTEDDURATION   CONDITION
    demo-client-cert   8m24s   kubernetes.io/kube-apiserver-client   kubernetes-admin   120m                Approved,Issued
    ```

    If the certificate is not automatically verified, verify it:

    ```shell
    d8 k certificate approve demo-client-cert
    ```

    Then, confirm that the certificate has been successfully approved.

1. Extract the encoded certificate from the CSR named `demo-client-cert`, decode it from Base64 and save it to a file (`myuser.crt` in this example):

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

{% alert level="info" %}
The example uses the [basic role-based model](./#basic-role-based-model).
{% endalert %}

To grant access to the created user, create a ClusterAuthorizationRule.

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

Working in multi-tenancy mode requires enabling the [Webhook authorization plugin](https://kubernetes.io/docs/reference/access-authn-authz/webhook/) and configuring a `kube-apiserver.` All actions necessary for the multi-tenancy mode are performed **automatically** by the [control-plane-manager](/modules/control-plane-manager/) module; no additional steps are required.

Multi-tenancy mode is available in the editions listed in the [parameter description](configuration.html#parameters-enablemultitenancy). The API server is configured automatically while the [`controlPlaneConfigurator.enabled`](configuration.html#parameters-controlplaneconfigurator-enabled) parameter is `true`.

Changes to the `kube-apiserver` manifest that will occur after enabling multi-tenancy mode:

* The `--authorization-mode=Node,RBAC` flag is replaced with the structured authorization configuration (`--authorization-config=/etc/kubernetes/deckhouse/extra-files/authorization-config.yaml`) with the `Node`, `Webhook`, and `RBAC` authorizers;
* The `kubeconfig` for the authorization webhook will be created at `/etc/kubernetes/deckhouse/extra-files/webhook-config.yaml`. Both files are in the `/etc/kubernetes/deckhouse/extra-files` directory, which is already mounted into `kube-apiserver`.

## How do I check that a user has access?

Execute the command below with the following parameters:

* `resourceAttributes` (the same as in RBAC) - target resources;
* `user` - the name of the user;
* `groups` - user groups;

{% alert level="info" %}
If this module is used together with the `user-authn` module, you can find the groups and the username of a user in the Dex logs (`d8 k -n d8-user-authn logs -l app=dex`). The logs have them only after the user has signed in.
{% endalert %}

```shell
cat  <<EOF | 2>&1 d8 k  create --raw  /apis/authorization.k8s.io/v1/subjectaccessreviews -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "dev",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "user@example.com",
    "groups": [
      "developers"
    ]
  }
}
EOF
```

You will see if access is allowed and what role is used:

```json
{
  "allowed": true,
  "reason": "RBAC: allowed by RoleBinding \"<BINDING_NAME>/dev\" of ClusterRole \"<ROLE_NAME>\" to User \"user@example.com\""
}
```

The answer of the API server already includes the decision of the authorization webhook. To see the answer of the webhook alone, run the following command as root on a master node:

```shell
cat  <<EOF | 2>&1 d8 k --kubeconfig /etc/kubernetes/deckhouse/extra-files/webhook-config.yaml create --raw / -f - | jq .status
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReview",
  "spec": {
    "resourceAttributes": {
      "namespace": "dev",
      "verb": "watch",
      "version": "v1",
      "resource": "pods"
    },
    "user": "user@example.com",
    "groups": [
      "developers"
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
  "reason": "either you have no access to the namespace or the namespace does not exist"
}
```

## Customizing rights of high-level roles

If you want to grant more privileges to a specific [high-level role](./#basic-role-based-model), you only need to create a ClusterRole with the `user-authz.deckhouse.io/access-level: <AccessLevel>` annotation.

The module automatically sets the `user-authz.deckhouse.io/access-level` label on such ClusterRoles and includes their rules in the aggregated ClusterRole `user-authz:<access-level>:custom`. The label is internal: the module keeps it equal to the annotation and removes it when the annotation is absent, so there is no need to set it by hand. In audit logs, access granted through an annotated ClusterRole is attributed to that aggregated role.

The annotation accepts `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`, and `ClusterAdmin`. Other values, including `SuperAdmin`, are ignored. In the name of the aggregated role, the level is written in kebab case, for example, `user-authz:cluster-editor:custom`. The rules of a level are also included in the aggregated roles of every higher level.

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
