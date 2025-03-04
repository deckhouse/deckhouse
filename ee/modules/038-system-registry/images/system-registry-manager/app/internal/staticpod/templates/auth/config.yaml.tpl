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
  {{ quote .Registry.UserRW.Name }}:
    password: {{ quote .Registry.UserRW.PasswordHash }}
  {{ quote .Registry.UserRO.Name }}:
    password: {{ quote .Registry.UserRO.PasswordHash }}
  {{- if eq .Registry.Mode "Detached" }}
  {{ quote .Mirrorer.UserPuller.Name }}:
    password: {{ quote .Mirrorer.UserPuller.PasswordHash }}
  {{ quote .Mirrorer.UserPusher.Name }}:
    password: {{ quote .Mirrorer.UserPusher.PasswordHash }}
  {{- end }}

acl:
  - match: { account: {{ quote .Registry.UserRW.Name }} }
    actions: [ "*" ]
    comment: "has full access"
  - match: { account: {{ quote .Registry.UserRO.Name }} }
    actions: ["pull"]
    comment: "has readonly access"
  {{- if eq .Registry.Mode "Detached" }}
  - match: { account: {{ quote .Mirrorer.UserPusher.Name }} }
    actions: [ "*" ]
    comment: "mirrorer pusher"
  - match: { account: {{ quote .Mirrorer.UserPuller.Name }}, type: "registry", name: "catalog" }
    actions: ["*"]
    comment: "mirrorer puller catalog"
  - match: { account: {{ quote .Mirrorer.UserPuller.Name }} }
    actions: ["pull"]
    comment: "mirrorer puller"
  {{- end }}
  # Access is denied by default.
