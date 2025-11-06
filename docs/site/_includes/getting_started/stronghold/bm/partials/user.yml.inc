
---
# [<ru>] Данные статического пользователя.
# [<en>] Static user data.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/150-user-authn/cr.html#user
# [<en>] https://deckhouse.io/documentation/v1/modules/150-user-authn/cr.html#user
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  # [<ru>] E-mail пользователя.
  # [<en>] User e-mail.
  email: admin@deckhouse.io
  # [<ru>] Хэш пароля <GENERATED_PASSWORD>.
  # [<ru>] Сгенерируйте свой пароль или используйте этот, но только для тестирования.
  # [<ru>] echo -n '<GENERATED_PASSWORD>' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  # [<en>] Hash of the password <GENERATED_PASSWORD>.
  # [<en>] Generate your own password or use it at your own risk (for testing purposes).
  # [<en>] echo -n '<GENERATED_PASSWORD>' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo
  # [<ru>] Возможно, захотите изменить.
  # [<en>] You might consider changing this.
  password: <GENERATED_PASSWORD_HASH>
---
# [<ru>] Группа пользователей.
# [<en>] User group.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/150-user-authn/cr.html#group
# [<en>] https://deckhouse.io/documentation/v1/modules/150-user-authn/cr.html#group
apiVersion: deckhouse.io/v1alpha1
kind: Group
metadata:
  name: admins
spec:
  name: admins
  members:
    - kind: User
      name: admin
---
# [<ru>] Настройки RBAC и авторизации.
# [<en>] RBAC and authorization settings.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/140-user-authz/cr.html#clusterauthorizationrule
# [<en>] https://deckhouse.io/documentation/v1/modules/140-user-authz/cr.html#clusterauthorizationrule
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
