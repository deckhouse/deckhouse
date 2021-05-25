---
title: "The deckhouse-web module: configuration"
---

## Parameters

* `ingressClass` — the class of the igress controller of the documentation web UI;
    * An optional parameter; by default, the `modules.ingressClass` global value is used;
* `auth` — parameters to authenticate and authorize access to the documentation web interface:
    * `externalAuthentication` — parameters to enable external authentication (the Nginx Ingress [external-auth](https://kubernetes.github.io/ingress-nginx/examples/auth/external-auth/) mechanism is used that is based on the Nginx [auth_request](http://nginx.org/en/docs/http/ngx_http_auth_request_module.html) module);
         * `authURL` — the URL of the authentication service. If the user is authenticated, the service should return an HTTP 200 response code;
         * `authSignInURL` — the URL to redirect the user for authentication (if the authentication service returned a non-200 HTTP response code);
    * `password` — the password for http authorization of the `admin` user (it is generated automatically, but you can change it);
         * This parameter is used if the `externalAuthentication` is not enabled;
    * `allowedUserGroups` — an array of groups whose users can browse the documentation;
         * This parameter is used if the `user-authn` module is enabled or the `externalAuthentication` parameter is set;
         * **Caution!** Note that you must add those groups to the appropriate field in the DexProvider config if this module is used together with the user-authn one;
* `https` — what certificate type to use with the documentation web UI;
    * This parameter completely overrides the `global.modules.https` settings;
    * `mode` — the HTTPS usage mode:
        * `Disabled` — in this mode, the documentation web UI can only be accessed over HTTP;
        * `CertManager` — the web UI is accessed over HTTPS using a certificate obtained from a clusterIssuer specified in the `certManager.clusterIssuerName` parameter;
        * `CustomCertificate` — the web UI is accessed over HTTPS using a certificate from the `d8-system` namespace;
        * `OnlyInURI` — the documentation web UI will work over HTTP (thinking that there is an external HTTPS load balancer in front of it that terminates HTTPS traffic). All the links in the `user-authn` will be generated using the HTTPS scheme.
    * `certManager`
      * `clusterIssuerName` — what ClusterIssuer to use for getting an SSL certificate (currently, `letsencrypt`, `letsencrypt-staging`, `selfsigned` are available; also, you can define your own);
        * By default, `letsencrypt` is used;
    * `customCertificate`
      * `secretName` — the name of the secret in the `d8-system` namespace to use with the documentation web UI (this secret must have the [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets) format);
        * It is set to `false` by default;
* `nodeSelector` — the same as in the pods' `spec.nodeSelector` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](../../#advanced-scheduling);
    * You can set it to `false` to avoid adding any nodeSelector;
* `tolerations` — the same as in the pods' `spec.tolerations` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](../../#advanced-scheduling);
    * You can set it to `false` to avoid adding any tolerations;

### An example

```yaml
deckhouseWeb: |
  nodeSelector:
    node-role/example: ""
  tolerations:
  — key: dedicated
    operator: Equal
    value: example
  externalAuthentication:
    authURL: "https://<applicationDomain>/auth"
    authSignInURL: "https://<applicationDomain>/sign-in"
    authResponseHeaders: "Authorization"
```
