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
  {{ quote .RO.Name }}:
    password: {{ quote .RO.PasswordHash }}
  {{- with .RW }}
  {{ quote .Name }}:
    password: {{ quote .PasswordHash }}
  {{- end }}
  {{- with .MirrorPuller }}
  {{ quote .Name }}:
    password: {{ quote .PasswordHash }}
  {{- end }}
  {{- with .MirrorPusher }}
  {{ quote .Name }}:
    password: {{ quote .PasswordHash }}
  {{- end }}

acl:
  - match: { account: {{ quote .RO.Name }} }
    actions: ["pull"]
    comment: "has readonly access"
  {{- with .RW }}
  - match: { account: {{ quote .Name }} }
    actions: [ "*" ]
    comment: "has full access"
  {{- end }}
  {{- with .MirrorPusher }}
  - match: { account: {{ quote .Name }} }
    actions: [ "*" ]
    comment: "mirrorer pusher"
  {{- end }}
  {{- with .MirrorPuller }}
  - match: { account: {{ quote .Name }}, type: "registry", name: "catalog" }
    actions: ["*"]
    comment: "mirrorer puller catalog"
  - match: { account: {{ quote .Name }} }
    actions: ["pull"]
    comment: "mirrorer puller"
  {{- end }}
  # Access is denied by default.
