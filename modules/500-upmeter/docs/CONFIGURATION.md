---
title: "The upmeter module: confguration"
---

This module is **enabled** by default.

## Parameters:
* `disabledProbes` – a string array containing group names or specific probes from a group. You can view the names in the web UI;
  * An example:

		disabledProbes:
		- "synthetic/api" # disable a specific probe
		- "synthetic/"    # disable a group of probes
		- control-plane   # / can be omitted
* `statusPageAuthDisabled` – disables authorization for the status domain;
  * Set to `false` by default;
* `storageClass` — the name of the StorageClass to use;
    * If omitted, the StorageClass of the existing PVC is used. If there is no PVC yet, either `global.StorageClass` or `global.discovery.defaultStorageClass` is used, and if those are undefined, the emptyDir volume is used to store the data;
    * **CAUTION!** Setting this value to one that differs from the current one (in the existing PVC) will result in disk reprovisioning and data loss;
    * Setting it to `false` forces the use of an emptyDir volume;
* `smokeMiniDisabled` – disables smokeMini;
  * Set to `false` by default;
* `smokeMini`
	* `storageClass` — storageClass to use when checking the health of disks;
		* If omitted, the StorageClass of the existing PVC is used. If there is no PVC yet, either `global.StorageClass` or `global.discovery.defaultStorageClass` is used, and if those are undefined, the emptyDir volume is used to store the data;
		* Setting it to `false` forces the use of an emptyDir volume;
	* `ingressClass` —  the class of the Ingress controller used for the smoke-mini;
		* An optional parameter; by default, the `modules.ingressClass` global value is used;
	* `https` — what certificate type to use with the smoke-mini;
		* This parameter completely overrides the `global.modules.https` settings;
		* `mode` — the HTTPS usage mode:
			* `Disabled` — in this mode, the smoke-mini works over http only;
			* `CertManager` — the smoke-mini will use HTTPS and get a certificate from the clusterissuer defined in the `certManager.clusterIssuerName` parameter;
			* `CustomCertificate` — the smoke-mini will use the certificate from the `d8-system` namespace for HTTPS;
			* `OnlyInURI` — the smoke-mini will use HTTP (expecting that an HTTPS load balancer runs in front of them and terminates HTTPS traffic);
		* `certManager`
			* `clusterIssuerName` — what ClusterIssuer to use for the smoke-mini (currently, `letsencrypt`, `letsencrypt-staging`, `selfsigned` are available; also, you can define your own);
				* By default, `letsencrypt` is used;
		* `customCertificate`
			* `secretName` — the name of the secret in the `d8-system` namespace to use with the smoke-mini (this secret must have the [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets) format);
				* It is set to `false` by default;
* `auth` — parameters to authenticate and authorize access to the documentation web interface:
    * `status`/`webui` — the component for which authentication parameters are set:
		* `externalAuthentication` — parameters to enable external authentication (the Nginx Ingress [external-auth](https://kubernetes.github.io/ingress-nginx/examples/auth/external-auth/) mechanism is used that is based on the Nginx [auth_request](http://nginx.org/en/docs/http/ngx_http_auth_request_module.html) module);
			* `authURL` — the URL of the authentication service. If the user is authenticated, the service should return an HTTP 200 response code;
			* `authSignInURL` — the URL to redirect the user for authentication (if the authentication service returned a non-200 HTTP response code);
		* `password` — the password for http authorization of the `admin` user (it is generated automatically, but you can change it);
			* This parameter is used if the `externalAuthentication` is not enabled;
		* `allowedUserGroups` —  an array of groups whose users are allowed to view the application interface;
			* This parameter is used if the `user-authn` module is enabled or the `externalAuthentication` parameter is set;
			* **Caution!** Note that you must add those groups to the appropriate field in the DexProvider config if this module is used together with the user-authn one;
