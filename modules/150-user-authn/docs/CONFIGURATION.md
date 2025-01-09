---
title: "The user-authn module: configuration"
---

<!-- SCHEMA -->

The creation of the [`DexAuthenticator`](cr.html#dexauthenticator) Custom Resource leads to the automatic deployment of [oauth2-proxy](https://github.com/oauth2-proxy/oauth2-proxy) to your application's namespace and connecting it to Dex.

**Caution!** Since using OpenID Connect over HTTP poses a significant threat to security (the fact that Kubernetes API server doesn't support OICD over HTTP confirms that), this module can only be installed if HTTPS is enabled (to do this, set the `https.mode` parameter to the value other than `Disabled` either at the cluster level or in the module).

**Caution!** When this module is enabled, authentication in all web interfaces will be switched from HTTP Basic Auth to Dex (the latter, in turn, will use the external providers that you have defined). To configure kubectl, go to `https://kubeconfig.<modules.publicDomainTemplate>/`, log in to your external provider's account and copy the shell commands to your console.

**Caution!** The API server requires [additional configuration](faq.html#configuring-kube-apiserver) to use authentication for dashboard and kubectl. The [control-plane-manager](../../modules/control-plane-manager/) module (enabled by default) automates this process.
