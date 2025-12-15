---
title: "Incoming traffic management"
permalink: en/stronghold/documentation/admin/platform-management/network/ingress.html
lang: en
---

To provide external access to virtual machines, for example, for service publishing or remote administration, you can use `Ingress`resources, which are managed by the ingress-nginx module.

These created `Ingress` resources use Nginx as a reverse proxy and load balancer.
If the cluster includes multiple nodes for hosting the Ingress controller, it will be deployed in failover mode,
enhancing access reliability and resilience to failures.

Multiple instances of the Ingress NGINX Controller can run with separate configurations:
one primary controller and any number of additional controllers.
This approach lets you separate handling of `Ingress` resources for external and internal (intranet) applications,
ensuring their isolation and more flexible access control.

## Create a controller

To create a Ingress NGINX Controller, apply the following IngressNginxController resource:

```yaml
d8 k apply -f - <<EOF
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
EOF
```

For configuration details on the `IngressNginxController` resource, refer to the [corresponding article](/modules/ingress-nginx/cr.html#ingressnginxcontroller).

### HTTPS termination

The `ingress-nginx` module lets you configure HTTPS security policies for each Ingress NGINX Сontroller, including:

- Managing HTTP Strict Transport Security (HSTS) parameters
- Configuring supported versions of SSL/TLS and encryption protocols

Also, ingress-nginx is integrated with the cert-manager module,
which can be used for automatic ordering of SSL certificates and their use by the controllers.

### Monitoring and statistics

The DVP `ingress-nginx` implementation includes a statistics collection system in Prometheus,
providing various metrics:

- Overall response time and upstream time
- Response codes
- Number of request retries
- Request and response sizes
- Request methods
- `content-type` types
- Geographic distribution of requests

Data is displayed in multiple views:

- By namespace
- By `vhost`
- By Ingress resource
- By `location` (in NGINX)

All graphs are available on user-friendly dashboards in Grafana.
You can drill down into the data.
For example, when viewing statistics by namespace, you can explore deeper statistics by `vhosts` within that namespace
by clicking the corresponding link on the Grafana dashboard.

#### Basic principles of statistics collection

1. At the `log_by_lua_block` stage, a module is called for each request to calculate and store the required data in a buffer.
    Each NGINX worker has its own buffer.
1. At the `init_by_lua_block` stage, a process is started for each NGINX worker.
    This process asynchronously sends data in `protobuf` format via a TCP socket to `protobuf_exporter` once per second.
1. `protobuf_exporter` runs as a sidecar container in the pod with the Ingress controller.
    It receives, parses, and aggregates the `protobuf` messages according to predefined rules,
    and then exports the data in a format compatible with Prometheus.
1. Prometheus scrapes metrics every 30 seconds from both the Ingress controller,
    which collects a small number of necessary metrics, and `protobuf_exporter`.
    This ensures efficient system performance.

#### Statistics collection and representation

All collected metrics have service labels visible in `/prometheus/targets` that let you identify the controller instance: controller, app, instance, and endpoint.

- All metrics exported by protobuf_exporter (except for geo) are represented in three levels of detail:
  - `ingress_nginx_overall_*`: An overview. All metrics have the `namespace`, `vhost`, and `content_kind` labels.
  - `ingress_nginx_detail_*`: In addition to the overall labels, this level adds `ingress`, `service`, `service_port`, and `location` labels.
  - `ingress_nginx_detail_backend_*`: A limited part of the data collected on a per-backend basis.
  In addition to the detail labels, these metrics have the `pod_ip` label.

- The following metrics are collected for the overall and detail levels:
  - `*_requests_total`: A request number counter. Additional labels: `scheme`, `method`.
  - `*_responses_total`: A response number counter. Additional label: `status`.
  - `*_request_seconds_{sum,count,bucket}`: A response time histogram.
  - `*_bytes_received_{sum,count,bucket}`: A request size histogram.
  - `*_bytes_sent_{sum,count,bucket}`: A response size histogram.
  - `*_upstream_response_seconds_{sum,count,bucket}`: An upstream response time histogram.
  If there were multiple upstreams, it tracks the sum of response times of all upstreams.
  - `*_lowres_upstream_response_seconds_{sum,count,bucket}`: The same as the previous metric but less detailed.
  It's suitable for visualization but not for quantile calculation.
  - `*_upstream_retries_{count,sum}`: A number of requests with backend retries and the sum of retries.

- The following metrics are collected for the overall level:
  - `*_geohash_total`: A counter for the number of requests with a specific geohash. Additional labels: `geohash`, `place`.

- The following metrics are collected for the detail_backend level:
  - `*_lowres_upstream_response_seconds`: The same metric as the one used for the overall and detail levels.
  - `*_responses_total`: A response number counter. Additional label: `status_class`.
  - `*_upstream_bytes_received_sum`: A counter for the total size of backend responses.
