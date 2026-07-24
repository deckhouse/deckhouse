- name: d8.istio.ca
  rules:
    - alert: D8IstioCustomCAMaterialInvalid
      annotations:
        summary: The Istio custom CA material is invalid but is being served to istiod.
        description: |
          The Istio module is publishing CA material (`{{"{{$labels.source}}"}}`) that failed validation to istiod.

          The module never hard-blocks on module-owned or last-good material (doing so would rotate the mesh root and break mTLS cluster-wide), so it keeps serving this material with only a warning. However, istiod may reject it, which can prevent it from issuing or renewing workload certificates.

          Depending on the `source`:

          - `cacerts` — the `d8-istio/cacerts` Secret (or the CA reused from it) is not a valid CA. Inspect the Secret and restore valid `ca-cert.pem`/`ca-key.pem` (and `root-cert.pem`/`cert-chain.pem` if used).
          - `secretRef:<namespace>/<name>` — the last-good CA that was previously resolved from this `ca.secretRef` has become invalid (for example, its chain expired). Fix the referenced Secret so the next re-resolution (every 5 minutes) succeeds.

          Refer to the [custom CA documentation]({{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}{{- else }}https://deckhouse.io{{- end }}/modules/istio/examples.html).
        plk_markup_format: markdown
        plk_protocol_version: "1"
      expr: |
        d8_istio_ca_material_invalid{} == 1
      for: 5m
      labels:
        severity_level: "4"
        tier: cluster
    - alert: D8IstioCustomCASecretRefUnresolved
      annotations:
        summary: The Istio custom CA `ca.secretRef` cannot be resolved.
        description: |
          The Istio module could not re-resolve the configured `ca.secretRef` (`{{"{{$labels.source}}"}}`) and is keeping the last successfully-resolved CA.

          The mesh is not regressed — istiod keeps running on the exact CA material it already had — but any change made to the source Secret is not being picked up. Common causes:

          - the referenced Secret was deleted or renamed;
          - the referenced Secret was made malformed (not valid PEM, not a CA certificate, or a mismatched key pair);
          - a transient Kubernetes API problem.

          The module re-resolves the Secret every 5 minutes; this alert clears automatically once resolution succeeds. Inspect the referenced Secret and restore valid CA material.

          Refer to the [custom CA documentation]({{ if .Values.global.modules.publicDomainTemplate }}{{ include "helm_lib_module_uri_scheme" . }}://{{ include "helm_lib_module_public_domain" (list . "documentation") }}{{- else }}https://deckhouse.io{{- end }}/modules/istio/examples.html).
        plk_markup_format: markdown
        plk_protocol_version: "1"
      expr: |
        d8_istio_ca_secretref_unresolved{} == 1
      for: 15m
      labels:
        severity_level: "5"
        tier: cluster
