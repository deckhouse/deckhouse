- name: d8.istio.versions
  rules:
    - alert: D8IstioDeprecatedIstioVersionInstalled
      annotations:
        summary: The installed Istio version has been deprecated.
        description: |
          Deckhouse has detected that a deprecated Istio version `{{"{{$labels.version}}"}}` is installed.

          Support for this version will be removed in upcoming Deckhouse releases. The higher the alert severity, the greater the probability of support being discontinued.

          To learn how to upgrade Istio, refer to the [upgrade guide]({{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}{{- else }}https://deckhouse.io{{- end }}/modules/istio/examples.html#upgrading-istio).
        plk_markup_format: markdown
        plk_labels_as_annotations: pod,instance
        plk_protocol_version: "1"
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
        summary: The installed Istio version is incompatible with the Kubernetes version.
        description: |
          The installed Istio version `{{"{{$labels.istio_version}}"}}` may not work properly with the current Kubernetes version `{{"{{$labels.k8s_version}}"}}` because it's not supported officially.

          To resolve the issue, upgrade Istio following the [guide]({{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}{{- else }}https://deckhouse.io{{- end }}/products/kubernetes-platform/documentation/{{ $.Values.global.deckhouseVersion }}/modules/istio/examples.html#upgrading-istio).
        plk_markup_format: markdown
        plk_labels_as_annotations: pod,instance
        plk_protocol_version: "1"
      expr: |
        d8_telemetry_istio_version_incompatible_with_k8s_version{}
      for: 5m
      labels:
        severity_level: "3"
        tier: cluster
