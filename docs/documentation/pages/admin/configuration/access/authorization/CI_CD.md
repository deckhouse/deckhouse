---
title: "Access for CI/CD"
permalink: en/admin/configuration/access/authorization/ci_cd.html
description: "Configure CI/CD access to Kubernetes cluster in Deckhouse Kubernetes Platform. ServiceAccount setup, kubeconfig generation, and automated deployment access configuration."
---

To grant access to the Kubernetes cluster API for CI/CD systems such as GitLab Runner, Jenkins, and others,
create a ServiceAccount, assign the necessary permissions, and generate a kubeconfig file.
This file will be used to connect to the cluster API.

To set up access to the Kubernetes cluster API for a CI/CD system, follow these steps:

1. Create a ServiceAccount in the `d8-service-accounts` namespace:

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

1. Grant permissions to the ServiceAccount as described in [Granting permissions to users and service accounts](granting.html).

   For the current role model:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: ClusterAuthorizationRule
   metadata:
     name: gitlab-admin-access
   spec:
     subjects:
     - kind: ServiceAccount
       name: gitlab-runner-deploy
       namespace: d8-service-accounts
     accessLevel: SuperAdmin
     portForwarding: true
   ```

   For the experimental role model:

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: gitlab-admin-access
   subjects:
   - kind: ServiceAccount
     name: gitlab-runner-deploy
     namespace: d8-service-accounts
   roleRef:
     kind: ClusterRole
     name: d8:manage:all:manager
     apiGroup: rbac.authorization.k8s.io
    ```

1. Define variables to be used in the following commands (**replace with your own values**):

   ```shell
   export CLUSTER_NAME=my-cluster
   export USER_NAME=gitlab-runner-deploy.my-cluster
   export CONTEXT_NAME=${CLUSTER_NAME}-${USER_NAME}
   export FILE_NAME=kube.config
   ```

1. Generate the `cluster` section in the `kubectl` configuration file.
   Use one of the following methods depending on how the API server is accessed:

   - If the API server is directly accessible:
     - Download the cluster’s CA certificate:

       ```shell
       d8 k get cm kube-root-ca.crt -o jsonpath='{ .data.ca\.crt }' > /tmp/ca.crt
       ```

     - Generate the `cluster` section using the API server’s IP address:

       ```shell
       d8 k config set-cluster $CLUSTER_NAME --embed-certs=true \
         --server=https://$(d8 k get ep kubernetes -o json | jq -rc '.subsets[0] | "\(.addresses[0].ip):\(.ports[0].port)"') \
         --certificate-authority=/tmp/ca.crt \
         --kubeconfig=$FILE_NAME
       ```

   - If the API server is not directly accessible, use one of the following options:
     - Enable access to the API server via the Ingress controller using the [`publishAPI`](/modules/user-authn/configuration.html#parameters-publishapi) parameter, and specify the request source IP addresses using the [`whitelistSourceRanges`](/modules/user-authn/configuration.html#parameters-publishapi-whitelistsourceranges) parameter.
     - Alternatively, specify the request source IP addresses in a separate Ingress controller using the [`acceptRequestsFrom`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) parameter.

   - **If using a non-public CA**:
     - Extract the CA certificate from the secret used for the `api.%s` domain:

       ```shell
       d8 k -n d8-user-authn get secrets -o json \
         $(d8 k -n d8-user-authn get ing kubernetes-api -o jsonpath="{.spec.tls[0].secretName}") \
         | jq -rc '.data."ca.crt" // .data."tls.crt"' \
         | base64 -d > /tmp/ca.crt
       ```

     - Generate the `cluster` section using the external domain and the CA:

       ```shell
       d8 k config set-cluster $CLUSTER_NAME --embed-certs=true \
         --server=https://$(d8 k -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
         --certificate-authority=/tmp/ca.crt \
         --kubeconfig=$FILE_NAME
       ```

   - **If using a public CA**:
     - Generate the `cluster` section using the external domain:

       ```shell
       d8 k config set-cluster $CLUSTER_NAME \
         --server=https://$(d8 k -n d8-user-authn get ing kubernetes-api -ojson | jq '.spec.rules[].host' -r) \
         --kubeconfig=$FILE_NAME
       ```

1. Generate the `user` section with the token from the ServiceAccount secret in the `kubectl` configuration file:

   ```shell
   d8 k config set-credentials $USER_NAME \
     --token=$(d8 k -n d8-service-accounts get secret gitlab-runner-deploy-token -o json |jq -r '.data["token"]' | base64 -d) \
     --kubeconfig=$FILE_NAME
   ```

1. Generate the context in the `kubectl` configuration file:

   ```shell
   d8 k config set-context $CONTEXT_NAME \
     --cluster=$CLUSTER_NAME --user=$USER_NAME \
     --kubeconfig=$FILE_NAME
   ```

1. Set the newly created context as the default:

   ```shell
   d8 k config use-context $CONTEXT_NAME --kubeconfig=$FILE_NAME
   ```

You can now use the generated `$FILE_NAME` kubeconfig file to connect to the Kubernetes cluster API from your CI/CD system, such as GitLab Runner or Jenkins.
