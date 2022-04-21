{{ define "instruction" }}
  {{- if .Values.global.enabledModules | has "control-plane-manager" }}
**Control-plane-manager** module is enabled. It means that certificates of control plane components and kubelets will renew automatically .
If you see this alert, it's probably because someone uses stale kubeconfig on ci-runner or in user's home directory.

To find who it is and where stale kubeconfig is located, you need to search in a kube-apiserver logs.
```
kubectl -n kube-system logs -l component=kube-apiserver --tail=-1 --timestamps -c kube-apiserver | grep "expire"
```
  {{- else }}
You need to use `kubeadm` to check control plane certificates.
1. Install kubeadm: `apt install kubeadm={{ .Values.global.discovery.kubernetesVersion }}.*`.
2. Check certificates: `kubeadm alpha certs check-expiration`

To check kubelet certificates, on each node you need to:
1. Check kubelet config:
```
ps aux \
  | grep "/usr/bin/kubelet" \
  | grep -o -e "--kubeconfig=\S*" \
  | cut -f2 -d"=" \
  | xargs cat
```
2. Find field `client-certificate` or `client-certificate-data`
3. Check certificate using openssl

There are no tools to help you find other stale kubeconfigs.
It will be better for you to enable `control-plane-manager` module to be able to debug in this case.
  {{- end }}
{{- end }}

- name: coreos.kubernetes
  rules:
  - record: pod:container_memory_usage_bytes:sum
    expr: sum(container_memory_usage_bytes{container!="POD",pod!=""}) BY
      (pod)
  - record: pod:container_spec_cpu_shares:sum
    expr: sum(container_spec_cpu_shares{container!="POD",pod!=""}) BY (pod)
  - record: pod:container_cpu_usage:sum
    expr: sum(rate(container_cpu_usage_seconds_total{container!="POD",pod!=""}[5m]))
      BY (pod)
  - record: pod:container_fs_usage_bytes:sum
    expr: sum(container_fs_usage_bytes{container!="POD",pod!=""}) BY (pod)
  - record: namespace:container_memory_usage_bytes:sum
    expr: sum(container_memory_usage_bytes{container!=""}) BY (namespace)
  - record: namespace:container_spec_cpu_shares:sum
    expr: sum(container_spec_cpu_shares{container!=""}) BY (namespace)
  - record: namespace:container_cpu_usage:sum
    expr: sum(rate(container_cpu_usage_seconds_total{container!="POD"}[5m]))
      BY (namespace)
  - record: cluster:memory_usage:ratio
    expr: sum(container_memory_usage_bytes{container!="POD",pod!=""}) BY
      (cluster) / sum(machine_memory_bytes) BY (cluster)
  - record: cluster:container_spec_cpu_shares:ratio
    expr: sum(container_spec_cpu_shares{container!="POD",pod!=""}) / 1000
      / sum(machine_cpu_cores)
  - record: cluster:container_cpu_usage:ratio
    expr: sum(rate(container_cpu_usage_seconds_total{container!="POD",pod!=""}[5m]))
      / sum(machine_cpu_cores)
  - record: apiserver_latency_seconds:quantile
    expr: histogram_quantile(0.99, rate(apiserver_request_latencies_bucket[5m])) /
      1e+06
    labels:
      quantile: "0.99"
  - record: apiserver_latency:quantile_seconds
    expr: histogram_quantile(0.9, rate(apiserver_request_latencies_bucket[5m])) /
      1e+06
    labels:
      quantile: "0.9"
  - record: apiserver_latency_seconds:quantile
    expr: histogram_quantile(0.5, rate(apiserver_request_latencies_bucket[5m])) /
      1e+06
    labels:
      quantile: "0.5"
  - alert: K8SApiserverDown
    expr: absent(up{job="kube-apiserver"} == 1)
    for: 20m
    labels:
      severity_level: "3"
    annotations:
      plk_protocol_version: "1"
      description: No API servers are reachable or all have disappeared from service
        discovery
      summary: No API servers are reachable
  - alert: K8sCertificateExpiration
    expr: sum(label_replace(rate(apiserver_client_certificate_expiration_seconds_bucket{le="604800", job=~"kubelet|kube-apiserver"}[1m]) > 0, "component", "$1", "job", "(.*)")) by (component, node)
    labels:
      severity_level: "6"
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      description: |
        Some clients connect to {{`{{$labels.component}}`}} with certificate which expiring soon (less than 7 days) on node {{`{{$labels.node}}`}}.
        {{- include "instruction" . | nindent 8 }}
      summary: Kubernetes has API clients with soon expiring certificates
  - alert: K8sCertificateExpiration
    expr: sum(label_replace(rate(apiserver_client_certificate_expiration_seconds_bucket{le="86400", job=~"kubelet|kube-apiserver"}[1m]) > 0, "component", "$1", "job", "(.*)")) by (component, node)
    labels:
      severity_level: "5"
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      description: |
        Some clients connect to {{`{{$labels.component}}`}} with certificate which expiring soon (less than 1 day) on node {{`{{$labels.component}}`}}.
        {{- include "instruction" . | nindent 8 }}
      summary: Kubernetes has API clients with soon expiring certificates
