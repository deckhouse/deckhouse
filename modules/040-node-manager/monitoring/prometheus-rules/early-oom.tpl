{{- if .Values.nodeManager.earlyOomEnabled }}
- name: d8.early-oom.availability
  rules:
  - alert: EarlyOOMPodIsNotReady
    expr: min by (pod) (early_oom_psi_unavailable{namespace="d8-cloud-instance-manager", pod=~"early-oom-.*"}) == 1
    for: 3m
    labels:
      severity_level: "8"
      tier: cluster
      d8_module: node-manager
      d8_component: early-oom
    annotations:
      plk_protocol_version: "1"
      plk_markup_format: "markdown"
      plk_create_group_if_not_exists__d8_early_oom_malfunctioning: "EarlyOOMPodIsNotReadyGroup,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_grouped_by__d8_early_oom_malfunctioning: "EarlyOOMPodIsNotReadyGroup,tier=cluster,prometheus=deckhouse,kubernetes=~kubernetes"
      plk_labels_as_annotations: "pod"
      summary: Pod {{`{{$labels.pod}}`}} has detected an unavailable PSI subsystem.
      description: |-
        The Pod {{`{{$labels.pod}}`}} detected that the Pressure Stall Information (PSI) subsystem is unavailable.

        For details, check the logs:

        ```shell
        d8 k -n d8-cloud-instance-manager logs {{`{{$labels.pod}}`}}
        ```

        Troubleshooting options:

        - Upgrade the Linux kernel to version 4.20 or higher.
        - Enable the [Pressure Stall Information](https://docs.kernel.org/accounting/psi.html).
        - [Disable early OOM]({{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation")}}/en/platform/modules{{- else }}https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules{{- end }}/node-manager/configuration.html#parameters-earlyoomenabled).
{{- else }}
[]
{{- end }}
