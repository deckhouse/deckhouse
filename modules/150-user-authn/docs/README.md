---
title: "The user-authn module"
search: kube config generator
webIfaces:
- name: kubeconfig
  urlInfo: faq.html#how-can-i-generate-a-kubeconfig-and-access-kubernetes-api
---

The module sets up a unified authentication system integrated with Kubernetes and Web interfaces used in other modules (Grafana, Dashboard, etc.).

It consists of the following components:
- [dex](https://github.com/dexidp/dex) — is a federated OpenID Connect provider that acts as an identity service for static users and can be linked to one or more ID providers (e.g., SAML providers, GitHub, and Gitlab);
- `kubeconfig-generator` (in fact, [dex-k8s-authenticator](https://github.com/mintel/dex-k8s-authenticator)) — is a helper web application that (being authorized with dex) generates kubectl commands for creating and modifying a kubeconfig;
- `dex-authenticator` (in fact, [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy)) — is an application that gets NGINX Ingress (auth_request) requests and authenticates them with Dex.

Static users are managed using the [`User`](cr.html#user) custom resource. It contains all the user-related data, including the password.

The following external authentication protocols/providers are supported:
- GitHub
- GitLab
- BitBucket Cloud
- Crowd
- LDAP
- OIDC

You can use several external authentication providers simultaneously.

## Integration features

### Basic authentication for the Kubernetes API requests

[Basic authentication](https://en.wikipedia.org/wiki/Basic_access_authentication) for the Kubernetes API requests is currently only available for the Crowd provider (with the [`enableBasicAuth`](cr.html#dexprovider-v1-spec-crowd-enablebasicauth) parameter).

> You can also connect to the Kubernetes API [via other supported external providers](#web-interface-for-generating-ready-made-kubeconfig-files).

### Application integration

To enable authentication for any web application running in Kubernetes, create a [_DexAuthenticator_](cr.html#dexauthenticator) resource in the application's _Namespace_ and add several annotations to the _Ingress_ resource.
This will enable you to:
* limit the list of groups with access;
* limit the list of addresses for which authentication is allowed;
* integrate the application into a unified authentication system if the application supports OIDC. For that, Kubernetes will create a resource [_DexClient_](cr.html#dexclient) in the application _Namespace_. A secret with data for connecting to Dex via OIDC will also be created in that _Namespace_.

Following such an integration, you can: 
* limit the list of groups for which the connection is allowed; 
* define a list of clients with trusted OIDC tokens (`trustedPeers`).

### Web interface for generating ready-made kubeconfig files

The module allows you to automatically generate configuration for kubectl or other Kubernetes tools. 

The user will be presented with a set of commands to configure kubectl once authorized in the generator's web interface. These commands can be copied and pasted into the console to use kubectl.
The authentication mechanism for kubeconfig uses an OIDC token. The OIDC session can be extended automatically if the authentication provider used in Dex supports session extension. For this, the `refresh token` is specified in kubeconfig.

On top of that, you can configure multiple `kube-apiserver` addresses and CA certificates for each of them. This may come in handy, e. g., if access to the Kubernetes cluster is via VPN or direct connection.

## Exposing the Kubernetes API over Ingress

The kube-apiserver component (without advanced configuration) is only accessible in the internal cluster network. This module enables easy and secure access to Kubernetes API from outside the cluster. The API server is exposed on a dedicated domain (for more details, see the [section on service domains in the documentation](../../deckhouse-configure-global.html)).

When configuring, you can:
* list network addresses from which connection is allowed;
* list groups that are allowed to access the API server;
* specify Ingress-controller to authenticate on.

By default, a special CA certificate will be generated and the kubeconfig generator will be automatically configured.

## Extensions by Flant

The module uses a modified version of Dex to support:
* groups for static user accounts and Bitbucket Cloud provider (parameter [`bitbucketCloud`](cr.html#dexprovider-v1-spec-bitbucketcloud));
* passing the `group` parameter to clients;
* the `obsolete tokens` mechanism to avoid a race condition when an OIDC client renews a token.

## High availability mode

The module also supports the `highAvailability` mode. When this mode is enabled, all components responding to the `auth requests` are deployed with the redundancy required to operate continuously without failure. Thus, the user authentication sessions are kept alive even if any of the authenticator instances fail.
