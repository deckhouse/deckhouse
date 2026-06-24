{{- /*
  registry.joiningNodeConfig — the bashible.Context (go_lib/registry/models/bashible)
  for new-arch JOINING nodes. containerd resolves registry.d8-system.svc:5001 via,
  in order:
    1. the same-node agent (127.0.0.1:5001) — preferred, but not up yet on a brand-
       new node;
    2. each master's hostNetwork agent (<master-ip>:5001) — the PRE-CNI bootstrap
       path: a node-IP endpoint reachable before the joining node has CNI / CoreDNS
       / kube-proxy, so it can pull its own agent + CNI images. skip_verify (the
       agent serving cert SANs do not cover master node IPs); safe because pulls are
       digest-pinned and the hop is on the cluster network. Auth'd as ReadOnly;
    3. the cache Service (registry-cache.d8-system.svc:5001) — a ClusterIP, usable
       only once kube-proxy/CNI are up.
  Once the node's own agent starts it rewrites registry.d (the .managed-by-agent
  marker) to [127.0.0.1:5001, cache] and these bootstrap mirrors are dropped.
*/ -}}
{{- define "registry.joiningNodeConfig" -}}
{{- $ca := .Values.registry.internal.pki.ca.cert | default "" -}}
{{- $roUser := "" -}}
{{- $roPass := "" -}}
{{- range .Values.registry.internal.pki.users -}}
  {{- if eq .role "ReadOnly" -}}
    {{- $roUser = .name -}}
    {{- $roPass = .password -}}
  {{- end -}}
{{- end -}}
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
{{- range $ip := .Values.registry.internal.bootstrapMasterEndpoints }}
      - host: "{{ $ip }}:5001"
        scheme: "https"
        skipVerify: true
        auth:
          username: {{ $roUser | quote }}
          password: {{ $roPass | quote }}
{{- end }}
{{- if .Values.registry.cache.enabled }}
      - host: "registry-cache.d8-system.svc:5001"
        scheme: "https"
        ca: |
{{ $ca | indent 10 }}
{{- end }}
{{- end -}}
