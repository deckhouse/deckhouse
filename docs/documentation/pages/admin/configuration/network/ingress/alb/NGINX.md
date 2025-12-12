---
title: "ALB with Ingress NGINX Controller"
permalink: en/admin/configuration/network/ingress/alb/nginx.html
description: "Configure Application Load Balancer with Ingress NGINX Controller in Deckhouse Kubernetes Platform. High availability setup, SSL termination, and traffic routing configuration."
---

The [`ingress-nginx`](/modules/ingress-nginx/) module is used to implement ALB using the [Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx).

The `ingress-nginx` module installs the Ingress NGINX Controller and manages it with custom resources.
If there is more than one node available for hosting the Ingress controller,
it is deployed in the HA mode, taking into account the infrastructure specifics of both cloud and bare-metal environments,
as well as various Kubernetes cluster types.

The module supports running and configuring several Ingress NGINX controllers simultaneously
(one of the controllers is the **primary** one; you can create as many **additional** controllers as you want).
This approach allows you to separate extranet and intranet Ingress resources of applications.

## Traffic termination options

Traffic to `ingress-nginx` can be routed in several ways:

- Directly without the use of an external load balancer.
- Using an external LoadBalancer; the following variants are supported:
  - Qrator
  - Cloudflare
  - AWS LB
  - GCE LB
  - ACS LB
  - Yandex LB
  - OpenStack LB

## HTTPS termination

The module allows you to manage HTTPS security policies for each of the Ingress NGINX controllers, including:

- HSTS parameters
- Available SSL/TLS versions and encryption protocols

The module is integrated with the [`cert-manager`](/modules/cert-manager/) module.
Thus, it can get SSL certificates automatically and pass them to Ingress NGINX controllers for further use.

## Monitoring and statistics

The current `ingress-nginx` implementation has a Prometheus-based system for collecting statistical data.
It uses a variety of metrics based on:

- The overall and upstream response time
- Response codes
- Number of repeated requests (retries)
- Request and response sizes
- Request methods
- `content-types`
- Geography of requests, etc.

The data can be grouped by the:

- `namespace`
- `vhost`
- `ingress` resources
- `location` (in nginx)

All graphs are grouped by Grafana dashboards. Also, you can do a drill-down for any graph:
for example, from a `namespace` statistics view, you can click through to the corresponding `vhost` dashboard for more detail,
and continue down the hierarchy.

## Statistics

### Basic principles of collecting statistics

1. At the `log_by_lua_block` stage, the module calculates the necessary metrics for each request
   and stores them in a buffer (each NGINX worker has its own buffer).
1. At the `init_by_lua_block` stage, each NGINX worker starts a process that sends data in `protobuf` format via TCP socket
   to the `protobuf_exporter` every second (developed by Deckhouse Kubernetes Platform).
1. `protobuf_exporter` runs as a sidecar container in the Ingress controller pod, receives `protobuf` messages,
   parses and aggregates them, and exports metrics for Prometheus.
1. Prometheus scrapes metrics every 30 seconds from both the Ingress controller and the `protobuf_exporter`.
   This scraped data is what statistics is based on.

### Metrics structure and representation

All collected metrics include service labels identifying the controller instance:
`controller`, `app`, `instance`, and `endpoint` (visible in `/prometheus/targets`).

- All non-geo metrics exported by `protobuf_exporter` are provided at three detail levels:
  - `ingress_nginx_overall_*`: Top-level aggregated metrics
    (non-detailed, all metrics have the following labels: `namespace`, `vhost`, `content_kind`).
  - `ingress_nginx_detail_*`: In addition to overall metrics, adds `ingress`, `service`, `service_port`, and `location`.
  - `ingress_nginx_detail_backend_*`: Backend-level metrics. In addition to detail metrics, adds the `pod_ip` label.

- Metrics collected for overall and detail levels:
  - `*_requests_total`: Total requests (extra labels: `scheme`, `method`).
  - `*_responses_total`: Number of responses (extra label: `status`).
  - `*_request_seconds_{sum,count,bucket}`: Response time histogram.
  - `*_bytes_received_{sum,count,bucket}`: Request size histogram.
  - `*_bytes_sent_{sum,count,bucket}`: Response size histogram.
  - `*_upstream_response_seconds_{sum,count,bucket}`: Upstream service response time histogram (total for multiple upstreams).
  - `*_lowres_upstream_response_seconds_{sum,count,bucket}`: Simplified histogram
    (for visualization; can't be used for quantiles).
  - `*_upstream_retries_{count,sum}`: Number and total of backend retries.

- Metrics collected for overall level:
  - `*_geohash_total`: Request counts per geohash (additional labels: `geohash`, `place`).

- Metrics collected for detail_backend level:
  - `*_lowres_upstream_response_seconds`: Simplified response time histogram for overall and detail.
  - `*_responses_total`: Number of responses (additional label: `status_class`, not just `status`).
  - `*_upstream_bytes_received_sum`: Total size of data received from backends.

## Load balancing configuration examples

Use the [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) custom resource to configure load balancing.

### Example for AWS (Network Load Balancer)

When setting up the balancer, all available zones in the cluster are used.

Each zone's balancer receives its own public IP.
If a zone has an Ingress controller instance, its IP is added to the load balancer’s domain name as an A record.

If no instances remain in a zone, that IP is removed from DNS.

If only one Ingress controller instance exists in a zone, its IP is temporarily removed from DNS during pod restarts.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

### Example for GCP / Yandex Cloud / Azure

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
```

{% alert level="info" %}
In GCP, nodes must have an annotation allowing external connections for NodePort services.
{% endalert %}

### Example for OpenStack

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main-lbwpp
spec:
  inlet: LoadBalancerWithProxyProtocol
  ingressClass: nginx
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.org/proxy-protocol: "true"
      loadbalancer.openstack.org/timeout-member-connect: "2000"
```

### Example for VK Cloud

The following example is relevant when the internal balancer would be used.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/openstack-internal-load-balancer: "true"
  nodeSelector:
    node.deckhouse.io/group: worker
```

### Example for bare metal

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostWithFailover
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: frontend
```

### Example for bare metal with external load balancer

The following example is relevant when using Cloudflare, Qrator, Nginx+, Citrix ADC, Kemp or other external load balancers.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
    behindL7Proxy: true
```

### Example for bare metal (MetalLB in BGP LoadBalancer mode)

{% alert level="info" %}
Available in DKP Enterprise Edition only.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: frontend
```

When using MetalLB, its speaker pods must run on the same nodes as the Ingress controller pods.

To preserve the real client IP addresses,
the Ingress controller Service should be created with `externalTrafficPolicy: Local` to avoid inter-node SNAT.
In this configuration, MetalLB speaker will only announce the Service from nodes running target pods.

Example [`metallb`](/modules/metallb/configuration.html) configuration:

```yaml
metallb:
 speaker:
   nodeSelector:
     node-role.deckhouse.io/frontend: ""
   tolerations:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
```

### Example for bare metal (MetalLB in L2 LoadBalancer mode)

{% alert level="info" %}
Available in DKP Enterprise Edition only.
{% endalert %}

1. Enable the [`metallb`](/modules/metallb/) module:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: metallb
   spec:
     enabled: true
     version: 2
   ```

1. Create a [MetalLoadBalancerClass](/modules/metallb/cr.html#metalloadbalancerclass) resource:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: MetalLoadBalancerClass
   metadata:
     name: ingress
   spec:
     addressPool:
       - 192.168.2.100-192.168.2.150
     isDefault: false
     nodeSelector:
       node-role.kubernetes.io/loadbalancer: "" # Load balancer node selector.
     type: L2
   ```

1. Create a [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) resource:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: IngressNginxController
   metadata:
     name: main
   spec:
     ingressClass: nginx
     inlet: LoadBalancer
     loadBalancer:
       loadBalancerClass: ingress
       annotations:
         # Number of addresses to allocate from the pool defined in MetalLoadBalancerClass.
         network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
   ```

The platform will create a LoadBalancer Service with the specified number of IPs:

```shell
d8 k -n d8-ingress-nginx get svc
```

Example output:

```console
NAME                   TYPE           CLUSTER-IP      EXTERNAL-IP                                 PORT(S)                      AGE
main-load-balancer     LoadBalancer   10.222.130.11   192.168.2.100,192.168.2.101,192.168.2.102   80:30689/TCP,443:30668/TCP   11s
```

### Example of segregating access between public and administrative zones

In many applications, the same backend serves both the public part and the administrative interface. For example:

- `https://example.com` is the public zone;
- `https://admin.example.com` is the administrative zone, access to which must be restricted (`ACL`, `mTLS`, `IP whitelist`, and so on).

For this scenario, we recommend offloading administrative traffic to a separate Ingress controller (with a dedicated Ingress class if necessary) and restricting access to it by using the [`spec.acceptRequestsFrom`](cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) parameter.

#### Specifics of using a single Ingress controller

Consider an example where a single Ingress controller is used to serve requests from both the public zone and the administrative interface.

Example of Ingress resource configuration for this case:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: admin-ingress
  annotations:
    nginx.ingress.kubernetes.io/whitelist-source-range: "1.2.3.4/32"
spec:
  ingressClassName: nginx # The Ingress resource for administrative traffic is associated with the same Ingress controller as the Ingress resource for public traffic.
  rules:
    - host: admin.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: backend
                port:
                  number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: public-ingress
spec:
  ingressClassName: nginx # The Ingress resource for public traffic is associated with the same Ingress controller as the Ingress resource for administrative traffic.
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: backend
                port:
                  number: 80
```

With [processing and forwarding of X-Forwarded-* headers enabled](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-hostport-behindl7proxy), the backend can rely on the `x-forwarded-host` header when making authorization decisions. In the example above, it is possible to reach the administrative zone through the Ingress resource that serves public traffic by using `x-forwarded-host`. Therefore, when using this option you must be sure that requests to the Ingress controller come only from trusted sources.

#### Using separate Ingress controllers

To avoid the situation described above (when, with [processing and forwarding of X-Forwarded-* headers enabled](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-hostport-behindl7proxy) it is possible, for example, to reach the administrative zone via the Ingress resource that serves public traffic by using `x-forwarded-host`), we recommend that you:

- configure access rules at the Ingress resource level,
- use separate Ingress controllers,
- restrict which source addresses are allowed to connect to the Ingress controllers.

Example of Ingress resource configuration for this case:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: admin-ingress
  annotations:
    nginx.ingress.kubernetes.io/whitelist-source-range: "1.2.3.4/32"
spec:
  ingressClassName: admin-nginx # The Ingress resource for administrative traffic is associated with a separate Ingress controller.
  rules:
    - host: admin.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: backend
                port:
                  number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: public-ingress
spec:
  ingressClassName: public-nginx # The Ingress resource for public traffic is associated with a separate Ingress controller.
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: backend
                port:
                  number: 80
```

Example of an Ingress controller that serves administrative Ingress resources and accepts connections only from specified subnets:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: admin
spec:
  ingressClass: admin-nginx
  inlet: HostPort
  acceptRequestsFrom:
    - 1.2.3.4/32
    - 10.0.0.0/16
  hostPort:
    httpPort: 80
    httpsPort: 443
    behindL7Proxy: true
```

In this example:

- The Ingress controller is exposed on node ports through the `HostPort` inlet.
- The [`acceptRequestsFrom`](cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) parameter allows connections to the controller only from the listed subnets.
- Even if an external load balancer or client can set its own `X-Forwarded-*` header values, the decision whether to allow the connection to reach the controller is made based on the actual source address, not on headers.
- Administrative Ingress resources (in this example `admin-ingress`) are served by this controller according to the configured Ingress class.

Example of an Ingress controller that serves Ingress resources for public traffic:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: public
spec:
  ingressClass: public-nginx
  inlet: HostPort
  hostPort:
    httpPort: 8080
    httpsPort: 8443
    behindL7Proxy: true
```
