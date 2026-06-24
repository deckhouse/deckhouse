{{- /*
  registry.cache.ingressEnabled — truthy ("true") when the cache should be published
  via Ingress for d8 mirror push. Orchestrator-free gate shared by the cache config
  (realip mTLS) and the ingress resources.
*/ -}}
{{- define "registry.cache.ingressEnabled" -}}
{{- if and .Values.registry.cache.enabled .Values.registry.cache.publish .Values.global.modules.publicDomainTemplate (.Values.global.enabledModules | has "cert-manager") -}}
true
{{- end -}}
{{- end -}}

{{- /*
  registry.cache.distributionConfig — renders docker-distribution config.yaml.
  Input dict: { pki <internal.pki>, cache <internal.cache>, ingress <string> }
*/ -}}
{{- define "registry.cache.distributionConfig" }}
version: 0.1
log:
  level: info
storage:
  filesystem:
    rootdirectory: /data
  delete:
    enabled: true
  redirect:
    disable: true
http:
  addr: :5001
  prefix: /
  secret: {{ .pki.httpSecret | quote }}
  debug:
    addr: "127.0.0.1:5002"
    prometheus:
      enabled: true
      path: /metrics
  tls:
    certificate: /pki/distribution.crt
    key: /pki/distribution.key
{{- if .ingress }}
  realip:
    enabled: true
    clientcert:
      ca: /pki/ingress-client-ca.crt
{{- end }}
{{- with .cache.upstream }}
proxy:
  remoteurl: "{{ .scheme }}://{{ .host }}"
  {{- if .username }}
  username: {{ .username | quote }}
  password: {{ .password | quote }}
  {{- end }}
  remotepathonly: {{ .path | quote }}
  localpathalias: "/system/deckhouse"
  {{- if .hasCA }}
  ca: /pki/upstream-registry-ca.crt
  {{- end }}
  {{- with .ttl }}
  ttl: {{ . | quote }}
  {{- end }}
{{- end }}
auth:
  token:
    # realm points at distribution (:5001), not the auth server (:5051): the
    # auth server is not separately exposed in the Deployment/Service model, so
    # distribution serves /auth and proxies token requests to 127.0.0.1:5051
    # (proxy block below). Token-flow reachability via the Service is verified
    # in integration, not at render time.
    realm: "https://registry-cache.d8-system.svc:5001/auth"
    service: Deckhouse registry
    issuer: Registry server
    rootcertbundle: /pki/token.crt
    autoredirect: true
    proxy:
      url: https://127.0.0.1:5051/auth
      ca: /pki/ca.crt
{{- end }}

{{- /*
  registry.cache.authConfig — renders docker-auth config.yaml.
  Input dict: { pki <internal.pki> }
*/ -}}
{{- define "registry.cache.authConfig" }}
server:
  # Bind all interfaces (not only pod-loopback): distribution still reaches auth
  # via 127.0.0.1:5051, but the kubelet probe must dial the pod IP. kubelet runs
  # probes from the node netns, where 127.0.0.1:5051 is the registry-agent (HTTP),
  # not this container — so a host:127.0.0.1 probe hit the agent and crash-looped
  # auth. Binding :5051 lets the probe reach auth on the pod IP.
  addr: ":5051"
  real_ip_header: "X-Forwarded-For"
  certificate: "/pki/auth.crt"
  key: "/pki/auth.key"
token:
  issuer: "Registry server"
  expiration: 900
  certificate: "/pki/token.crt"
  key: "/pki/token.key"
users:
{{- range .pki.users }}
  {{ .name | quote }}:
    password: {{ .passwordHash | quote }}
{{- end }}
acl:
{{- range .pki.users }}
  - match: { account: {{ .name | quote }} }
    actions: {{ if eq .role "ReadWrite" }}["*"]{{ else }}["pull"]{{ end }}
{{- end }}
{{- end }}
