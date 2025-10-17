apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: dvp-admin
subjects:
- kind: User
  name: admin@deckhouse.io # для users.deckhouse.io параметр .spec.email
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  # Роль пользователя
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
---
# [<en>] section containing the parameters of the static user
# [<ru>] секция, описывающая параметры статического пользователя
# [<en>] version of the Deckhouse API
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  # [<en>] user e-mail
  # [<ru>] e-mail пользователя
  email: admin@deckhouse.io
  # [<en>] this is a hash of the password <GENERATED_PASSWORD>, generated  now
  # [<en>] generate your own or use it at your own risk (for testing purposes)
  # [<en>] echo -n '<GENERATED_PASSWORD>' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  # [<ru>] это хэш пароля <GENERATED_PASSWORD>, сгенерированного сейчас
  # [<ru>] сгенерируйте свой или используйте этот, но только для тестирования
  # [<ru>] echo -n '<GENERATED_PASSWORD>' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  password: <GENERATED_PASSWORD_HASH>
