server:
  addr: "{{ .IpAddress }}:5051"
  certificate: "/system_registry_pki/auth.crt"
  key: "/system_registry_pki/auth.key"
token:
  issuer: "Registry server"
  expiration: 900
  certificate: "/system_registry_pki/token.crt"
  key: "/system_registry_pki/token.key"

users:
  # Password is specified as a BCrypt hash. Use htpasswd -nB USERNAME to generate.
  {{ quote .Registry.UserRw.Name }}:
    password: {{ quote .Registry.UserRw.PasswordHash }}
  {{ quote .Registry.UserRo.Name }}:
    password: {{ quote .Registry.UserRo.PasswordHash }}

acl:
  - match: { account: {{ quote .Registry.UserRw.Name }} }
    actions: [ "*" ]
    comment: "has full access"
  - match: { account: {{ quote .Registry.UserRo.Name }} }
    actions: ["pull"]
    comment: "has readonly access"
  # Access is denied by default.
