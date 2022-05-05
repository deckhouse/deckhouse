---
title: "The dashboard module: configuration"
---

The module does not have any mandatory settings.

## Parameters
* `ingressClass` —  the class of the Ingress controller to use for the dashboard;
    * An optional parameter; by default, the `modules.ingressClass` global value is used;
* `auth` — options related to authentication or authorization in the application:
    * `externalAuthentication` — parameters to enable external authentication (the Nginx Ingress [external-auth](https://kubernetes.github.io/ingress-nginx/examples/auth/external-auth/) mechanism is used that is based on the Nginx's [auth_request](http://nginx.org/en/docs/http/ngx_http_auth_request_module.html) module);
         * `authURL` — the URL of the authentication service. The service should return an HTTP 200 response code if the user authentication is successful;
         * `authSignInURL` — the URL to redirect the user for authentication (if the authentication service returned a non-200 HTTP response code);
         * `useBearerTokens` — the dashboard must use the user ID to work with the Kubernetes API (the authentication service must return the Authorization HTTP header that contains the bearer-token – the dashboard will use this token to make requests to the Kubernetes API server).
             * Set to `false` by default;
             * Caution! For security reasons, this mode only works if `https.mode` (global or for a module) is not set to `Disabled`;
    * `password` — the password for HTTP authorization of the `admin` user (it is generated automatically, but you can change it);
         * This parameter is used if the `externalAuthentication` parameter is not enabled;
    * `whitelistSourceRanges` — the CIDR range for which authentication to access the dashboard is allowed;
    * `allowScale` — activating the ability to scale Deployment and StatefulSet from the web interface;
         * This parameter is used if the `externalAuthentication` parameter is not enabled;
* `https` — what certificate type to use with the dashboard;
    * This parameter completely overrides the `global.modules.https` settings;
    * `mode` — the HTTPS usage mode:
        * `Disabled` — in this mode, the dashboard works over http only;
        * `CertManager` — the dashboard will use HTTPS and get a certificate from the clusterissuer defined in the `certManager.clusterIssuerName` parameter;
        * `CustomCertificate` — the dashboard will use the certificate from the `d8-system` namespace for HTTPS;
        * `OnlyInURI` — the dashboard will work over HTTP (thinking that there is an external HTTPS load balancer in front of it that terminates HTTPS traffic). All the links in the `user-authn` will be generated using the HTTPS scheme;
    * `certManager`
      * `clusterIssuerName` — what ClusterIssuer to use for the dashboard (currently, `letsencrypt`, `letsencrypt-staging`, `selfsigned` are available; also, you can define your own);
        * By default, `letsencrypt` is used;
    * `customCertificate`
      * `secretName` - the name of the secret in the `d8-system` namespace to use with the dashboard (this secret must have the [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets) format);
        * It is set to `false` by default;
* `nodeSelector` — the same as in the Pod's `spec.nodeSelector` parameter in Kubernetes;
    * If the parameter is omitted or `false`, it will be determined [automatically](../../#advanced-scheduling).
* `tolerations` — the same as in the Pod's `spec.tolerations` parameter in Kubernetes;
    * If the parameter is omitted or `false`, it will be determined [automatically](../../#advanced-scheduling).
* `accessLevel` — the level of access to the dashboard if the `user-authn` module is switched off and no `externalAuthentication` is defined. You can view the list of supported values in the [user-authz](../../modules/140-user-authz/) documentation;
    * By default, the `User` role is set;
    * In case of using the `user-authn` module or another `externalAuthentication`, you should configure access right with the `user-authz` modules.
