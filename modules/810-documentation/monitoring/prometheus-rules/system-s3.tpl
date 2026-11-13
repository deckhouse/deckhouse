{{- if and (eq (include "documentation_s3_requested" .) "System") (not (has "system-s3" .Values.global.enabledModules)) }}
- name: d8.documentation.system-s3
  rules:
    - alert: D8DocumentationSystemS3Disabled
      expr: vector(1)
      labels:
        severity_level: "9"
        tier: cluster
        d8_module: documentation
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: markdown
        summary: Module versions documentation is not kept in S3, the system-s3 module is disabled.
        description: |-
          `moduleStorage.s3.type: System` is set in the `documentation` ModuleConfig, but the `system-s3` module is disabled.
          Until it is enabled, the prepared documentation of module versions is kept in the documentation pod only
          and is prepared again after a restart of the pod.

          Enable the `system-s3` module, or set `moduleStorage.s3.type` to `Disabled` in the `documentation` ModuleConfig.
{{- end }}
