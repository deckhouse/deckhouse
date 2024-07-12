# [<en>] Section containing the parameters of instance class for worker nodes.
# [<en>] hhttps://deckhouse.io/documentation/v1/modules/030-cloud-provider-vcd/cr.html
# [<ru>] Секция, описывающая параметры инстанс-класса для узлов c компонентами, обеспечивающими рабочую нагрузку.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/030-cloud-provider-vcd/cr.html
apiVersion: deckhouse.io/v1
kind: VCDInstanceClass
metadata:
  name: worker
spec:
  rootDiskSizeGb: 50
  sizingPolicy: *!CHANGE_SIZING_POLICY*
  storageProfile: *!CHANGE_STORAGE_PROFILE*
  # [<en>] The name of the image, taking into account the vCloudDirector without catalog path.
  # [<en>] Example: "ubuntu-jammy-22.04".
  # [<ru>] Имя образа, созданного с без учета каталога размещения образа в vCloudDirector.
  # [<ru>] Пример: "ubuntu-jammy-22.04".
  template: *!CHANGE_TEMPLATE_NAME*
---
# [<en>] Section containing the parameters of worker node group.
# [<en>] https://deckhouse.io/documentation/v1/modules/040-node-manager/cr.html#nodegroup
# [<ru>] Секция, описывающая параметры группы узлов c компонентами, обеспечивающими рабочую нагрузку.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/040-node-manager/cr.html#nodegroup
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: VCDInstanceClass
      name: worker
    maxPerZone: 2
    maxSurgePerZone: 0
    maxUnavailablePerZone: 0
    minPerZone: 1
  nodeType: CloudEphemeral
---
# [<en>] Section containing the parameters of NGINX Ingress controller.
# [<en>] https://deckhouse.io/documentation/v1/modules/402-ingress-nginx/cr.html
# [<ru>] Секция, описывающая параметры NGINX Ingress controller.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/402-ingress-nginx/cr.html
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  # [<en>] The way traffic goes to cluster from the outer network.
  # [<ru>] Способ поступления трафика из внешнего мира.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
    realIPHeader: X-Forwarded-For
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
  - operator: Exists
---
# [<en>] RBAC and authorization settings.
# [<en>] https://deckhouse.io/documentation/v1/modules/140-user-authz/cr.html#clusterauthorizationrule
# [<ru>] Настройки RBAC и авторизации.
# [<ru>] https://deckhouse.ru/documentation/v1/modules/140-user-authz/cr.html#clusterauthorizationrule
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: User
    name: admin@deckhouse.io
  accessLevel: SuperAdmin
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
  # [<en>] This is a hash of the password <GENERATED_PASSWORD>, generated when loading the page of the Getting Started.
  # [<en>] Generate your own or use it at your own risk (for testing purposes)
  # [<en>] echo "<GENERATED_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
  # [<ru>] Это хэш пароля <GENERATED_PASSWORD>, сгенерированного при загрузке страницы "Быстрого Старта".
  # [<ru>] Сгенерируйте свой или используйте этот, но только для тестирования
  # [<ru>] echo "<GENERATED_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2 | base64 -w0
  # [<en>] You might consider changing this.
  # [<ru>] Возможно, захотите изменить.
  password: <GENERATED_PASSWORD_HASH>
