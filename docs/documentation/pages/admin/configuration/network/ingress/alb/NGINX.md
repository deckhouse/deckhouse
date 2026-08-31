---
title: "ALB with Ingress NGINX Controller"
permalink: en/admin/configuration/network/ingress/alb/nginx.html
description: "Configure Application Load Balancer with Ingress NGINX Controller in Deckhouse Kubernetes Platform. High availability setup, SSL termination, and traffic routing configuration."
extractedLinksMax: 4
relatedLinks:
  - title: "Migrating from ingress-nginx to alb"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/migration.html
  - title: "ALB with Kubernetes Gateway API"
    url: /products/kubernetes-platform/documentation/v1/admin/configuration/network/ingress/alb/alb-gateway-api.html
  - title: "Utilizing Application Load Balancer (ALB)"
    url: /products/kubernetes-platform/documentation/v1/user/network/ingress/alb.html
  - title: "ingress-nginx module documentation"
    url: /modules/ingress-nginx/
  - title: "ingress-nginx module Custom Resources"
    url: /modules/ingress-nginx/cr.html
  - title: "ingress-nginx module examples"
    url: /modules/ingress-nginx/examples.html
  - title: "metallb module documentation"
    url: /modules/metallb/
---

The [`ingress-nginx`](/modules/ingress-nginx/) module is used to implement ALB using the [Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx).

{% alert level="info" %}
In 2025, Ingress NGINX was [placed](https://kubernetes.io/blog/2025/11/11/ingress-nginx-retirement/) in maintenance mode, with no plans for active development of new features. Further evolution of inbound traffic load balancing in Kubernetes is focused on the [Gateway API](https://kubernetes.io/docs/concepts/services-networking/gateway/).

This does not apply to the module as part of Deckhouse Kubernetes Platform (DKP): the module is maintained by the DKP team, including security updates. Details are in ["Module support and security"](#module-support-and-security).

Step-by-step migration to Gateway API is in [Migrating from ingress-nginx to alb](migration.html).
{% endalert %}

The `ingress-nginx` module installs the Ingress NGINX Controller and manages it with custom resources.
If there is more than one node available for hosting the Ingress controller,
it is deployed in the HA mode, taking into account the infrastructure specifics of both cloud and bare-metal environments,
as well as various Kubernetes cluster types.

The module supports running and configuring several Ingress NGINX controllers simultaneously
(one of the controllers is the primary one. You can create as many additional controllers as you want).
This approach allows you to separate extranet and intranet Ingress resources of applications.

## Traffic termination options

Traffic to `ingress-nginx` can be routed in several ways:

- Directly without the use of an external load balancer.
- Using an external LoadBalancer. The following variants are supported:
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

The current `ingress-nginx` implementation has a Prometheus-based system for collecting statistical data with the following set of metrics:

- Total response time and backend response time separately
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

All graphs are grouped by Grafana dashboards. From any graph you can open a more detailed view:
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

## Load balancing configuration examples {#load-balancing-configuration-examples}

Use the [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) custom resource to configure load balancing.

{% tabs Environment examples %}
{% tab "AWS (NLB)" %}

### Example for AWS (Network Load Balancer)

When setting up the balancer, all available zones in the cluster are used.

Each zone's balancer receives its own public IP.
If a zone has an Ingress controller instance, its IP is added to the load balancer’s domain name as an A record.

If no instances remain in a zone, that IP is removed from DNS.

If only one Ingress controller instance exists in a zone, its IP is temporarily removed from DNS during pod restarts.

Example IngressNginxController with the [`LoadBalancer`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-loadbalancer) inlet and AWS NLB annotations:

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

{% endtab %}
{% tab "GCP, Yandex Cloud, and Azure" %}

### Example for GCP, Yandex Cloud, and Azure

IngressNginxController with the [`LoadBalancer`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-loadbalancer) inlet:

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

{% endtab %}
{% tab "OpenStack" %}

### Example for OpenStack

IngressNginxController with the [`LoadBalancerWithProxyProtocol`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-loadbalancerwithproxyprotocol) inlet and OpenStack Proxy Protocol annotations:

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

{% endtab %}
{% tab "VK Cloud" %}

### Example for VK Cloud

Use this configuration for an internal cloud balancer (without a public address).

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

{% endtab %}
{% tab "Bare metal (HostWithFailover)" %}

### Example for bare metal

IngressNginxController with the [`HostWithFailover`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-hostwithfailover) inlet on frontend nodes:

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

{% endtab %}
{% tab "Bare metal with external LB" %}

### Example for bare metal with external load balancer

Use this configuration with Cloudflare, Qrator, Nginx+, Citrix ADC, Kemp, or other external load balancers.

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

{% endtab %}
{% tab "MetalLB BGP" %}

### Example for bare metal (MetalLB in BGP LoadBalancer mode)

{% alert level="info" %}
Available in DKP Enterprise Edition only.
{% endalert %}

IngressNginxController with the [`LoadBalancer`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v2-spec-loadbalancer) inlet for use with MetalLB in BGP mode:

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

When using MetalLB, its speaker pods (MetalLB components that announce IP addresses) must run on the same nodes as the Ingress controller pods.

To preserve the real client IP addresses,
the Ingress controller Service should be created with `externalTrafficPolicy: Local` to avoid inter-node SNAT.
In this configuration, MetalLB speaker will only announce the Service from nodes running target pods.

Example ModuleConfig for the [`metallb`](/modules/metallb/configuration.html) module:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  enabled: true
  version: 2
  settings:
    speaker:
      nodeSelector:
        node-role.deckhouse.io/frontend: ""
      tolerations:
        - effect: NoExecute
          key: dedicated.deckhouse.io
          value: frontend
```

{% endtab %}
{% tab "MetalLB L2" %}

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

   {% alert level="info" %}
   MetalLB balancers should be placed on the same nodes as Ingress controllers. In [typical deployment scenarios](/products/kubernetes-platform/guides/hardware-requirements.html#deployment-scenarios), frontend nodes are used for this purpose. To deploy Ingress controllers and MetalLB load balancers on frontend nodes, set the label `node-role.deckhouse.io/frontend: ""` in `nodeSelector`.
   {% endalert %}

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
       node-role.deckhouse.io/frontend: "" # Load balancer node selector.
     type: L2
   ```

1. Create an [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) resource:

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
     nodeSelector:
       node-role.deckhouse.io/frontend: ""
     tolerations:
       - effect: NoExecute
         key: dedicated.deckhouse.io
         value: frontend
         operator: Equal
   ```

   {% alert level="info" %}
When creating an ingress controller, you can also specify certain IP addresses from the pool that will be assigned to its Service. Use the annotation `network.deckhouse.io/load-balancer-ips`.

If you need more than one address, also set `network.deckhouse.io/l2-load-balancer-external-ips-count` to the number of addresses allocated from the pool. That value must not be less than the number of addresses listed in `network.deckhouse.io/load-balancer-ips`.

See ["Example of using annotations"](/modules/metallb/examples.html#creating-a-service-and-assigning-it-specific-ip-addresses-from-the-pool) to assign specific addresses from the pool to the Service.
{% endalert %}

DKP will create a LoadBalancer Service with the specified number of IPs:

```shell
d8 k -n d8-ingress-nginx get svc
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME                   TYPE           CLUSTER-IP      EXTERNAL-IP                                 PORT(S)                      AGE
main-load-balancer     LoadBalancer   10.222.130.11   192.168.2.100,192.168.2.101,192.168.2.102   80:30689/TCP,443:30668/TCP   11s
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

{% endtab %}
{% endtabs %}

### Example of segregating access between public and administrative zones

In many applications, the same backend serves both the public part and the administrative interface. For example:

- `https://example.com` is the public zone;
- `https://admin.example.com` is the administrative zone, access to which must be restricted (`ACL`, `mTLS`, `IP whitelist`, and so on).

For this scenario, we recommend offloading administrative traffic to a separate Ingress controller (with a dedicated Ingress class if necessary) and restricting access to it by using the [`spec.acceptRequestsFrom`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) parameter.

{% tabs Zone segregation options %}
{% tab "Single Ingress controller" %}

#### Specifics of using a single Ingress controller

The example below shows a single Ingress controller serving requests from both the public zone and the administrative interface.

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

With [processing and forwarding of X-Forwarded-* headers enabled](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-hostport-behindl7proxy), the backend can rely on the `x-forwarded-host` header when making authorization decisions. In the example above, public Ingress traffic can reach the administrative zone via `x-forwarded-host`. Therefore, requests to the Ingress controller must come only from trusted sources.

{% endtab %}
{% tab "Separate Ingress controllers" %}

#### Using separate Ingress controllers

To avoid that situation, we recommend that you:

- Configure access rules at the Ingress resource level.
- Use separate Ingress controllers.
- Restrict which source addresses are allowed to connect to the Ingress controllers.

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
- The [`acceptRequestsFrom`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) parameter allows connections to the controller only from the listed subnets.
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

{% endtab %}
{% endtabs %}

## Module support and security

The `ingress-nginx` module is covered by DKP maintenance for the entire platform support lifecycle, regardless of the upstream project's development status. The DKP team tracks CVEs in the controller and its dependencies — NGINX, Lua modules, and base images — and delivers fixes in platform releases.

For compliance with PCI DSS expectations regarding vendor support and vulnerability remediation timelines, Flant is the responsible vendor of the module. DKP certification with FSTEC of Russia also covers vulnerability management processes and the release of security updates.
