{{- /*
  registry.joiningNodeConfig — the bashible.Context (go_lib/registry/models/bashible)
  for new-arch JOINING nodes: containerd resolves registry.d8-system.svc:5001 to the
  same-node agent, failing over to the on-master cache while the node's agent starts.
  The agent then takes over registry.d via the ownership marker.
*/ -}}
{{- define "registry.joiningNodeConfig" -}}
{{- $ca := .Values.registry.internal.pki.ca.cert | default "" -}}
registryModuleEnable: true
# mode/version are required by bashible.Config.Validate() (the node-manager
# StateController rejects a config missing them). For new-arch joining nodes the
# value is informational — no Normal-runType bashible step branches on it
# behaviorally (001 keys on proxyEndpoints, 030 on .cri), so "Direct" triggers
# nothing legacy.
mode: "Direct"
version: {{ .Values.registry.internal.pki.hash | default "registry-agent" | quote }}
imagesBase: "registry.d8-system.svc:5001/system/deckhouse"
hosts:
  "registry.d8-system.svc:5001":
    mirrors:
      - host: "127.0.0.1:5001"
        scheme: "https"
        ca: |
{{ $ca | indent 10 }}
{{- if .Values.registry.cache.enabled }}
      - host: "registry-cache.d8-system.svc:5001"
        scheme: "https"
        ca: |
{{ $ca | indent 10 }}
{{- end }}
{{- end -}}
