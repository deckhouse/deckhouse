server:
  addr: "{{ .hostIP }}:5051"
token:
  issuer: "Registry server"
  expiration: 900
  certificate: "/system_registry_pki/token.crt"
  key: "/system_registry_pki/token.key"
users:
  # Password is specified as a BCrypt hash. Use htpasswd -nB USERNAME to generate.
  "pusher":
    password: '\$2y\$05\$d9Ko2sN9YKSgeu9oxfPiAeopkPTaD65RWQiZtaZ2.hnNnLyFObRne'  # pusher
  "puller":
    password: '\$2y\$05\$wVbhDuuhL/TAVj4xMt3lbeCAYWxP1JJNZJdDS/Elk7Ohf7yhT5wNq'  # puller
acl:
  - match: { account: "pusher" }
    actions: [ "*" ]
    comment: "Pusher has full access to everything."
  - match: {account: "/.+/"}  # Match all accounts.
    actions: ["pull"]
    comment: "readonly access to all accounts"
  # Access is denied by default.
