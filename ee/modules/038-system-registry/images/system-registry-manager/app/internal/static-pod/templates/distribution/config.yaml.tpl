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
  addr: {{ .IpAddress }}:5001
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

{{- if eq .Registry.RegistryMode "Proxy" }}
proxy:
  remoteurl: "{{ .Registry.UpstreamRegistry.Scheme }}://{{ .Registry.UpstreamRegistry.Host }}"
  username: {{ quote .Registry.UpstreamRegistry.User }}
  password: {{ quote .Registry.UpstreamRegistry.Password }}
  remotepathonly: {{ quote .Registry.UpstreamRegistry.Path }}
  localpathalias: "/system/deckhouse"
  {{- if .Registry.UpstreamRegistry.TTL }}
  ttl: {{ quote .Registry.UpstreamRegistry.TTL }}
  {{- end }}
{{- end }}
auth:
  token:
    realm: "https://{{ .IpAddress }}:5051/auth"
    service: Docker registry
    issuer: Registry server
    rootcertbundle: /system_registry_pki/token.crt
    autoredirect: false
