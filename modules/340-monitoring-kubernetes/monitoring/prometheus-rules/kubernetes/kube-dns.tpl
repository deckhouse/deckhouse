{{- if ( .Values.global.enabledModules | has "kube-dns") }}
- name: kubernetes.dns
  rules:
  - alert: KubernetesDnsTargetDown
    expr: absent(up{job="kube-dns"} == 1)
    for: 5m
    labels:
      severity_level: "5"
      tier: cluster
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      summary: Kube-dns or CoreDNS are not being monitored.
      description: |-
        Prometheus is unable to collect metrics from `kube-dns`, which makes its status unknown.

        Steps to troubleshoot:

        1. Check the deployment details:

           ```bash
           d8 k -n kube-system describe deployment -l k8s-app=kube-dns
           ```

        2. Check the pod details:

           ```bash
           d8 k -n kube-system describe pod -l k8s-app=kube-dns
           ```
{{- end }}
