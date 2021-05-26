---
title: "The ingress-nginx module: Custom Resources"
---

## IngressNginxController

The parameters shall be specified in the `spec` field.

### Mandatory parameters
* `ingressClass` — the name of the ingress class to use with the ingress nginx controller. Using this option, you can create several controllers to use with a single ingress class;
    * **Caution!** If you set it to "nginx", then ingress resources lacking the `kubernetes.io/ingress.class` annotation will also be handled;
* `inlet` — the way traffic flows from the outside world;
    * `LoadBalancer` — installs the ingress controller and provisions a service of the LoadBalancer type;
    * `LoadBalancerWithProxyProtocol` — installs the ingress controller and provisions a service of the LoadBalancer type. The ingress controller uses the proxy-protocol to get the actual IP address of the client;
    * `HostPort` — installs the ingress controller that is accessible via the node ports at the hostPort;
    * `HostPortWithProxyProtocol` — installs the ingress controller that is accessible via the node ports at the hostPort; the proxy-protocol is used to get the actual IP address of the client;
        * **Caution!** Make sure that requests to the ingress are sent from trusted sources when using this inlet. The `acceptRequestsFrom` parameter can help you with defining trusted sources;
    * `HostWithFailover` — installs two ingress controllers, the primary and the backup one. The primary controller runs in a hostNetwork. If the pods of the primary controller are not available, the traffic is routed to the backup one;
        * **Caution!** There can be only one controller with this inlet type on a host.
        * **Caution!** The following ports must be available on the node: 80, 81, 443, 444, 10354, 10355.

### Non-mandatory parameters
* `controllerVersion` — version of the ingress-nginx controller;
    * By default, the version in the module settings is used;
    * Available alternatives are `"0.25"`, `"0.26"`, `"0.33"`;
* `nodeSelector` — the same as in the pods' `spec.nodeSelector` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](../../#advanced-scheduling).
    * You can set it to `false` to avoid adding any nodeSelector;
* `tolerations` — the same as in the pods' `spec.tolerations` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](../../#advanced-scheduling).
    * You can set it to `false` to avoid adding any tolerations;

* `loadBalancer` — a section of parameters of the `LoadBalancer` inlet:
    * `annotations` — annotations to assign to the service for flexible configuration of the load balancer;
        * **Caution!** The module does not take into account the specifics of setting annotations in different clouds. Note that you will need to recreate `IngressNginxController` (or create a new controller and then delete the old one) if annotations to provision a load balancer are only used when creating the service;
    * `sourceRanges` — a list of CIDRs that are allowed to connect to the load balancer;
        * The cloud provider may not support this option or ignore it;
    * `behindL7Proxy` — enables parsing and forwarding X-Forwarded-* headers;
        * **Caution!** Make sure that requests to the ingress are sent from trusted sources when using this option;
    * `realIPHeader` — the header with the actial IP address of the client;
        * By default, `X-Forwarded-For` is used;
        * This option works only if `behindL7Proxy` is enabled;

* `loadBalancerWithProxyProtocol` — a section of parameters of the `LoadBalancerWithProxyProtocol` inlet:
    * `annotations` — annotations to assign to the service for flexible configuration of the load balancer;
        * **Caution!** The module does not take into account the specifics of setting annotations in different clouds. Note that you will need to recreate `IngressNginxController` (or create a new controller and then delete the old one) if annotations to provision a load balancer are only used when creating the service;
    * `sourceRanges` — a list of CIDRs that are allowed to connect to the load balancer;
        * The cloud provider may not support this option or ignore it;

* `hostPort` — a section of parameters of the `HostPort` inlet:
  * `httpPort` — a port to connect over HTTP (insecure);
        * If the parameter is not set, the connection over HTTP cannot be established;
        * This parameter is mandatory if `httpsPort` is not set;
    * `httpsPort` — a port for secure connection over HTTPS;
        * If the parameter is not set, the connection over HTTPS cannot be established;
        * This parameter is mandatory if `httpPort` is not set;
    * `behindL7Proxy` — enables parsing and forwarding X-Forwarded-* headers;
        * **Caution!** Make sure that requests to the ingress are sent from trusted sources when using this option. The `acceptRequestsFrom` parameter can help you with defining trusted sources;
    * `realIPHeader` — the header with the actual IP address of the client;
        * By default, `X-Forwarded-For` is used;
        * This option works only if `behindL7Proxy` is enabled;

* `hostPortWithProxyProtocol` — a section of parameters of the `HostPortWithProxyProtocol` inlet:
    * `httpPort` — a port to connect over HTTP (insecure);
        * If the parameter is not set, the connection over HTTP cannot be established;
        * This parameter is mandatory if `httpsPort` is not set;
    * `httpsPort` — a port for secure connection over HTTPS;
        * If the parameter is not set, the connection over HTTPS cannot be established;
        * This parameter is mandatory if `httpPort` is not set;

* `acceptRequestsFrom` — a list of CIDRs that are allowed to connect to the controller. Regardless of the inlet type, the source IP address gets always verified (the `original_address` field in logs) (the address that the connection was established from) and not the "address of the client" that can be passed in some inlets via headers or using the proxy protocol;
    * This parameter is implemented using the [map module](http://nginx.org/en/docs/http/ngx_http_map_module.html). If the source address is not in the list of allowed addresses, nginx closes the connection immediately using HTTP 444;
    * By default, the connection to the controller can be made from any address;
* `resourcesRequests` — max amounts of CPU and memory resources that the pod can request when selecting a node (if the VPA is disabled, then these values become the default ones);
    * `mode` — the mode for managing resource requests:
        * Possible options: `VPA`, `Static`;
        * By default, the `VPA` mode is enabled;
    * `vpa` — parameters of the vpa mode:
        * `mode` — the VPA usage mode:
            * Possible options: `Initial`, `Auto`;
            * By default, the `Initial` mode is enabled;
        * `cpu` — CPU-related parameters:
            * `max` — the max cpu value that the VPA can request;
                * Set to `50m` by default;
            * `min` — the min cpu value that the VPA can request;
                * Set to `10m` by default;
        * `memory` — the amount of memory requested:
            * `max` — the max memory amount that the VPA can request;
                * Set to `200Mi` by default;
            * `min` — the min memory amount that the VPA can request;
                * Set to `50Mi` by default;
    * `static` — parameters of the static mode:
        * `cpu` — the cpu cores requested;
            * Set to `50m` by default;
        * `memory` —  the amount of memory requested;
            * Set to `200Mi` by default;
* `hsts` — bool; determines whether hsts is enabled (read more [here](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Strict-Transport-Security));
    * It is set to `false` by default;
* `hstsOptions` — parameters if HTTP Strict Transport Security:
    * `maxAge` — specifies for how long future requests to the site must use HTTPS;
        * By default, `31536000` seconds (one year) are set;
    * `preload` — specifies whether to add the site to the hsts preload list. This list contains domain names that support HSTS by default and is used by all modern web browsers;
        * It is set to `false` by default;
    * `includeSubDomains` — if hsts settings should be applied to all sub-domains of the site;
        * It is set to `false` by default;
* `legacySSL` — bool, determines whether legacy TLS versions are enabled. Also, this options enables legacy cipher suites to support legacy libraries and software: [OWASP Cipher String 'C' ](https://cheatsheetseries.owasp.org/cheatsheets/TLS_Cipher_String_Cheat_Sheet.html). Learn more [here](https://github.com/deckhouse/deckhouse/blob/master/modules/402-ingress-nginx/templates/controller/configmap.yaml);
    * By default, only TLSv1.2 and the newest cipher suites are enabled;
* `disableHTTP2` — bool, enables/disables HTTP/2;
    * By default, HTTP/2 is enabled (`false`);
* `geoIP2` — parameters to enable GeoIP2 databases (version `"0.33"` or later of the controller is required):
    * `maxmindLicenseKey` — a license key to download the GeoIP2 database. If the key is set, the module downloads the GeoIP2 database every time the controller is started. Click [here](https://blog.maxmind.com/2019/12/18/significant-changes-to-accessing-and-using-geolite2-databases/) to learn more about obtaining a license key;
    * `maxmindEditionIDs` — a list of database editions to download at startup. [This article](https://support.maxmind.com/geolite-faq/general/what-is-the-difference-between-geoip2-and-geolite2/) sheds light on the differences between GeoIP2-City and GeoLite2-City databases;
        * By default, `["GeoLite2-City", " GeoLite2-ASN"]` databases are enabled;
        * Possible options:
            * GeoIP2-Anonymous-IP
            * GeoIP2-Country
            * GeoIP2-City
            * GeoIP2-Connection-Type
            * GeoIP2-Domain
            * GeoIP2-ISP
            * GeoIP2-ASN
            * GeoLite2-ASN
            * GeoLite2-Country
            * GeoLite2-City
* `underscoresInHeaders` — bool, determines whether underscores are allowed in headers. Learn more [here](http://nginx.org/en/docs/http/ngx_http_core_module.html#underscores_in_headers). [This tutorial](https://www.nginx.com/resources/wiki/start/topics/tutorials/config_pitfalls/#missing-disappearing-http-headers) sheds light on why you should not enable it without careful consideration;
    * It is set to `false` by default;
* `customErrors` — the section with parameters of custom HTTP errors (all parameters in this section are mandatory if it is defined; changing any parameter **leads to the restart of all ingress-nginx controllers**);
    * `serviceName` — the name of the service to use as the custom default backend;
    * `namespace` — the name of the namespace where the custom default backend service is running;
    * `codes` — a list of response codes (an array) for which the request will be redirected to the custom default backend;
* `config` — the section with the ingress controller parameters; you can specify [any supported parameter](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/) in it in the `key: value (string)` format;
    * **Caution!** An erroneous option may lead to the failure of the ingress controller;
    * **Caution!** The usage of this parameter is not recommended; the backward compatibility or operability of the ingress controller that uses this option is not guaranteed;
* `additionalHeaders` — additional headers to add to each request (in the `key: value(string)` format);
* `enableIstioSidecar` — attach annotations to the controller pods to automatically inject Istio sidecar containers. With this flag set, the controller can only serve services that Istio controls.
