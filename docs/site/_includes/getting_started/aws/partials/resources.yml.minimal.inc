# [<en>] section containing the parameters of instance class for worker nodes
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c компонентами, обеспечивающими рабочую нагрузку
# [<en>] version of the Deckhouse API
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: AWSInstanceClass
metadata:
  # [<en>] name of instance class
  # [<ru>] имя инстанс класса
  name: worker
spec:
  # [<en>] VM disk size
  # [<ru>] размер диска для виртуальной машины
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  diskSizeGb: 30
  # [<en>] VM disk type to use
  # [<ru>] используемый тип диска для виртуальной машины
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  diskType: gp3
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  instanceType: c5.xlarge
---
# [<en>] section containing the parameters of worker node group
# [<ru>] секция, описывающая параметры группы узлов c компонентами, обеспечивающими рабочую нагрузку
# [<en>] version of the Deckhouse API
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  # [<en>] name of node group
  # [<ru>] имя группы узлов
  name: worker
spec:
  # [<en>] parameters for provisioning the cloud-based VMs
  # [<ru>] параметры заказа облачных виртуальных машин
  cloudInstances:
    # [<en>] the reference to the InstanceClass object
    # [<ru>] ссылка на объект InstanceClass
    classReference:
      kind: AWSInstanceClass
      name: worker
    # [<en>] the maximum number of instances for the group in each zone
    # [<ru>] максимальное количество инстансов в зоне
    maxPerZone: 1
    # [<en>] the minimum number of instances for the group in each zone
    # [<ru>] минимальное количество инстансов в зоне
    minPerZone: 1
    # [<en>] list of availability zones to create instances in
    # [<ru>] переопределение перечня зон, в которых создаются инстансы
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    zones:
      - eu-central-1a
  nodeType: CloudEphemeral
---
# [<en>] section containing the parameters of nginx ingress controller
# [<ru>] секция, описывающая параметры nginx ingress controller
# [<en>] version of the Deckhouse API
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  # [<en>] the name of the Ingress class to use with the Ingress nginx controller
  # [<ru>] имя Ingress-класса для использования Ingress Nginx controller
  ingressClass: nginx
  # [<en>] the way traffic goes to cluster from the outer network
  # [<ru>] способ поступления трафика из внешнего мира
  inlet: LoadBalancer
  loadBalancer:
    # [<en>] annotations to assign to the service for flexible configuration of the load balancer
    # [<ru>] аннотации, которые будут проставлены сервису для гибкой настройки балансировщика
    annotations:
      # [<en>] provision Network Load Balancer for the Ingress Controller Service
      # [<ru>] использовать Network Load Balancer для сервиса Ingress контроллера
      service.beta.kubernetes.io/aws-load-balancer-type: nlb
  # [<en>] describes on which nodes the component will be located. Label node.deckhouse.io/group: <NAME_GROUP_NAME> is set automatically.
  # [<ru>] описывает, на каких узлах будет находиться компонент. Лейбл node.deckhouse.io/group: <NODE_GROUP_NAME> устанавливается автоматически.
  nodeSelector:
    node.deckhouse.io/group: worker
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
  # [<en>] this is a hash of the password <GENERATED_PASSWORD>, generated  now
  # [<en>] generate your own or use it at your own risk (for testing purposes)
  # [<en>] echo "<GENERATED_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2
  # [<ru>] это хэш пароля <GENERATED_PASSWORD>, сгенерированного сейчас
  # [<ru>] сгенерируйте свой или используйте этот, но только для тестирования
  # [<ru>] echo "<GENERATED_PASSWORD>" | htpasswd -BinC 10 "" | cut -d: -f2
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  password: <GENERATED_PASSWORD_HASH>
