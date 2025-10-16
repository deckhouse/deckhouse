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
  addr: {{ .ListenAddress }}:5001
  prefix: /
  secret: {{ quote .HTTPSecret }}
  debug:
    addr: "127.0.0.1:5002"
    prometheus:
      enabled: true
      path: /metrics
  tls:
    certificate: /pki/distribution.crt
    key: /pki/distribution.key
{{- if .Ingress }}
  realip:
    enabled: true
    clientcert:
      ca: /pki/ingress-client-ca.crt
      cn: kube-rbac-proxy-ca-key-pair
{{- end }}

{{- with .Upstream }}
proxy:
  remoteurl: "{{ .Scheme }}://{{ .Host }}"
  username: {{ quote .User }}
  password: {{ quote .Password }}
  remotepathonly: {{ quote .Path }}
  localpathalias: "/system/deckhouse"
  {{- if .CA }}
  ca: /pki/upstream-registry-ca.crt
  {{- end }}
  {{- with .TTL }}
  ttl: {{ quote . }}
  {{- end }}
{{- end }}
auth:
  token:
    realm: "https://{{ .ListenAddress }}:5051/auth"
    service: Deckhouse registry
    issuer: Registry server
    rootcertbundle: /pki/token.crt
    autoredirect: true
    proxy:
      url: https://127.0.0.1:5051/auth
      ca: /pki/ca.crt
