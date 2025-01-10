- name: d8.istio.versions
  rules:
    - alert: D8IstioDeprecatedIstioVersionInstalled
      annotations:
        description: |
          There is deprecated istio version `{{"{{$labels.version}}"}}` installed.
          Impact — version support will be removed in future deckhouse releases. The higher alert severity — the higher probability of support cancelling.
          Read [documentation]({{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}{{- else }}https://deckhouse.io{{- end }}/products/kubernetes-platform/documentation/{{ $.Values.global.deckhouseVersion }}/modules/istio/examples.html#upgrading-istio) on upgrading Istio.
        plk_markup_format: markdown
        plk_labels_as_annotations: pod,instance
        plk_protocol_version: "1"
        summary: There is deprecated istio version installed
      expr: |
        d8_istio_deprecated_version_installed{}
      for: 5m
      labels:
        severity_level: "{{"{{$labels.alert_severity}}"}}"
        tier: cluster
- name: d8.istio.k8sVersionsCompatibility
  rules:
    - alert: D8IstioVersionIsIncompatibleWithK8sVersion
      annotations:
        description: |
          The current istio version `{{"{{$labels.istio_version}}"}}` may not work properly with the current k8s version `{{"{{$labels.k8s_version}}"}}`, because it is unsupported officially.
          Please upgrade istio as soon as possible.
          Read [documentation]({{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}{{- else }}https://deckhouse.io{{- end }}/products/kubernetes-platform/documentation/{{ $.Values.global.deckhouseVersion }}/modules/istio/examples.html#upgrading-istio) on upgrading Istio.
        plk_markup_format: markdown
        plk_labels_as_annotations: pod,instance
        plk_protocol_version: "1"
        summary: The installed istio version is incompatible with the k8s version
      expr: |
        d8_telemetry_istio_version_incompatible_with_k8s_version{}
      for: 5m
      labels:
        severity_level: "3"
        tier: cluster
