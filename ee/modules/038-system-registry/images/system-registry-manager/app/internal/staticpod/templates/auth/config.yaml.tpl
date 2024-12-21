server:
  addr: "{{ .Address }}:5051"
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

acl:
  - match: { account: {{ quote .Registry.UserRW.Name }} }
    actions: [ "*" ]
    comment: "has full access"
  - match: { account: {{ quote .Registry.UserRO.Name }} }
    actions: ["pull"]
    comment: "has readonly access"
  # Access is denied by default.
