apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  # [<ru>] Список учётных записей Kubernetes RBAC.
  # [<en>] Kubernetes RBAC accounts list.
  subjects:
  - kind: User
    name: admin@example.com
  # [<ru>] Предустановленный шаблон уровня доступа.
  # [<en>] Pre-defined access template.
  accessLevel: SuperAdmin
  # [<ru>] Разрешить пользователю делать kubectl port-forward.
  # [<en>] Allow user to do kubectl port-forward.
  portForwarding: true
---
# [<en>] Section containing the parameters of the static user.
# [<ru>] Секция, описывающая параметры статического пользователя.
# [<en>] Version of the Deckhouse API.
# [<ru>] Используемая версия API Deckhouse.
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  # [<en>] User e-mail.
  # [<ru>] E-mail пользователя.
  email: admin@example.com
  # [<en>] This is a hash of the password <GENERATED_PASSWORD>, generated  now.
  # [<en>] Generate your own or use it at your own risk (for testing purposes):
  # [<en>] echo "<GENERATED_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2
  # [<ru>] Это хэш пароля <GENERATED_PASSWORD>, сгенерированного сейчас.
  # [<ru>] Сгенерируйте свой или используйте этот, но только для тестирования:
  # [<ru>] echo "<GENERATED_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  password: <GENERATED_PASSWORD_HASH>
