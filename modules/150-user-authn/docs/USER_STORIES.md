---
title: "User Stories" 
---

1. Connect an authentication provider using config values 
2. Create static users using User CRD
3. Deploy DexAuthenticator CRD leads to the creation of oauth2 proxy Deployment with various parameters, Ingress, Service and Oauth2 client for accessing dex 
4. Specifying Atlassian Crowd provider with enableBasicAuth option set to true leads to the creation of Deployment with crowd-basic-auth-proxy
5. Enabling of publishAPI option leads to the creation of Ingress object for apiserver connection with desired ingress-shim annotation
6. Switching on Control Plane Configurator for the module should add special Configmap to the cluster and generate necessary values
7. Specifying KubeconfigGenerator in module settings adds parameters to KubeconfigGenerator Configmap
8. Deploy of DexClient CRD must register oauth2 client entry to Dex.
