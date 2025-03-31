{{- with .Registry -}}

server:
  addr: "127.0.0.1:5051"
  real_ip_header: "X-Forwarded-For"
  certificate: "/system_registry_pki/auth.crt"
  key: "/system_registry_pki/auth.key"
token:
  issuer: "Registry server"
  expiration: 900
  certificate: "/system_registry_pki/token.crt"
  key: "/system_registry_pki/token.key"

users:
  # Password is specified as a BCrypt hash. Use htpasswd -nB USERNAME to generate.
  {{ quote .UserRW.Name }}:
    password: {{ quote .UserRW.PasswordHash }}
  {{ quote .UserRO.Name }}:
    password: {{ quote .UserRO.PasswordHash }}
  {{- with .Mirrorer }}
  {{ quote .UserPuller.Name }}:
    password: {{ quote .UserPuller.PasswordHash }}
  {{ quote .UserPusher.Name }}:
    password: {{ quote .UserPusher.PasswordHash }}
  {{- end }}

acl:
  - match: { account: {{ quote .UserRW.Name }} }
    actions: [ "*" ]
    comment: "has full access"
  - match: { account: {{ quote .UserRO.Name }} }
    actions: ["pull"]
    comment: "has readonly access"
  {{- with .Mirrorer }}
  - match: { account: {{ quote .UserPusher.Name }} }
    actions: [ "*" ]
    comment: "mirrorer pusher"
  - match: { account: {{ quote .UserPuller.Name }}, type: "registry", name: "catalog" }
    actions: ["*"]
    comment: "mirrorer puller catalog"
  - match: { account: {{ quote .UserPuller.Name }} }
    actions: ["pull"]
    comment: "mirrorer puller"
  {{- end }}
  # Access is denied by default.

{{- end }}
