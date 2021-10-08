# [<en>] version of the Deckhouse API
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1alpha1
# [<en>] type of the configuration section
# [<ru>] тип секции конфигурации
kind: IngressNginxController
# [<en>] data that helps uniquely identify the object
# [<ru>] данные, которые помогают однозначно идентифицировать объект
metadata:
  # [<en>] object name
  # [<ru>] имя объекта
  name: main
spec:
  # [<en>] the name of the Ingress class to use with the Ingress nginx controller
  # [<ru>] имя Ingress-класса для обслуживания Ingress NGINX controller
  ingressClass: "nginx"
  # [<en>] the way traffic goes to cluster from the outer network
  # [<ru>] способ поступления трафика из внешнего мира
  inlet: "LoadBalancer"
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  # [<ru>] список учётных записей Kubernetes RBAC
  # [<en>] Kubernetes RBAC accounts list
  subjects:
  - kind: User
    name: admin@example.com
  # [<ru>] предустановленный шаблон уровня доступа
  # [<en>] pre-defined access template
  accessLevel: SuperAdmin
  # [<ru>] разрешить пользователю делать kubectl port-forward
  # [<en>] allow user to do kubectl port-forward
  portForwarding: true
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
  email: admin@example.com
  # [<en>] this is a hash for generated password: <GENERATED_PASSWORD>
  # [<ru>] это хэш сгенерированного пароля: <GENERATED_PASSWORD>
  # [<en>] echo "<GENERATED_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2
  # [<ru>] echo "<GENERATED_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  password: <GENERATED_PASSWORD_HASH>
