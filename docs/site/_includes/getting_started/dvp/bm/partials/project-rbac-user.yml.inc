d8 k create -f - <<EOF
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: test-user
spec:
  # [<en>] User e-mail.
  # [<ru>] e-mail пользователя.
  email: test-user@deckhouse.io
  # [<en>] this is a hash of the password <GENERATED_PASSWORD>, generated  now
  # [<en>] generate your own or use it at your own risk (for testing purposes)
  # [<en>] echo -n '<GENERATED_PASSWORD>' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  # [<ru>] это хэш пароля <GENERATED_PASSWORD>, сгенерированного сейчас
  # [<ru>] сгенерируйте свой или используйте этот, но только для тестирования
  # [<ru>] echo -n '<GENERATED_PASSWORD>' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  password: <GENERATED_PASSWORD_HASH>
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  namespace: test-project
  name: admin
subjects:
  - kind: User
    name: test-user@deckhouse.io
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  # [<en>] User role.
  # [<ru>] Роль пользователя.
  name: d8:use:role:admin
  apiGroup: rbac.authorization.k8s.io
EOF
