{{- if ne (include "documentation_s3_requested" .) "Disabled" }}
- name: d8.documentation.s3-not-ready
  rules:
    - alert: D8DocumentationS3NotReady
      expr: min(docs_builder_ondemand_s3_ready) == 0
      for: 15m
      labels:
        severity_level: "9"
        tier: cluster
        d8_module: documentation
        d8_component: docs-builder
      annotations:
        plk_protocol_version: "1"
        plk_markup_format: markdown
        summary: Module versions documentation is not kept in S3, the S3 storage is not ready.
        description: |-
          The `documentation` ModuleConfig keeps the prepared documentation of module versions in S3 (`moduleStorage.s3.type`), but the S3 storage has not been usable for 15 minutes.
          Until it is, the documentation is kept in the documentation pod only and is prepared again after a restart of the pod.

          With `type: External`, check that the Secret from `moduleStorage.s3.external.secretRef` exists and holds the keys listed on the usage page of the module.
          The reason is in the log of the `builder` container:

          ```bash
          d8 k -n d8-system logs deploy/documentation -c builder | grep -i s3
          ```
{{- end }}
