---
title: "The ingress-nginx module"
description: "HTTP/HTTPS traffic balancing and termination in the Deckhouse Kubernetes Platform cluster using Ingress NGINX Controller."
---

Installs and manages the [Ingress NGINX Controller](https://github.com/kubernetes/ingress-nginx) using custom resources. The module installs the Ingress NGINX Controller in the HA mode if there is more than one node. In doing so, it takes into account all the aspects of cloud and bare-metal infrastructure and various types of Kubernetes clusters.

The module supports running and configuring several Ingress NGINX Controllers simultaneously (one of the controllers is the **primary** one; you can create any number of **additional** controllers). This approach allows you to separate extranet and intranet Ingress resources of applications.

## Traffic routing

Traffic to `ingress-nginx` can be routed in several ways:

- Directly without the use of an external load balancer
- Using an external LoadBalancer; the following LB varieties are supported:
  - Qrator
  - Cloudflare
  - AWS LB
  - GCE LB
  - ACS LB
  - Yandex LB
  - OpenStack LB

## Terminating HTTPS

The module allows you to manage HTTPS security policies for each Ingress NGINX Controller, including:

- HSTS parameters
- Available SSL/TLS versions and encryption protocols

The module integrates with the [`cert-manager`](/modules/cert-manager/) module. Thus, it can get SSL certificates automatically and pass them to Ingress NGINX Controllers for further use.

## Monitoring and statistics

Our Ingress Nginx implementation has a Prometheus-based system for collecting statistical data built-in. It uses a variety of metrics based on:

- The overall and upstream response time
- Response codes
- Number of repeated requests (retries)
- Request and response sizes
- Request methods
- `content-types`
- Geography of requests, and others

The data can be grouped by the:

- `namespace`
- `vhost`
- `ingress` resource
- `location` (in nginx)

The graphs are collected in convenient dashboards in Grafana, and there is a drill-down option for graphs. For example, when viewing namespace statistics, you can click a link to a dashboard in Grafana and drill down into the `vhosts` statistics in the corresponding `namespace`.

## Statistics

### Basic principles of collecting statistics

1. The module is called for each request (at the `log_by_lua_block` stage). It calculates the necessary data and forwards it to the buffer (each nginx worker has its own buffer).
1. For every nginx worker at the `init_by_lua_block` stage, the process is run that asynchronously sends data in the `protobuf` format over a tcp socket to `protobuf_exporter` (DKP development) once a second.
1. `protobuf_exporter` runs as a sidecar container in the ingress-controller's Pod. It receives messages in the protobuf format, parses them, aggregates them according to the specified rules, and exports them in the Prometheus format.
1. Every 30 seconds, Prometheus scrapes both the ingress-controller (since it exports some of the required metrics) and protobuf_exporter. Then these data are used for stats.

### What information does Prometheus collect, and in what form?

All the collected metrics have service labels that allow you to identify the controller's instance: `controller`, `app`, `instance`, and `endpoint` (displayed in `/prometheus/targets`).

- All metrics (except for geo) exported by protobuf_exporter have three levels of detail:
  - `ingress_nginx_overall_*` — the "bird's-eye view"; all the metrics have `namespace`, `vhost`, and `content_kind` labels attached;
  - `ingress_nginx_detail_*` — — in addition to the `overall` level labels, `ingress`, `service`, `service_port` and `location` are added;
  - `ingress_nginx_detail_backend_*` — some detailed data; they are collected on a per-backend basis. The `pod_ip` label is added to those listed for the detail level.

- The following metrics are collected for the overall and detail levels:
  - `*_requests_total` — the total number of requests (additional labels: `scheme`, `method`);
  - `*_responses_total` — the total number of responses (additional labels: `status`);
  - `*_request_seconds_{sum,count,bucket}` — histogram of the response time;
  - `*_bytes_received_{sum,count,bucket}` — histogram of the request size;
  - `*_bytes_sent_{sum,count,bucket}` — histogram of the response size;
  - `*_upstream_response_seconds_{sum,count,bucket}` — histogram of the upstream response time (the sum of the response times of all upstreams is used if several of them are present);
  - `*_lowres_upstream_response_seconds_{sum,count,bucket}` — the same as above but less detailed (it is suitable for visualization, but not at all for calculating quantiles);
  - `*_upstream_retries_{count,sum}` — the number of requests for which retries were sent to backends, and the number of retries;

- The following metrics are collected for the overall level:
  - `*_geohash_total` — the total number of requests with a specific geohash (additional labels: `geohash`, `place`);

- The following metrics are collected for the detail_backend level:
  - `*_lowres_upstream_response_seconds` — same as a similar metric for overall and detail;
  - `*_responses_total` — the total number of responses (additional labels: `status_class` instead of `status`);
  - `*_upstream_bytes_received_sum` — the sum of the backend's response sizes.
