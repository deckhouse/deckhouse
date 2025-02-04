---
title: "ALB means NGINX Ingress controller"
permalink: en/admin/alb-nginx.html
---

The [ingress-nginx](ingress-nginx) module is used to implement ALB by means of [NGINX Ingress controller](https://github.com/kubernetes/ingress-nginx).

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/ingress-nginx/ -->

The `ingress-nginx` module installs the NGINX Ingress controller and manages it with Custom Resources. The module installs the Ingress controller in the HA mode if there is more than one node. In doing so, it takes into account all the aspects of cloud / bare metal infrastructure and various types of Kubernetes clusters.

The module supports running and configuring several NGINX Ingress controllers simultaneously (one of the controllers is the **primary** one; you can create as many **additional** controllers as you want). This approach allows you to separate extranet and intranet Ingress resources of applications.

## Traffic routing

Traffic to nginx-ingress can be routed in several ways:

* directly without the use of an external load balancer;
* using an external LoadBalancer; the following LB varieties are supported:
  * Qrator
  * Cloudflare
  * AWS LB
  * GCE LB
  * ACS LB
  * Yandex LB
  * OpenStack LB

## Terminating HTTPS

The module allows you to manage HTTPS security policies for each of the NGINX Ingress controllers, including:

* hsts parameters;
* available SSL/TLS versions and encryption protocols.

The module integrates with the [cert-manager](../../modules/cert-manager/) module. Thus, it can get SSL certificates automatically and pass them to NGINX Ingress controllers for further use.

## Monitoring and statistics

Our Ingress Nginx implementation has a Prometheus-based system for collecting statistical data built-in. It uses a variety of metrics based on:

* the overall and upstream response time;
* response codes;
* number of repeated requests (retries);
* request and response sizes;
* request methods;
* `content-types`;
* geography of requests, etc.

The data can be grouped by the:

* `namespace`,
* `vhost`,
* `ingress` resource,
* `location` (in nginx).

All graphs are conveniently grouped by Grafana dashboards; also, you can do a drill-down for any graph: e.g., you can instantly shift from an overview of the namespace to a more detailed view of, say, `vhosts` in this namespace by simply clicking the link on the Grafana dashboard.

## Statistics

### Basic principles of collecting statistics

1. Our module is called for each request (at the `log_by_lua_block` stage). It calculates the necessary data and forwards it to the buffer (each nginx worker has its own buffer).
2. For every nginx worker at the `init_by_lua_block` stage), the process is run that asynchronously sends data in the `protobuf` format over a tcp socket to `protobuf_exporter` (our in-house development) once a second.
3. `protobuf_exporter` runs as a sidecar container in the ingress-controller's Pod. It receives messages in the protobuf format, parses them, aggregates them according to the specified rules, and exports them in the Prometheus format.
4. Every 30 seconds, Prometheus scrapes both the ingress-controller (since it exports some of the required metrics) and protobuf_exporter. Then these data are used for stats.

### What information does Prometheus collect, and in what form?

All the collected metrics have service labels that allow you to identify the controller's instance: `controller`, `app`, `instance`, and `endpoint` (displayed in `/prometheus/targets`).

* All metrics (except for geo) exported by protobuf_exporter have three levels of detail:
  * `ingress_nginx_overall_*` — the "bird's-eye view"; all the metrics have `namespace`, `vhost`, and `content_kind` labels attached;
  * `ingress_nginx_detail_*` — the `ingress`, `service`, `service_port`, and `location` labels are added to those listed above;
  * `ingress_nginx_detail_backend_*` — some detailed data; they are collected on a per-backend basis. The `pod_ip` label is added to those listed for the detail level.

* The following metrics are collected for the overall and detail levels:
  * `*_requests_total` — the total number of requests (additional labels: `scheme`, `method`);
  * `*_responses_total` — the total number of responses (additional labels: `status`);
  * `*_request_seconds_{sum,count,bucket}` — histogram of the response time;
  * `*_bytes_received_{sum,count,bucket}` — histogram of the request size;
  * `*_bytes_sent_{sum,count,bucket}` — histogram of the response size;
  * `*_upstream_response_seconds_{sum,count,bucket}` — histogram of the upstream response time (the sum of the response times of all upstreams is used if several of them are present);
  * `*_lowres_upstream_response_seconds_{sum,count,bucket}` — the same as above but less detailed (it is suitable for visualization, but not at all for calculating quantiles);
  * `*_upstream_retries_{count,sum}` — the number of requests for which retries were sent to backends, and the number of retries;

* The following metrics are collected for the overall level:
  * `*_geohash_total` — the total number of requests with a specific geohash (additional labels: `geohash`, `place`);

* The following metrics are collected for the detail_backend level:
  * `*_lowres_upstream_response_seconds` — same as a similar metric for overall and detail;
  * `*_responses_total` — the total number of responses (additional labels: `status_class` instead of `status`);
  * `*_upstream_bytes_received_sum` — the sum of the backend's response sizes.
