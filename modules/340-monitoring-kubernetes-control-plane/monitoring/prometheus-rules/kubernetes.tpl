{{ define "instruction" }}
  {{- if .Values.global.enabledModules | has "control-plane-manager" }}
**Control-plane-manager** module is enabled, meaning control plane component certificates and kubelets will renew automatically.
If you see this alert, it's probably because someone uses a stale kubeconfig on ci-runner or in a user's home directory.

To identify who is using a stale kubeconfig and where it is stored, search through the kube-apiserver logs:

```bash
kubectl -n kube-system logs -l component=kube-apiserver --tail=-1 --timestamps -c kube-apiserver | grep "expire"
```
  {{- else }}
To check control plane certificates, use kubeadm:

1. Install kubeadm using the following command:
   
   ```bash
   apt install kubeadm={{ .Values.global.discovery.kubernetesVersion }}.*
   ```

2. Check certificates:

   ```bash
   kubeadm alpha certs check-expiration
   ```

To check kubelet certificates, do the following on each node:

1. Check kubelet configuration:

   ```bash
   ps aux \
     | grep "/usr/bin/kubelet" \
     | grep -o -e "--kubeconfig=\S*" \
     | cut -f2 -d"=" \
     | xargs cat
   ```

2. Locate the `client-certificate` or `client-certificate-data` field.
3. Check certificate expiration using OpenSSL.

Note that there are no tools to find other stale kubeconfig files.
Consider enabling the `control-plane-manager` module for advanced debugging.
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
      summary: API servers can't be reached.
      description: No API servers are reachable, or they have all disappeared from service discovery.
  - alert: K8sCertificateExpiration
    expr: sum(label_replace(rate(apiserver_client_certificate_expiration_seconds_bucket{le="604800", job=~"kubelet|kube-apiserver"}[1m]) > 0, "component", "$1", "job", "(.*)")) by (component, node)
    labels:
      severity_level: "6"
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Kubernetes has API clients with soon-to-expire certificates.
      description: |
        Some clients are connecting to {{`{{$labels.component}}`}} with certificates that will expire in less than 7 days on node `{{`{{$labels.node}}`}}`.
        {{- include "instruction" . | nindent 8 }}
  - alert: K8sCertificateExpiration
    expr: sum(label_replace(rate(apiserver_client_certificate_expiration_seconds_bucket{le="86400", job=~"kubelet|kube-apiserver"}[1m]) > 0, "component", "$1", "job", "(.*)")) by (component, node)
    labels:
      severity_level: "5"
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Kubernetes has API clients with soon-to-expire certificates.
      description: |
        Some clients are connecting to {{`{{$labels.component}}`}} with certificates that will expire in less than a day on node `{{`{{$labels.component}}`}}`.
        {{- include "instruction" . | nindent 8 }}
