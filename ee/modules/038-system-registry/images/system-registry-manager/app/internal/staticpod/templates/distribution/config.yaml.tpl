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
  addr: {{ .Address }}:5001
  prefix: /
  secret: {{ quote .Registry.HttpSecret }}
  debug:
    addr: "127.0.0.1:5002"
    prometheus:
      enabled: true
      path: /metrics
  tls:
    certificate: /system_registry_pki/distribution.crt
    key: /system_registry_pki/distribution.key

{{- if eq .Registry.Mode "Proxy" }}
proxy:
  remoteurl: "{{ .Registry.Upstream.Scheme }}://{{ .Registry.Upstream.Host }}"
  username: {{ quote .Registry.Upstream.User }}
  password: {{ quote .Registry.Upstream.Password }}
  remotepathonly: {{ quote .Registry.Upstream.Path }}
  localpathalias: "/system/deckhouse"
  {{- if .Registry.Upstream.TTL }}
  ttl: {{ quote .Registry.Upstream.TTL }}
  {{- end }}
{{- end }}
auth:
  token:
    realm: "https://{{ .Address }}:5051/auth"
    service: Docker registry
    issuer: Registry server
    rootcertbundle: /system_registry_pki/token.crt
    autoredirect: false