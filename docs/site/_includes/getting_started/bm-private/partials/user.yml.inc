# [<en>] RBAC and authorization settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/140-user-authz/cr.html#clusterauthorizationrule
# [<ru>] Настройки RBAC и авторизации.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/140-user-authz/cr.html#clusterauthorizationrule
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  # [<ru>] Список учётных записей Kubernetes RBAC.
  # [<en>] Kubernetes RBAC accounts list.
  subjects:
  - kind: User
    name: admin@deckhouse.io
  # [<ru>] Предустановленный шаблон уровня доступа.
  # [<en>] Pre-defined access template.
  accessLevel: SuperAdmin
  # [<ru>] Разрешить пользователю делать kubectl port-forward.
  # [<en>] Allow user to do kubectl port-forward.
  portForwarding: true
---
# [<en>] Parameters of the static user.
# [<en>] https://deckhouse.io/documentation/v1/modules/150-user-authn/cr.html#user
# [<ru>] Данные статического пользователя.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/150-user-authn/cr.html#user
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  # [<en>] User e-mail.
  # [<ru>] E-mail пользователя.
  email: admin@deckhouse.io
  # [<en>] This is a hash of the newly generated <GENERATED_PASSWORD> password.
  # [<en>] Generate your own or use it at your own risk (for testing purposes):
  # [<en>] echo "<GENERATED_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2
  # [<ru>] Это хэш пароля <GENERATED_PASSWORD>, сгенерированного сейчас.
  # [<ru>] Сгенерируйте свой или используйте этот, но только для тестирования:
  # [<ru>] echo "<GENERATED_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  password: <GENERATED_PASSWORD_HASH>
