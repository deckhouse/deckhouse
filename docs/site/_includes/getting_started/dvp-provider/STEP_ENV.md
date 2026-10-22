{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

To deploy Deckhouse Platform on built-in virtualization environment and enable integration (for example, ordering nodes for a nested cluster within the virtualization environment), you must perform some preliminary configuration. Create a user (ServiceAccount), assign permissions, and obtain a kubeconfig.

The resources of the new cluster are created in a single namespace of the virtualization cluster, and the ServiceAccount gets permissions only in this namespace.
If the virtualization cluster is shared with other teams, choose the namespace of your project.
To create the ServiceAccount and assign a role to it, you need [administrator permissions in the chosen namespace](/products/kubernetes-platform/documentation/v1/admin/configuration/access/authorization/rbac-experimental.html#use-roles).

1. Create a user (ServiceAccount and token) by running:

   ```bash
   d8 k create -f -<<EOF
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: sa-demo
     namespace: <NAMESPACE>
   ---
   apiVersion: v1
   kind: Secret
   metadata:
     name: sa-demo-token
     namespace: <NAMESPACE>
     annotations:
       kubernetes.io/service-account.name: sa-demo
   type: kubernetes.io/service-account-token
   EOF
   ```

   Replace `<NAMESPACE>` with the chosen namespace.
{%- if page.gs_installer %}
   Specify the same namespace in the "Project" field of the cloud provider connection parameters at the ["Basic installation parameters"](step5.html) step.
{%- else %}
   Specify the same namespace in the `provider.namespace` parameter of the `config.yml` file at the ["Installation"](step4.html) step.
{%- endif %}

1. Assign a role to the user by running:

   ```bash
   d8 k create -f -<<EOF
   apiVersion: rbac.authorization.k8s.io/v1
   kind: RoleBinding
   metadata:
     name: sa-demo-rb
     namespace: <NAMESPACE>
   subjects:
     - kind: ServiceAccount
       name: sa-demo
       namespace: <NAMESPACE>
   roleRef:
     kind: ClusterRole
     name: d8:use:role:manager
     apiGroup: rbac.authorization.k8s.io
   EOF
   ```

1. Enable kubeconfig issuance via API. Open the `user-authn` module settings (create a [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) resource named `user-authn` if it does not exist):

   ```shell
   d8 k edit mc user-authn
   ```

1. Add the following section to the `settings` block and save:

   ```yaml
   publishAPI:
     enabled: true
   ```

1. Generate a kubeconfig to be used in the cluster initial configuration file in the next step:

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
       namespace: <NAMESPACE>
     name: sa-demo-context
   current-context: sa-demo-context
   kind: Config
   preferences: {}
   users:
   - name: sa-demo
     user:
       token: $(d8 k get secret sa-demo-token -n <NAMESPACE> -o json | jq -rc .data.token | base64 -d)
   EOF
   ```

   Encode the generated kubeconfig file using Base64 encoding (it appears in the initial configuration file as follows):

   ```bash
   base64 kubeconfig | tr -d '\n'
   ```
