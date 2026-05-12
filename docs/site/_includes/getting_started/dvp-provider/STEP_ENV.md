{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

To deploy Deckhouse Kubernetes Platform on DVP, perform the initial setup in the virtualization system. Create a user (ServiceAccount), assign permissions, and obtain a kubeconfig.

1. Create a user (ServiceAccount and token) by running:

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

1. Assign a role to the user by running:

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

1. Enable kubeconfig issuance via API. Open the `user-authn` module settings (create a [ModuleConfig](../../../documentation/v1/reference/api/cr.html#moduleconfig) resource named `user-authn` if it does not exist):

   ```shell
   d8 k edit mc user-authn
   ```

1. Add the following section to the `settings` block and save:

   ```yaml
   publishAPI:
     enabled: true
   ```

1. Generate a kubeconfig in [the kubeconfigurator web interface](/products/kubernetes-platform/documentation/v1/user/web/kubeconfig.html). The interface URL depends on [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) (e.g., `kubeconfig.kube.my` for template `%s.kube.my`).
