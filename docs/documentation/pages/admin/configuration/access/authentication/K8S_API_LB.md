---
title: "Accessing the Kubernetes API via load balancer"
permalink: en/admin/configuration/access/authentication/k8s-api-lb.html
description: "Configure authenticated access to Kubernetes API through load balancer in Deckhouse Kubernetes Platform. Secure kubectl access via Ingress controller with authentication."
---

DKP allows using authentication when accessing the Kubernetes API. In this case, a user can generate a `kubectl` configuration via the DKP kubeconfig web interface to securely access the Kubernetes API through a traffic balancer (Ingress controller).

To configure access, follow these steps:

1. Enable Kubernetes API publishing. To do this, set the parameter [`publishAPI.enabled: true`](/modules/user-authn/configuration.html#parameters-publishapi-enabled) in the `user-authn` module settings or via the Deckhouse admin web interface.

   Example module configuration:

   ```yaml
   spec:
     enabled: true
     version: 2
     settings:
       publishAPI:
         enabled: true
   ```

1. Open the [kubeconfig](../../../../user/web/kubeconfig.html) web interface.  
   The kubeconfig generation interface in DKP is automatically activated after enabling the `publishAPI` parameter in the `user-authn` module.  
   This web interface is available at the following URL:

   ```console
   https://kubeconfig.<publicDomainTemplate>
   ```

   For example, if `publicDomainTemplate` is `%s.kube.my`, the URL will be `https://kubeconfig.kube.my`.

1. Generate the `kubectl` configuration.  
   After logging into the kubeconfig interface, the user will receive a set of commands to configure `kubectl`.  
   These commands can be copied and pasted into the terminal.  
   Authentication will be performed using an OIDC token issued by Dex.  
   If the provider supports session renewal, the configuration will include a `refresh token`, allowing access to be extended without re-authentication.

1. Configure multiple API access points.  
   In the [`user-authn` module configuration](/modules/user-authn/configuration.html#parameters-kubeconfiggenerator), you can specify multiple connection points (kube-apiserver), each with its own description and CA certificates.  
   This may be useful if the cluster is accessible through different networks — for example, via VPN or public IP:

   ```yaml
   settings:
     kubeconfigGenerator:
     - id: direct
       masterURI: https://159.89.5.247:6443
       description: "Direct access to kubernetes API"
   ```

## How API access protection works in Kubernetes

In Deckhouse Kubernetes Platform, you can safely expose the Kubernetes API externally using an Ingress controller while maintaining access control.
API exposure and authentication configuration are handled via the [`user-authn`](/modules/user-authn/) module. You can configure:

- A list of trusted IP addresses or networks allowed to access the API.
- A list of user groups permitted to authenticate.
- The Ingress controller through which access will be provided.

To configure:

1. Enable API publishing as shown in the example above.
1. Configure access restrictions. In the [module configuration](/modules/user-authn/configuration.html), you can specify:
   - A list of IP addresses or networks allowed to access (`allowedSourceRanges`).
   - A list of user groups allowed to connect to the Kubernetes API (`allowedUserGroups`).
   - The Ingress controller to be used for publishing (`ingressClass`).
1. Use the kubeconfig web interface.  
   Users will be able to securely access the API using the kubeconfig generated via the web interface (`https://kubeconfig.<publicDomainTemplate>`).  
   This kubeconfig will include the OIDC token and the Ingress connection settings.

The following will be automatically configured when API publishing is enabled:

- Deckhouse will set the required arguments for the kube-apiserver.
- A CA certificate will be generated and added to the kubeconfig.
- Login via Dex with OIDC support will be configured.

## Access using Basic Authentication (LDAP)

In addition to OIDC, you can configure direct access to the API using Basic Authentication (username and password). In this case, credentials are verified via LDAP.

To configure:

1. Enable API publishing (`publishAPI` parameter).
2. Configure an LDAP provider in the `user-authn` module and enable the `enableBasicAuth: true` option.

> **Important:** Only one provider in the cluster can have `enableBasicAuth` enabled.

After this, users can configure their `kubeconfig` by specifying their LDAP username and password:

```yaml
apiVersion: v1
kind: Config
clusters:
- name: my-cluster
  cluster:
    server: https://api.example.com
    # Path to CA certificate or insecure-skip-tls-verify: true
    certificate-authority: /path/to/ca.crt
users:
- name: ldap-user
  user:
    username: janedoe@example.com
    password: userpassword
contexts:
- name: default
  context:
    cluster: my-cluster
    user: ldap-user
current-context: default
```
