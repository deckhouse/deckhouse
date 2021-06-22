---
title: "The user-authn module: configuration"
---

This module is **disabled** by default. To enable it, add the following lines to the `deckhouse` ConfigMap:

```yaml
data:
  userAuthnEnabled: "true"
```

## Parameters

* `publishAPI` — settings for exposing the API server using Ingress:
  * `enable` — setting it to `true` will create an Ingress resource in the d8-user-authn namespace in the cluster (it exposes the Kubernetes API);
    * Set to `false` by default;
  * `ingressClass` — the ingress class that will be used to expose the Kubernetes API via Ingress;
  * `whitelistSourceRanges` — an array of CIDRs that are allowed to connect to the API;
    * If this parameter is omitted, the API connection is not restricted by an IP address;
  * `https` — the HTTPS mode for the API server Ingress:
    * `mode` — the mode of issuing certificates for the Ingress resource. Possible values are `SelfSigned` and `Global`. In the `SelfSigned` mode, a self-signed certificate will be issued for the ingress resource. In the `Global` mode, the policies specified in the `global.modules.https.mode` global parameter will be applied. Thus, if the global parameter has the `CertManager` mode set (with `letsencrypt` as the clusterissuer), then the Lets Encrypt certificate will be issued for the Ingress resource;
      * Set to `SelfSigned` by default;
    * `global` — an additional parameter for the `Global` mode;
      * `kubeconfigGeneratorMasterCA` — if there is an external load balancer in front of the Ingress that terminates HTTPS traffic, then you need to specify the CA of the certificate used on the load balancer so that kubectl can reach the API server;
         * Also, you can set the external LB's certificate itself as a CA if you can't get the CA that signed it for some reason. Note that after the certificate is updated on the LB, all the previously generated kubeconfigs will stop working.
* `kubeconfigGenerator` — an array in which additional possible methods for accessing the API are specified. This option comes in handy if you prefer not to grant access to the cluster's API via Ingress but rather do it by other means (e.g., using a bastion host or over OpenVPN).
  * `id` — the name of the method for accessing the API server (no spaces, lowercase letters);
  * `masterURI` — an address of the API server;
    * If you plan to use a TCP proxy, then you must configure a certificate on the API server's side for the TCP proxy address. Suppose your API servers use three different addresses (`192.168.0.10`, `192.168.0.11`, and `192.168.0.12`) while the client uses a TCP load balancer (say, `192.168.0.15`). In this case, you have to re-generate the API server certificates:
      * edit `kubeadm-config`: `kubectl -n kube-system edit configmap kubeadm-config` and add `192.168.0.15` to `.apiServer.certSANs`;
      * save the resulting config: `kubeadm config view > kubeadmconf.yaml`;
      * delete old API server certificates: `mv /etc/kubernetes/pki/apiserver.* /tmp/`;
      * reissue new certificates: `kubeadm init phase certs apiserver --config=kubeadmconf.yaml`;
      * restart the API server's container: `docker ps -a | grep 'kube-apiserver' | grep -v pause| awk '{print $1}' | xargs docker restart`;
      * repeat this step for all master nodes;
  * `masterCA` — a CA for accessing the API;
    * If the parameter is not set, Kubernetes CA is used;
    * We recommend using a self-signed certificate (and specify it as masterCA) if an HTTP proxy (that terminates HTTPS traffic) is used for exposing;
  * `description` — description of the method for accessing the API server that is displayed to the user (in the list);
* `idTokenTTL` — the TTL of the id token (use s for seconds, m for minutes, h for hours);
  * By default, it is set to 10 minutes;
  * An example: `1h`
* `highAvailability` — manually manage the high availability mode. By default, the HA mode gets enabled/disabled automatically. Read [more](../../deckhouse-configure-global.html#parameters) about the HA mode for modules;
* `nodeSelector` — the same as in the pods' `spec.nodeSelector` parameter in Kubernetes;
    * If the parameter is omitted of `false`, it will be determined [automatically](../../#advanced-scheduling);
* `tolerations` — the same as in the pods' `spec.tolerations` parameter in Kubernetes;
    * If the parameter is omitted of `false`, it will be determined [automatically](../../#advanced-scheduling);
* `ingressClass` — the Ingress controller class used for dex and kubeconfig-generator;
  * An optional parameter; by default, the `modules.ingressClass` global value is used;
* `https` — selects the type of certificate to use for dex and kubeconfig-generator;
  * This parameter completely overrides the `global.modules.https` settings;
  * `mode` — the HTTPS usage mode:
    * `Disabled` — the module is automatically disabled;
    * `CertManager` — dex and kubeconfig-generator will run over HTTPS and get a certificate using clusterissuer as specified by the `certManager.clusterIssuerName` parameter;
    * `CustomCertificate` — dex and kubeconfig-generator will run over HTTPS using the certificate from the `d8-system` namespace;
    * `OnlyInURI` — dex and kubeconfig-generator will run over HTTP (thinking that there is an external HTTPS load balancer in front of them that terminates HTTPS). All the links in the `user-authn` will be generated using the HTTPS scheme.
  * `certManager`
    * `clusterIssuerName` — what ClusterIssuer to use for dex and kubeconfig-generator (currently, `letsencrypt`, `letsencrypt-staging`, `selfsigned` are available; also, you can define your own);
  * `customCertificate`
    * `secretName` — the name of the secret in the `d8-system`, namespace that will be used for dex & kubeconfig-generator (this secret must have the [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets)) format;
* `controlPlaneConfigurator` — parameters of the [control-plane-manager](../../modules/040-control-plane-manager/) module;
  * `enabled` — defines if the control-plane-manager module should be used to configure OIDC for the kube-apiserver;
    * It is set to `true` by default;
  * `dexCAMode` — how to determine the CA that will be used when configuring kube-apiserver;
    * Possible values:
      * `FromIngressSecret` — extract the CA of certificate from the secret that is used in the Ingress. This option comes in handy if you use self-signed certificates with Ingresses;
      * `Custom` — use the CA explicitly set via the `dexCustomCA` parameter (see below). This option comes in handy if you use an external HTTPS load balancer in front of Ingresses, and this load balancer relies on a self-signed certificate;
      * `DoNotNeed` — a CA is not required (e.g., when using a public LE or other TLS providers);
    * The default value is `DoNotNeed`;
  * `dexCustomCA` — the CA to use if `dexCAMode` = `Custom`.
    * Format — plain text (no base64).
    * An optional parameter.

The creation of the [`DexAuthenticator`](cr.html#dexauthenticator) Custom Resource leads to the automatic deployment of [oauth2-proxy](https://github.com/pusher/oauth2_proxy) to your application's namespace and connecting it to dex.

**Caution!** Since using OpenID Connect over HTTP poses a significant threat to security (the fact that Kubernetes API server doesn't support OICD over HTTP confirms that), this module can only be installed if HTTPS is enabled (to do this, set the `https.mode` parameter to the value other than `Disabled` either at the cluster level or in the module).

**Caution!** Note that when this module is enabled, authentication in all web interfaces will be switched from HTTP Basic Auth to dex (the latter, in turn, will use the external providers that you have defined). To configure kubectl, go to `https://kubeconfig.<modules.publicDomainTemplate>/`, log in to your external provider's account and copy the shell commands to your console.

**Caution!** Note that the API server requires [additional configuration](usage.html#configuring-kube-apiserver) to use authentication for dashboard and kubectl. The [control-plane-manager](../../modules/040-control-plane-manager/) module (enabled by default) automates this process.
