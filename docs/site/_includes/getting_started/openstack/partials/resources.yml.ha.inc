# [<en>] section containing the parameters of instance class for worker nodes
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры инстанс-класса для рабочей ноды
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  # [<en>] name of instance class
  # [<ru>] имя инстанс класса
  name: worker
spec:
  # [<en>] flavor in use for this instance class
  # [<ru>] используемый flavor для данного инстанс-класса
  # [<en>] you might consider changing this
  # [<ru>] возможно захотите изменить
  flavorName: Standard-2-4-50
  rootDiskSize: 30
  # [<en>] VM image in use
  # [<ru>] используемый образ виртуальной машины
  # [<en>] you might consider changing this
  # [<ru>] возможно захотите изменить
  imageName: ubuntu-18-04-cloud-amd64
---
# [<en>] section containing the parameters of worker node group
# [<en>] version of the Deckhouse API
# [<ru>] секция, описывающая параметры нодгруппы worker
# [<ru>] используемая версия API Deckhouse
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  # [<en>] name of node group
  # [<ru>] имя нод-группы
  name: worker
spec:
  nodeType: CloudEphemeral
  # [<en>] parameters for provisioning the cloud-based VMs
  # [<ru>] параметры заказа облачных виртуальных машин
  cloudInstances:
    # [<en>] the reference to the InstanceClass object
    # [<ru>] ссылка на объект InstanceClass
    classReference:
      kind: OpenStackInstanceClass
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
    # [<ru>] возможно захотите изменить
    zones:
    - DP1
  disruptions:
    approvalMode: Automatic
  nodeTemplate:
    # [<en>] similar to the standard metadata.labels field
    # [<ru>] аналогично стандартному полю metadata.labels
    labels:
      node-role.deckhouse.io/worker: ""
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
  # [<ru>] описывает на каких нодах будет находится компонент
  nodeSelector:
    node-role.deckhouse.io/worker: ""
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
  # [<en>] you might consider changing this
  # [<ru>] возможно захотите изменить
  password: <GENERATED_PASSWORD_HASH>
