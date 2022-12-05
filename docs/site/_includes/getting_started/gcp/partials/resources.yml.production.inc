# [<en>] section containing the parameters of instance class for system nodes
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c системными компонентами
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  # [<en>] name of instance class
  # [<ru>] имя инстанс класса
  name: system
spec:
  diskSizeGb: 40
  # [<en>] Machine type in use for this instance class
  # [<ru>] используемый Machine type для данного инстанс-класса
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  machineType: n2-standard-4
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
  # [<en>] parameters for provisioning the cloud-based VMs
  # [<ru>] параметры заказа облачных виртуальных машин
  cloudInstances:
    # [<en>] the reference to the InstanceClass object
    # [<ru>] ссылка на объект InstanceClass
    classReference:
      kind: GCPInstanceClass
      name: system
    # [<en>] the minimum number of instances for the group in each zone
    # [<ru>] минимальное количество инстансов в зоне
    minPerZone: 2
    # [<en>] the maximum number of instances for the group in each zone
    # [<ru>] максимальное количество инстансов в зоне
    maxPerZone: 2
    # [<en>] list of availability zones to create instances in
    # [<ru>] переопределение перечня зон, в которых создаются инстансы
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    zones:
    - europe-west3-a
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
  nodeType: CloudEphemeral
---
# [<en>] section containing the parameters of instance class for frontend nodes
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c компонентами, принимающими трафик
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  # [<en>] name of instance class
  # [<ru>] имя инстанс класса
  name: frontend
# [<en>] you might consider changing this
# [<ru>] возможно, захотите изменить
spec:
  diskSizeGb: 40
  # [<en>] Machine type in use for this instance class
  # [<ru>] используемый Machine type для данного инстанс-класса
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  machineType: n2-standard-4
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: frontend
spec:
  cloudInstances:
    classReference:
      kind: GCPInstanceClass
      name: frontend
    # [<en>] the minimum number of instances for the group in each zone
    # [<ru>] минимальное количество инстансов в зоне
    minPerZone: 2
    # [<en>] the maximum number of instances for the group in each zone
    # [<ru>] максимальное количество инстансов в зоне
    maxPerZone: 3
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    zones:
    - europe-west3-a
  disruptions:
    approvalMode: Automatic
  nodeTemplate:
    # [<en>] similar to the standard metadata.labels field
    # [<ru>] аналогично стандартному полю metadata.labels
    labels:
      node-role.deckhouse.io/frontend: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: frontend
  nodeType: CloudEphemeral
---
# [<en>] section containing the parameters of instance class for worker nodes
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c компонентами, обеспечивающими рабочую нагрузку
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  # [<en>] name of instance class
  # [<ru>] имя инстанс класса
  name: worker
spec:
  diskSizeGb: 40
  # [<en>] Machine type in use for this instance class
  # [<ru>] используемый Machine type для данного инстанс-класса
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  machineType: n2-standard-4
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  # [<en>] parameters for provisioning the cloud-based VMs
  # [<ru>] параметры заказа облачных виртуальных машин
  cloudInstances:
    classReference:
      kind: GCPInstanceClass
      name: worker
    # [<en>] the minimum number of instances for the group in each zone
    # [<ru>] минимальное количество инстансов в зоне
    minPerZone: 1
    # [<en>] the maximum number of instances for the group in each zone
    # [<ru>] максимальное количество инстансов в зоне
    maxPerZone: 1
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    zones:
    - europe-west3-a
  disruptions:
    approvalMode: Automatic
  nodeType: CloudEphemeral
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
  # [<en>] Ingress version to use (use version 1.1 with Kubernetes 1.23+)
  # [<ru>] версия Ingress-контроллера (используйте версию 1.1 с Kubernetes 1.23+)
  controllerVersion: "1.1"
  # [<en>] the way traffic goes to cluster from the outer network
  # [<ru>] способ поступления трафика из внешнего мира
  inlet: LoadBalancer
  # [<en>] describes on which nodes the component will be located
  # [<ru>] описывает, на каких узлах будет находиться компонент
  nodeSelector:
    node-role.kubernetes.io/frontend: ''
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
