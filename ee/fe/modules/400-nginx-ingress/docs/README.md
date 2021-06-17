---
title: "The nginx-ingress module"
---

This module installs **one or more** [nginx-ingress controllers](https://github.com/kubernetes/ingress-nginx/) while considering all the specificities of integration with various types of Kubernetes clusters.

Additional information
----------------------
* A video guide ([part 1](https://www.youtube.com/watch?v=BS9QrmH6keI), [part 2](https://www.youtube.com/watch?v=_ZG8umyd0B4)) explaining module features and settings;
* A [graph guide](https://www.youtube.com/watch?v=IQac_TgiSao) and how to use them.

Configuration
------------

### What do I need to configure?

**Caution!** In most cases you **do not need to configure anything**! The best config is an empty config.

### Parameters

The module supports multiple controllers — **one** primary controller and **any number** of additional ones — and has the following parameters:
* `inlet` — the way traffic flows from the outside world;
    * This parameter is determined automatically based on the cluster type (GCE & ACS — LoadBalancer, AWS — AWSClassicLoadBalancer, Manual — Direct; learn more [here](https://github.com/deckhouse/deckhouse/blob/master/modules/400-nginx-ingress/templates/_helpers.tpl#L22-30))!
    * The following inlets are supported:
        * `LoadBalancer` (set automatically for `GCE` & `ACS`) — provisions the LoadBalancer automatically;
        * `AWSClassicLoadBalancer` (set automatically for`AWS`) — зprovisions the LoadBalancer and enables the proxy protocol; this inlet is the default one for AWS;
        * `Direct` (set automatically for  `Manual`) — pods are running in the host network, nginx listens on ports 80 & 443; also, a direct-fallback scheme is implemented for this mode;
        * `NodePort` — the NodePort service; this mode is suitable for situations where you need to configure a third-party load balancer (such as AWS Application Load Balancer, Qrator or CloudFlare). The acceptable range of ports is 30000-32767 (it is set using the `kube-apiserver --service-node-port-range` parameter);
    * This [file](https://github.com/deckhouse/deckhouse/blob/master/modules/400-nginx-ingress/templates/controller.yaml) clearly demonstrates the differences between the four types of inlets;
* `nodePortHTTP` — this parameter allows you to specify a specific `nodePort` to expose port 80 for `NodePort` inlets (by default, kube-controller-manager picks a random free port number);
* `nodePortHTTPS` —  this parameter allows you to specify a specific `nodePort` to expose port 443 for `NodePort` inlets (the default behavior is similar to that of `nodePortHTTP`);
* `config.hsts` — bool, determines if HSTS is enabled;
    * It is set to `false` by default;
* `config.legacySSL` — bool, determines whether legacy TLS versions are enabled. Also, this options enables legacy cipher suites to support legacy libraries and software: [OWASP Cipher String 'C' ](https://cheatsheetseries.owasp.org/cheatsheets/TLS_Cipher_String_Cheat_Sheet.html). Learn more [here](https://github.com/deckhouse/deckhouse/blob/master/modules/400-nginx-ingress/templates/_template.config.tpl);
    * By default, only TLSv1.2 and the newest cipher suites are enabled;
* `config.disableHTTP2` — bool, determines if HTTP/2 is disabled;
    * By default, HTTP/2 is enabled (`false`);
* `config.ComputeFullForwardedFor` - bool, determines whether the X-Forwarded-For header must include proxy addresses (or must be replaced). It is only required when there is a load balancer not under our control at the front;
    * By default, `compute-full-forwarded-for` is disabled (`false`);
* `config.underscoresInHeaders` — bool, determines whether underscores are allowed in headers. Learn more [here](http://nginx.org/en/docs/http/ngx_http_core_module.html#underscores_in_headers). This [tutorial](https://www.nginx.com/resources/wiki/start/topics/tutorials/config_pitfalls/#missing-disappearing-http-headers) sheds light on why you should not enable it without careful consideration;
    * It is set to `false` by default;
* `config.setRealIPFrom` — a list of CIDRs for which the address specified in the `X-Forwarded-For` header can be used as a client address;
    * Note that the list must be in **YAML format** and not a string of comma-separated values!
    * **Caution!** Since nginx ingress (as well as nginx itself) does not support getting the client address from `X-Forwarded-For`, when used together with the proxy protocol, the `config.setRealIPFrom` parameter cannot be used with `Direct` & `AWSClassicLoadBalancer` inlets;
* (only for additional controllers) `name` (mandatory) — the name of the controller;
    * The name is added (as a suffix) to the namespace {% raw %}`kube-nginx-ingress-{{ $name }}` and to the nginx class name `nginx-{{ $name }}` (this class is later included in the `kubernetes.io/ingress.class` annotation attached to ingress resources);{% endraw %}
* `customErrorsServiceName` — the name of the service to use as the custom default backend;
    * **Caution!** This parameter is mandatory if any other `customErrors` parameter is set;
    * **Caution!** Adding, deleting, or editing this parameter causes nginx instances to restart;
* `customErrorsNamespace` — the name of the namespace where the custom default backend service is running;
    * **Caution!** This parameter is mandatory if any other `customErrors` parameter is set;
    * **Caution!** Adding, deleting, or editing this parameter causes nginx instances to restart;
* `customErrorsCodes` - a list of response codes (an array) for which the request will be redirected to the custom default backend;
    * Note that the list must be in **YAML format** and not a string of comma-separated values!
    * **Caution!** This parameter is mandatory if any other `customErrors` parameter is set;
    * **Caution!** Adding, deleting, or editing this parameter causes nginx instances to restart;
* `enableIstioSidecar` — add sidecars to controller's pods to control traffic using Istio. After setting this parameter, the `sidecar.istio.io/inject: "true"` & `traffic.sidecar.istio.io/includeOutboundIPRanges: "<Service CIDR>"` annotations will be attached to the pods, while the Istio's `sidecar-injector` will inject sidecar containers into pods;
    * **Caution!** Since in the Direct mode pods are using the host network, the Service CIDR-based method of identifying traffic for Istio can be potentially dangerous for other services sharing the ClusterIP service. The Istio support is disabled for such inlets;
* `nodeSelector` — the same as in the pods' `spec.nodeSelector` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](/overview.html#advanced-scheduling);
    * You can set it to `false` to avoid adding any nodeSelector;
* `tolerations` — the same as in the pods' `spec.tolerations` parameter in Kubernetes;
    * If the parameter is omitted, it will be set [automatically](/overview.html#advanced-scheduling);
    * You can set it to `false` to avoid adding any tolerations;

### An example of a configuration file
{% raw %}

```yaml
nginxIngress: |
  config:
    hsts: true
  nodeSelector: false
  tolerations:
  - key: dedicated
    operator: Equal
    value: example
  additionalControllers:
  - name: direct
    inlet: Direct
    config:
      hsts: true
    customErrorsServiceName: "error-backend"
    customErrorsNamespace: "default"
    customErrorsCodes:
    - 404
    - 502
    nodeSelector:
      node-role/example: ""
    tolerations:
    - key: dedicated
      operator: Equal
      value: example
  - name: someproject
    inlet: NodePort
    nodeSelector: false
    tolerations: false
  - name: foo
```
{% endraw %}

### Aspects of using additional controllers

* You must specify a `name` for each additional controller. At the same time, it get its own copy of all resources in an individual `kube-nginx-ingress-<name>` namespace;
* The additional controller instances use a dedicated class (you have to specify it via the `kubernetes.io/ingress.class: "nginx-<name>"` annotation for the ingress resources.

### Aspects of using canary annotations

The use of canary annotations in ingress-nginx has its own *specifics*. Here is a list of known nuances:

1. You can only use one Ingress with canary annotations attached for each non-canary Ingress;
1. **Only** canary Ingresses with a single path behave [correctly](https://github.com/kubernetes/ingress-nginx/pull/4198);
1. Sticky sessions fail to preserve differentiation between canary/non-canary (each subsequent request may end up in the canary version as well as in the original one).

### Custom error pages

To use it, you will need an application that can return pages for the specified error codes. The HTML file must include all assets (styles, graphics, fonts). The service must accept connections on port 80. This service must return the 500 code for 500 errors, the 502 code for 502 errors, etc., if there are no apparent reasons to return other response codes.

Usage
---------------------

### Bare Metal + Qrator

Case study:
* Non-production environments (test, stage, etc.) and infrastructure components (prometheus, dashboard, etc.) connect to the internet directly.
* All production resources use the Qrator filtering.

Implementation:
* Leave the main controller unchanged.
* Set the `NodePort` inlet for the additional controller.
* Set the specific nodePort ports for HTTP and HTTPS (`nodePortHTTP` and `nodePortHTTPS`) in the additional controller (optional).
* Attach the `kubernetes.io/ingress.class: "nginx-qrator"` annotation to the production's ingress resources.
* The following command allows you to find out what ports are selected by controller-manager (if you have chosen not to set `nodePortHTTP`, `nodePortHTTPS` ports yourself): `kubectl -n kube-nginx-ingress-qraror get svc nginx -o yaml`.

```
nginxIngress: |
  additionalControllers:
  - name: qrator
    inlet: NodePort
    nodePortHTTP: 30080
    nodePortHTTPS: 30443
    config:
      setRealIPFrom:
      - 87.245.197.192
      - 87.245.197.193
      - 87.245.197.194
      - 87.245.197.195
      - 87.245.197.196
      - 83.234.15.112
      - 83.234.15.113
      - 83.234.15.114
      - 83.234.15.115
      - 83.234.15.116
      - 66.110.32.128
      - 66.110.32.129
      - 66.110.32.130
      - 66.110.32.131
      - 130.117.190.16
      - 130.117.190.17
      - 130.117.190.18
      - 130.117.190.19
      - 185.94.108.0/24
```

### AWS + CloudFlare

Case study:
* Most of the production resources, all non-production resources (test, stage, etc) and infrastructure components (prometheus, dashboard, etc) use the regular AWSClassicLoadBalancer.
* At the same time, some production resources must connect via CloudFront while the `setRealIPFrom` parameter is not supported when using `AWSClassicLoadBalancer` (due to incompatibility with the proxy protocol).

Implementation:
* Leave the main controller unchanged.
* Set the `NodePort` inlet for the additional controller.
* Configure CloudFlare to forward traffic to the service address: `kubectl -n kube-nginx-ingress-cf get svc nginx -o yaml`

```
nginxIngress: |
  additionalControllers:
  - name: cf
    inlet: LoadBalancer
    config:
      setRealIPFrom:
      - 103.21.244.0/22
      - 103.22.200.0/22
      - 103.31.4.0/22
      - 104.16.0.0/12
      - 108.162.192.0/18
      - 131.0.72.0/22
      - 141.101.64.0/18
      - 162.158.0.0/15
      - 172.64.0.0/13
      - 173.245.48.0/20
      - 188.114.96.0/20
      - 190.93.240.0/20
      - 197.234.240.0/22
      - 198.41.128.0/17
```

### AWS + AWS Application Load Balancer

Case study:
* Suppose the customer has Amazon-issued certificates already.
* The customer would like to avoid creating multiple controllers and LoadBalancers in Amazon to minimize costs.

Implementation:
* In this case, we will use `AWS Application Load Balancer` as the main and only entry point.
* To do this, reconfigure the main controller to use the `NodePort` inlet.
* Configure AWS's `Application Load Balancer` to route traffic to "ephemeral" service ports of the `NodePort` type: `kubectl -n kube-nginx-ingress get svc nginx -o yaml`.

```
nginxIngress: |
  inlet: NodePort
  config:
    setRealIPFrom:
    - 0.0.0.0/0
```

### AWS + AWS HTTP Classic Load Balancer

Case study:
* All cluster resources use the regular `AWS Classic Load Balancer`. However, there is a need to issue an Amazon certificate and you cannot use it with the existing `AWS Classic Load Balancer`.


Implementation:
* Leave the main controller unchanged.
* Set the `NodePort` inlet for the additional controller.
* Create (manually or using the infra project on gitlab) as many services as necessary (with special annotations for attaching certificates).

```
apiVersion: v1
kind: Service
metadata:
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-backend-protocol: http
    service.beta.kubernetes.io/aws-load-balancer-ssl-cert: arn:aws:acm:eu-central-1:206112445282:certificate/23341234d-7813-45e8-b249-123421351251234
    service.beta.kubernetes.io/aws-load-balancer-ssl-ports: "443"
  name: nginx-site-1
  namespace: kube-nginx-ingress-aws-http
spec:
  externalTrafficPolicy: Local
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: 80
  - name: https
    port: 443
    protocol: TCP
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```


```
nginxIngress: |
  additionalControllers:
  - name: aws-http
    inlet: NodePort
    config:
      setRealIPFrom:
      - 0.0.0.0/0
```

### Bare Metal + several applications that should not be affiliated

Case study:
* Suppose there is one main application supplemented by two additional ones. However, nobody can know that they belong to the same owners (hosted together).

Implementation:
* Dedicate the main controller to use with a subset of machines (attach the `node-role.deckhouse.io/frontend` label & taint to them).
* Create two additional controllers to use with two other subsets of machines (attach the `node-role/frontend-foo` & `node-role/frontend-bar` labels and taints to the respective subset).

```
nginxIngress: |
  additionalControllers:
  - name: foo
    nodeSelector:
      node-role/frontend-foo: ""
    tolerations:
    - key: node-role/frontend-foo
      operator: Exists
  - name: bar
    nodeSelector:
      node-role/frontend-bar: ""
    tolerations:
    - key: node-role/frontend-bar
      operator: Exists
```

Statistics
----------

### Basic principles of collecting statistics

1. Our [nginx module](https://github.com/deckhouse/deckhouse/blob/master/modules/400-nginx-ingress/images/controller/rootfs/etc/nginx/template/nginx.tmpl#L750) is called for each request (at the `log_by_lua` stage). It calculates the necessary data and sends them via UDP to `statsd`.
2. Instead of the regular `statsd`, a sidecar container with [statsd_exporter](https://github.com/prometheus/statsd_exporter) is running in the ingress-controller pod. It collects data in the `statsd` format, parses, aggregates them (according to the [specified rules](https://github.com/deckhouse/deckhouse/blob/master/modules/400-nginx-ingress/images/statsd-exporter/rootfs/etc/statsd_mapping.conf)), and exports them in the Prometheus format.
3. Every 30 seconds, Prometheus scrapes both the ingress-controller (since it exports some of the required metrics) and statsd_exporter. Then these data are used for stats.

### What information does Prometheus collect, and in what form?

* All the collected metrics have service labels that allow you to identify the controller's instance: `controller`, `app`, `instance`, and `endpoint` (displayed in `/prometheus/targets`).
* All metrics (except for geo) exported by statsd_exporter have three levels of detail:
    * `ingress_nginx_overall_*` — the "bird's-eye view"; all the metrics have the `namespace`, `vhost`, and `content_kind` labels attached;
    * `ingress_nginx_detail_*` — the `ingress`, `service`, `service_port`, and `location` labels are added to those listed above;
    * `ingress_nginx_detail_backend_*` — some detailed data; they are collected on a per-backend basis. The `pod_ip` label is added to those listed for the `detail` level;
* The following metrics are collected for the overall and detail levels:
    * `..._requests_total` — the total number of requests (additional labels: `scheme`, `method`);
    * `..._responses_total` — the total number of responses (additional labels: `status`);
    * `..._request_seconds_{sum,count,bucket}` — a histogram of the response time;
    * `..._bytes_received_{sum,count,bucket}` — a histogram of the request size;
    * `..._bytes_sent_{sum,count,bucket}` — a histogram of the response size;
    * `..._upstream_response_seconds_{sum,count,bucket}` — a histogram of the upstream response time (the sum of the response times of all upstreams is used several of them are present);
    * `..._lowres_upstream_response_seconds_{sum,count,bucket}` — the same as above but less detailed (it is suitable for visualization, but not at all for calculating quantiles);
    * `..._upstream_retries_{count,sum}` — the number of requests for which retries were sent to backends, and the number of retries.
* The following metrics are collected for the overall level:
    * `..._geohash_total` — the total number of requests with a specific geohash (additional labes: `geohash`, `place`).
* The following metrics are collected for the detail_backend level:
    * `..._lowres_upstream_response_seconds` — same as similar metrics for overall and detail levels;
    * `..._responses_total` — the total number of responses (additional label: `status_class` instead of `status`);
    *  `..._upstream_bytes_received_sum` — the sum of the backend's response sizes.

Additional information
-------------------------

### Key differences in the operation of load balancers in different Cloud environments

* When creating a Service with `spec.type=LoadBalancer`, Kubernetes creates a `NodePort` service. Also, it configures the load balancer in the cloud so that it routes traffic to specific `spec.ports[*].nodePort` (these ports are chosen randomly from the range `30000-32767`) of all Kubernetes nodes.
* In GCE and Azure, the load balancer routes traffic to the nodes while saving the client's source address. Setting `spec.externalTrafficPolicy=Local` when creating a service will lead to Kubernetes not forwarding traffic coming to the node to all nodes with endpoints. Instead, K8s will route it to the local endpoints on this node (if there are none, the connection will not be established. You can learn more about it [here](https://kubernetes.io/docs/tutorials/services/source-ip/) and [here](https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/#preserving-the-client-source-ip) (recommended)).
* Things get even more interesting in AWS:
    * Prior to Kubernetes 1.9, Classic was the only LB type that could be created in AWS by Kubernetes means. The `AWS Classic LoadBalancer` is created by default (it proxies TCP traffic (also on `spec.ports[*].nodePort`)). Meanwhile, the traffic comes from LoadBalaner's addresses instead of the client's address. Thus, the only way to find out the client's address is to enable the proxy protocol (you can do it by [annotating the service in Kubernetes](https://github.com/kubernetes/legacy-cloud-providers/blob/master/aws/aws.go)).
    * Network LoadBalancers [are supported](https://kubernetes.io/docs/concepts/services-networking/service/#network-load-balancer-support-on-aws-alpha) on AWS starting with Kubernetes 1.19. Such LoadBalancers run similarly to Azure and GCE ones — they forward traffic while keeping the client's source address.
