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
  addr: {{ hostPort .ListenAddress 5001 | quote }}
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
{{- end }}

{{- with .Upstream }}
proxy:
  remoteurl: {{ printf "%s://%s" .Scheme .Host | quote }}
  {{- if .User }}
  username: {{ quote .User }}
  password: {{ quote .Password }}
  {{- end }}
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
    realm: {{ printf "https://%s/auth" (hostPort .ListenAddress 5051) | quote }}
    service: Deckhouse registry
    issuer: Registry server
    rootcertbundle: /pki/token.crt
    autoredirect: true
    proxy:
      url: https://127.0.0.1:5051/auth
      ca: /pki/ca.crt
