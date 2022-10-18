---
# [<en>] section containing the parameters of instance class for worker nodes
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c компонентами, обеспечивающими рабочую нагрузку
# [<en>] version of the Deckhouse API
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: VsphereInstanceClass
metadata:
  # [<en>] name of instance class
  # [<ru>] имя инстанс класса
  name: worker
spec:
  numCPUs: 8
  memory: 16384
  # [<en>] VM disk size
  # [<ru>] размер диска для виртуальной машины
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  rootDiskSize: 70
  template: *!CHANGE_TEMPLATE_NAME*
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
      kind: VsphereInstanceClass
      name: worker
    # [<en>] the maximum number of instances for the group in each zone
    # [<ru>] максимальное количество инстансов в зоне
    maxPerZone: 1
    # [<en>] the minimum number of instances for the group in each zone
    # [<ru>] минимальное количество инстансов в зоне
    minPerZone: 1
    # [<en>] list of availability zones to create instances in
    # [<ru>] переопределение перечня зон, в которых создаются инстансы
    zones:
    - *!CHANGE_ZONE_TAG_NAME*
    - *!CHANGE_ANOTHER_ZONE_TAG_NAME*
  nodeType: CloudEphemeral
---
# [<en>] section containing the parameters of instance class for system nodes
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c системными компонентами
# [<en>] version of the Deckhouse API
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: VsphereInstanceClass
metadata:
  # [<en>] name of instance class
  # [<ru>] имя инстанс класса
  name: system
spec:
  numCPUs: 8
  memory: 16384
  # [<en>] VM disk size
  # [<ru>] размер диска для виртуальной машины
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  rootDiskSize: 100
  template: *!CHANGE_TEMPLATE_NAME*
---
# [<en>] section containing the parameters of system node group
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры группы узлов c системными компонентами
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  # [<en>] name of node group
  # [<ru>] имя группы узлов
  name: system
spec:
  # [<en>] parameters for provisioning the cloud-based VMs
  # [<ru>] параметры заказа облачных виртуальных машин
  cloudInstances:
    # [<en>] the reference to the InstanceClass object
    # [<ru>] ссылка на объект InstanceClass
    classReference:
      kind: VsphereInstanceClass
      name: system
    # [<en>] the maximum number of instances for the group in each zone
    # [<ru>] максимальное количество инстансов в зоне
    maxPerZone: 2
    # [<en>] the minimum number of instances for the group in each zone
    # [<ru>] минимальное количество инстансов в зоне
    minPerZone: 1
    # [<en>] list of availability zones to create instances in
    # [<ru>] переопределение перечня зон, в которых создаются инстансы
    # [<ru>] возможно, захотите изменить
    # [<en>] you might consider changing this
    zones:
    - *!CHANGE_ZONE_TAG_NAME*
    - *!CHANGE_ANOTHER_ZONE_TAG_NAME*
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
  nodeType: CloudEphemeral
---
# [<en>] section containing the parameters of instance class for frontend nodes
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c компонентами, принимающими трафик
# [<en>] version of the Deckhouse API
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: VsphereInstanceClass
metadata:
  # [<en>] name of instance class
  # [<ru>] имя инстанс класса
  name: frontend
spec:
  numCPUs: 4
  memory: 8192
  # [<en>] VM disk size
  # [<ru>] размер диска для виртуальной машины
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  rootDiskSize: 50
  template: *!CHANGE_TEMPLATE_NAME*
---
# [<en>] section containing the parameters of frontend node group
# [<ru>] секция, описывающая параметры группы узлов c компонентами, принимающими трафик
# [<en>] version of the Deckhouse API
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  # [<en>] name of node group
  # [<ru>] имя группы узлов
  name: frontend
spec:
  # [<en>] parameters for provisioning the cloud-based VMs
  # [<ru>] параметры заказа облачных виртуальных машин
  cloudInstances:
    # [<en>] the reference to the InstanceClass object
    # [<ru>] ссылка на объект InstanceClass
    classReference:
      kind: VsphereInstanceClass
      name: frontend
    # [<en>] the maximum number of instances for the group in each zone
    # [<ru>] максимальное количество инстансов в зоне
    maxPerZone: 3
    # [<en>] the minimum number of instances for the group in each zone
    # [<ru>] минимальное количество инстансов в зоне
    minPerZone: 2
    # [<en>] list of availability zones to create instances in
    # [<ru>] переопределение перечня зон, в которых создаются инстансы
    # [<ru>] возможно, захотите изменить
    # [<en>] you might consider changing this
    zones:
    - *!CHANGE_ZONE_TAG_NAME*
    - *!CHANGE_ANOTHER_ZONE_TAG_NAME*
  nodeTemplate:
    # [<en>] similar to the standard metadata.labels field
    # [<ru>] аналогично стандартному полю metadata.labels
    labels:
      node-role.deckhouse.io/frontend: ""
    # [<en>] similar to the .spec.taints field of the Node object
    # [<en>] only effect, key, value fields are available
    # [<ru>] аналогично полю .spec.taints из объекта Node
    # [<ru>] доступны только поля effect, key, value
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: frontend
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
  # [<en>] Ingress version to use (use version 1.1 with Kubernetes 1.23+)
  # [<ru>] версия Ingress-контроллера (используйте версию 1.1 с Kubernetes 1.23+)
  controllerVersion: "1.1"
  # [<en>] the way traffic goes to cluster from the outer network
  # [<ru>] способ поступления трафика из внешнего мира
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
    realIPHeader: X-Forwarded-For
  nodeSelector:
    node-role.kubernetes.io/frontend: ""
  tolerations:
  - operator: Exists
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
