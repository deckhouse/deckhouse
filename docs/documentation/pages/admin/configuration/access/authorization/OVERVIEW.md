---
title: "Authorization"
permalink: en/admin/configuration/access/authorization/
description: "Configure authorization and access control for Deckhouse Platform using RBAC. Manage user permissions, roles, and service accounts for secure cluster access."
---

In Deckhouse Platform (DP),
authorization is based on the standard Kubernetes Role-Based Access Control (RBAC) mechanism.
This allows for flexible access control for different users, groups, and service accounts,
ensuring security and operational control in the cluster.

DP supports two role models:

- [Granular](../authorization/rbac-experimental.html) (recommended). Access permissions are configured the standard Kubernetes RBAC way:
  by creating [RoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/role-binding-v1/) or [ClusterRoleBinding](https://kubernetes.io/docs/reference/kubernetes-api/authorization-resources/cluster-role-binding-v1/) resources — or, for project-wide access, the [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) and [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) resources of the `multitenancy-manager` module.
- [Basic](../authorization/rbac-current.html). The end-to-end authorization [subsystem](../../../../glossary.html#subsystem) extends the standard RBAC mechanism
  with custom resources — [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) and [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule). Support for this model will be discontinued in future releases.

Both models are supported by the [`user-authz`](/modules/user-authz/) module and can be used at the same time — the permissions they grant are summed up (for details, see the [`user-authz` module FAQ](/modules/user-authz/faq.html#can-the-basic-and-the-granular-role-based-models-be-used-at-the-same-time)).
The choice of model depends on security requirements and usage scenarios.

## Combined use of ClusterAuthorizationRule, AuthorizationRule, and RBAC

If multitenancy mode is enabled in the cluster ([`enableMultiTenancy: true`](/modules/user-authz/configuration.html#parameters-enablemultitenancy)), a user's effective permissions are the union of the permissions from all of the following sources:

- **permissions from ClusterAuthorizationRule** — apply only within the namespaces allowed by the `namespaceSelector` parameter (or the deprecated `limitNamespaces` parameter);
- **permissions from AuthorizationRule** — apply within the namespace where the AuthorizationRule resource is created;
- **permissions from plain RoleBinding resources** — apply within the namespace where the RoleBinding resource is created;
- **permissions from plain ClusterRoleBinding resources** not created by the `user-authz` module — apply cluster-wide.

The namespace restrictions set in a ClusterAuthorizationRule apply only to the permissions granted by that ClusterAuthorizationRule. They do not override permissions granted via a RoleBinding, ClusterRoleBinding, or AuthorizationRule in other namespaces. At the same time, the permissions granted by the ClusterAuthorizationRule do not extend to those other namespaces.

The namespace restrictions are enforced by the authorization webhook, so they do not apply to the identities that the webhook does not check: control plane components, kubelets, and the ServiceAccounts of `kube-system` and of the namespaces whose names start with `d8-`. A ClusterAuthorizationRule that names such a ServiceAccount, directly or through a group such as `system:serviceaccounts`, grants its access level in every namespace. Create the ServiceAccounts of external systems in ordinary namespaces.

For example, if a user has a ClusterAuthorizationRule with `accessLevel: Editor` restricted to the `ns-a` namespace, as well as a RoleBinding with the `view` role in the `ns-b` namespace, they get `Editor`-level permissions in `ns-a` and read-only permissions in `ns-b`. The permissions granted via the RoleBinding are not restricted by the ClusterAuthorizationRule, and the `Editor` access level set by the ClusterAuthorizationRule does not extend to the `ns-b` namespace.

## Who gets access and when

There are two scenarios for granting access in DP:

- Granting access to users for working via command-line clients, web UI,
  and other tools used for administration, development, and cluster management.
- Granting access to service accounts for automating tasks such as application deployment and updates
  (most commonly using the IaC approach).
  Examples of such services include CI/CD systems, monitoring systems, and others.

After successful authentication, users and service accounts are granted access to cluster resources
based on the configured authorization settings.

### User authentication

DP supports multiple user authentication methods.
For details, refer to [User authentication](../authentication/).

### Service account authentication

In Kubernetes, service accounts (ServiceAccount) are special accounts used to automate tasks and interact with the cluster API.
They enable applications and services to securely communicate with the Kubernetes API.

{% alert level="warning" %}
Do not create ServiceAccounts for external systems in `kube-system` or in namespaces whose names start with `d8-`. The authorization webhook does not check the ServiceAccounts of these namespaces, so the namespace limits of a ClusterAuthorizationRule (`namespaceSelector`, `limitNamespaces`, `allowAccessToSystemNamespaces`) do not apply to them. Their access level applies in every namespace of the cluster, including [system namespaces](../../../../glossary.html#system-namespace).
{% endalert %}

You may need to create a ServiceAccount with access to the Kubernetes API, for example, when setting up application deployment through a CI system. To create a ServiceAccount and grant it access, follow these steps:

{% alert level="info" %}
The example uses the basic [role model](../../../../glossary.html#rbac).
{% endalert %}

1. Create a namespace for CI and a ServiceAccount in it:

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

1. Grant the ServiceAccount the necessary permissions using the [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule) custom resource:

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

   If multitenancy mode is enabled (the [`enableMultiTenancy`](/modules/user-authz/configuration.html#parameters-enablemultitenancy) parameter of the `user-authz` module), limit the namespaces available to the ServiceAccount with the [`namespaceSelector`](/modules/user-authz/cr.html#clusterauthorizationrule-v1-spec-namespaceselector) parameter.

1. Set the variable values (they are used later) by running the following commands (**insert your own values**):

   ```shell
   export CLUSTER_NAME=my-cluster
   export USER_NAME=gitlab-runner-deploy.my-cluster
   export CONTEXT_NAME=${CLUSTER_NAME}-${USER_NAME}
   export FILE_NAME=kube.config
   ```

1. Generate the `cluster` section in the kubectl configuration file:

   Use one of the following options to access the cluster API server:

   * If there is direct access to the API server:
     1. Get the Kubernetes cluster CA certificate:

        ```shell
        d8 k get cm kube-root-ca.crt -o jsonpath='{ .data.ca\.crt }' > /tmp/ca.crt
        ```

     1. Generate the `cluster` section (the API server IP address is used for access):

        ```shell
        d8 k config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(d8 k get endpointslices -l kubernetes.io/service-name=kubernetes -o json | jq -rc '.items[0] | "\(.endpoints[0].addresses[0]):\(.ports[0].port)"') \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * If there is no direct access to the API server, use one of the following options:
      * enable access to the API server through the [Ingress controller](../../../../glossary.html#ingress-controller) (the [apiserver.publishAPI](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi) parameter of the `control-plane-manager` module) and specify the addresses requests will come from (the [whitelistSourceRanges](/modules/control-plane-manager/configuration.html#parameters-apiserver-publishapi-ingress-whitelistsourceranges) parameter);
      * specify the addresses requests will come from in a separate Ingress controller (the [acceptRequestsFrom](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) parameter).

   * If a non-public CA is used:

     1. Get the CA certificate from the Secret of the `kubernetes-api` Ingress in the `kube-system` namespace. This is the certificate of the `api.%s` domain:

        ```shell
        d8 k -n kube-system get secrets -o json \
          $(d8 k -n kube-system get ing kubernetes-api -o jsonpath="{.spec.tls[0].secretName}") \
          | jq -rc '.data."ca.crt" // .data."tls.crt"' \
          | base64 -d > /tmp/ca.crt
        ```

     2. Generate the `cluster` section (an external domain and the CA are used for access):

        ```shell
        d8 k config set-cluster $CLUSTER_NAME --embed-certs=true \
          --server=https://$(d8 k -n kube-system get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
          --certificate-authority=/tmp/ca.crt \
          --kubeconfig=$FILE_NAME
        ```

   * If a public CA is used, generate the `cluster` section (an external domain is used for access):

     ```shell
     d8 k config set-cluster $CLUSTER_NAME \
       --server=https://$(d8 k -n kube-system get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
       --kubeconfig=$FILE_NAME
     ```

   * If the API server is published via Gateway API, the cluster has the `kubernetes-api` HTTPRoute in the `kube-system` namespace instead of the Ingress. In the commands above, use the host from it instead of `.spec.rules[].host`:

     ```shell
     d8 k -n kube-system get httproute kubernetes-api -o jsonpath='{.spec.hostnames[0]}'
     ```

     Take the name of the Secret with the CA certificate from the `control-plane-manager` ListenerSet instead of `.spec.tls[0].secretName`:

     ```shell
     d8 k -n kube-system get listenerset control-plane-manager -o jsonpath='{.spec.listeners[?(@.name=="api")].tls.certificateRefs[0].name}'
     ```

1. Generate the `user` section with the token from the ServiceAccount Secret in the kubectl configuration file:

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
