apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: system
    maxPerZone: 1
    minPerZone: 1
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
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
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
  diskSizeGb: 30
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
    maxPerZone: 2
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
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: frontend
# [<en>] you might consider changing this
# [<ru>] возможно, захотите изменить
spec:
  cores: 2
  memory: 4096
  diskSizeGb: 30
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
    - ru-central1-c
  disruptions:
    approvalMode: Automatic
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: worker
# [<en>] you might consider changing this
# [<ru>] возможно, захотите изменить
spec:
  cores: 4
  memory: 8192
  diskSizeGb: 30
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
