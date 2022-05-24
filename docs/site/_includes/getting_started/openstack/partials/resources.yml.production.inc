# [<en>] section containing the parameters of instance class for system nodes
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c системными компонентами
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  # [<en>] name of instance class
  # [<ru>] имя инстанс класса
  name: system
spec:
  # [<en>] flavor in use for this instance class
  # [<ru>] используемый flavor для данного инстанс-класса
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  flavorName: Standard-2-4-50
  rootDiskSize: 30
  # [<en>] VM image in use
  # [<ru>] используемый образ виртуальной машины
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  imageName: ubuntu-18-04-cloud-amd64
---
# [<en>] section containing the parameters of system node group
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры группы узлов c системными компонентами
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  nodeType: CloudEphemeral
  # [<en>] parameters for provisioning the cloud-based VMs
  # [<ru>] параметры заказа облачных виртуальных машин
  cloudInstances:
    # [<en>] the reference to the InstanceClass object
    # [<ru>] ссылка на объект InstanceClass
    classReference:
      kind: OpenStackInstanceClass
      name: system
    # [<en>] the maximum number of instances for the group in each zone
    # [<ru>] максимальное количество инстансов в зоне
    maxPerZone: 2
    # [<en>] the minimum number of instances for the group in each zone
    # [<ru>] минимальное количество инстансов в зоне
    minPerZone: 2
    # [<en>] list of availability zones to create instances in
    # [<ru>] переопределение перечня зон, в которых создаются инстансы
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    zones:
    - DP1
  disruptions:
    approvalMode: Automatic
  nodeTemplate:
    # [<en>] similar to the standard metadata.labels field
    # [<ru>] аналогично стандартному полю metadata.labels
    labels:
      node-role.deckhouse.io/system: ""
    # [<en>] similar to the .spec.taints field of the Node object
    # [<en>] only effect, key, value fields are available
    # [<ru>] аналогично полю .spec.taints из объекта Node
    # [<ru>] доступны только поля effect, key, value
    taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: system
---
# [<en>] section containing the parameters of instance class for frontend nodes
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c компонентами, принимающими трафик
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  # [<en>] name of instance class
  # [<ru>] имя инстанс класса
  name: frontend
spec:
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  flavorName: Standard-2-4-50
  rootDiskSize: 30
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  imageName: ubuntu-18-04-cloud-amd64
---
# [<en>] section containing the parameters of frontend node group
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры группы узлов frontend
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: frontend
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: system
    maxPerZone: 2
    minPerZone: 2
    zones:
    - DP1
  disruptions:
    approvalMode: Automatic
  nodeTemplate:
    labels:
      node-role.deckhouse.io/frontend: ""
    taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
---
# [<en>] section containing the parameters of instance class for worker nodes
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c компонентами, обеспечивающими рабочую нагрузку
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: worker
spec:
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  flavorName: Standard-2-4-50
  rootDiskSize: 30
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  imageName: ubuntu-18-04-cloud-amd64
---
# [<en>] section containing the parameters of worker node group
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры группы узлов worker
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: OpenStackInstanceClass
      name: worker
    maxPerZone: 1
    minPerZone: 1
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    zones:
    - DP1
  disruptions:
    approvalMode: Automatic
---
# [<en>] section containing the parameters of nginx ingress controller
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры nginx ingress controller
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  # [<en>] the name of the Ingress class to use with the Ingress nginx controller
  # [<ru>] имя Ingress-класса для обслуживания Ingress NGINX controller
  ingressClass: nginx
  # [<en>] the way traffic goes to cluster from the outer network
  # [<ru>] способ поступления трафика из внешнего мира
  inlet: LoadBalancer
  # [<en>] describes on which nodes the component will be located
  # [<ru>] описывает, на каких узлах будет находиться компонент
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
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
