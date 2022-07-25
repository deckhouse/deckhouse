# [<en>] section containing the parameters of system node group
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры группы узлов system
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
      kind: YandexInstanceClass
      name: system
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
    - ru-central1-a
    - ru-central1-b
  disruptions:
    approvalMode: Automatic
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: CloudEphemeral
---
# [<en>] section containing the parameters of instance class for system nodes
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c системными компонентами
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  # [<en>] name of instance class
  # [<ru>] имя инстанс класса
  name: system
spec:
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  cores: 4
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  memory: 8192
  # [<en>] you might consider changing this
  # [<ru>] возможно, захотите изменить
  diskSizeGB: 30
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: frontend
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: frontend
    # [<en>] the maximum number of instances for the group in each zone
    # [<ru>] максимальное количество инстансов в зоне
    maxPerZone: 2
    # [<en>] the minimum number of instances for the group in each zone
    # [<ru>] минимальное количество инстансов в зоне
    minPerZone: 1
  disruptions:
    approvalMode: Automatic
  nodeTemplate:
    labels:
      node-role.deckhouse.io/frontend: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: frontend
  nodeType: CloudEphemeral
---
# [<en>] section containing the parameters of instance class for frontend nodes
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c компонентами, принимающими трафик
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  # [<en>] name of instance class
  # [<ru>] имя инстанс класса
  name: frontend
# [<en>] you might consider changing this
# [<ru>] возможно, захотите изменить
spec:
  cores: 2
  memory: 4096
  diskSizeGB: 30
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: worker
    maxPerZone: 1
    minPerZone: 1
    # [<en>] you might consider changing this
    # [<ru>] возможно, захотите изменить
    zones:
    - ru-central1-a
  disruptions:
    approvalMode: Automatic
  nodeType: CloudEphemeral
---
# [<en>] section containing the parameters of instance class for worker nodes
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры инстанс-класса для узлов c компонентами, обеспечивающими рабочую нагрузку
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: worker
# [<en>] you might consider changing this
# [<ru>] возможно, захотите изменить
spec:
  cores: 4
  memory: 8192
  diskSizeGB: 30
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  maxReplicas: 3
  minReplicas: 2
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
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
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
