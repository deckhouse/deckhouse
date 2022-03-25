---
title: "The openvpn module: configuration"
---

This module is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  openvpnEnabled: "true"
```

## Parameters

* `inlet` — the way the connection is implemented;
    * The following inlet types are supported:
      * `ExternalIP` — when there are nodes with public IPs. It is used together with the `externalIP` parameter;
      * `LoadBalancer` — for all cloud providers and cloud-based placement strategies that support the provision of LoadBalancers;
      * `HostPort` — the port of the openvpn server will be available on the node where it is scheduled. The port can be configured in the `hostPort` parameter;
      * `Direct` — for non-standard cases. You need to create a service called `openvpn-external` in the `d8-openvpn` namespace. It will route traffic to the pod with the `app: openvpn` label to the port called `ovpn-tcp` (or just 1194). This service provides the externalIP, the IP address of the balancer or its host. If none of these are present, you need to specify the `externalHost` parameter;
* `loadBalancer` — a section of optional parameters of the `LoadBalancer` inlet:
    * `annotations` — annotations to assign to the service for flexible configuration of the load balancer;
        * **Note** that module does not take into account the specifics of setting annotations in different clouds. If annotations for the provision of the load balancer are only used when the service is being created, then you need to restart the module (disable/enable it) to update them;
    * `sourceRanges` — a list of CIDRs that are allowed to connect to the load balancer;
        * Format — an array of strings;
        * The cloud provider may not support this option or ignore it;
* `hostPort` — port to connect to the openvpn server, which will be available on the node where it is scheduled;
  * The parameter is available when selecting inlet `HostPort`;
  * The default value is `5416`;
* `externalIP` — the IP address of a cluster node to connect OpenVPN clients;
  * It is only required if the `ExternalIP` inlet is used;
* `externalPort` — the port to expose on the `externalIP` or load balancer;
  * The default port is `5416`;
* `tunnelNetwork` — a subnet used for tunneling;
  * The default subnet is `172.25.175.0/255.255.255.0`;
* `pushToClientRoutes` — a list of routes to send to clients upon their connection;
  * By default, this list is generated automatically using the local cluster network, service subnet, and pod subnet;
* `pushToClientDNS` — the IP address of the DNS server to send to clients upon connection;
  * By default, the IP address of the kube-system/kube-dns service is used;
* `pushToClientSearchDomains` — a list of search domains to send to clients upon connection;
  * The default value is `global.discovery.clusterDomain`;
* `auth` — options related to authentication or authorization in the application:
    * `externalAuthentication` — a set of parameters to enable external authentication (it is based on the Nginx Ingress [external-auth](https://kubernetes.github.io/ingress-nginx/examples/auth/external-auth/) mechanism that uses the Nginx [auth_request](http://nginx.org/en/docs/http/ngx_http_auth_request_module.html) module) (**the externalAuthentication parameters are set automatically if the user-authn module is enabled**);
        * `authURL` — the URL of the authentication service. If the user is authenticated, the service should return an HTTP 200 response code;
        * `authSignInURL` — the URL to redirect the user for authentication (if the authentication service returned a non-200 HTTP response code);
    * `password` — the password for http authorization of the `admin` user (it is generated automatically, but you can change it);
        * This parameter is used if the `externalAuthentication` parameter is not enabled;
    * `allowedUserGroups` — an array of user groups that can access the openvpn admin panel;
        * This parameter is used if the `user-authn` module is enabled or the `externalAuthentication` parameter is set;
        * **Caution!** Note that you must add those groups to the appropriate field in the DexProvider config if this module is used together with the user-authn one;
    * `whitelistSourceRanges` — the CIDR range for which authentication to access the openvpn is allowed;
* `externalHost` — an IP address or a domain clients use to connect to the OpenVPN server;
  * By default, data from an `openvpn-external` service are used;
* `ingressClass` — the class of the Ingress controller used for the openvpn admin panel;
    * By default, the `modules.ingressClass` global value is used;
* `https` — what certificate type to use with the openvpn admin panel;
    * This parameter completely overrides the `global.modules.https` settings;
    * `mode` — the HTTPS usage mode:
        * `Disabled` — in this mode, the openvpn admin panel works over http only;
        * `CertManager` — he openvpn admin panel will use HTTPS and get a certificate from the clusterissuer defined in the `certManager.clusterIssuerName` parameter;
        * `CustomCertificate` — the openvpn admin panel will use the certificate from the `d8-system` namespace for HTTPS;
        * `OnlyInURI` — the openvpn admin panel will work over HTTP (thinking that there is an external HTTPS load balancer in front of it that terminates HTTPS traffic). All the links in the `user-authn` will be generated using the HTTPS scheme;
    * `certManager`
      * `clusterIssuerName` — what ClusterIssuer to use for the openvpn admin panel (currently, `letsencrypt`, `letsencrypt-staging`, `selfsigned` are available; also, you can define your own);
        * By default, `letsencrypt` is used;
    * `customCertificate`
      * `secretName` - the name of the secret in the `d8-system` namespace to use with the openvpn admin panel (this secret must have the [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets) format);
        * It is set to `false` by default;
* `nodeSelector` — the same as in the pods' `spec.nodeSelector` parameter in Kubernetes;
    * If the parameter is omitted or `false`, it will be determined [automatically](../../#advanced-scheduling).
* `tolerations` — the same as in the pods' `spec.tolerations` parameter in Kubernetes;
    * If the parameter is omitted or `false`, it will be determined [automatically](../../#advanced-scheduling).
