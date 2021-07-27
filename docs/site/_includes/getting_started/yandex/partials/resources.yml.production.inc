apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: system
    maxPerZone: 2
    minPerZone: 2
  disruptions:
    approvalMode: Automatic
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: frontend
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: system
spec:
  # [<en>] you might consider changing this
  # [<ru>] возможно захотите изменить
  cores: 4
  # [<en>] you might consider changing this
  # [<ru>] возможно захотите изменить
  memory: 8192
  # [<en>] you might consider changing this
  # [<ru>] возможно захотите изменить
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
    minPerZone: 3
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
# [<ru>] возможно захотите изменить
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
  disruptions:
    approvalMode: Automatic
  nodeTemplate:
    labels:
      node-role.deckhouse.io/worker: ""
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: worker
# [<en>] you might consider changing this
# [<ru>] возможно захотите изменить
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
kind: User
metadata:
  name: admin
spec:
  email: admin@example.com
  # [<en>] this is a hash for generated password: <GENERATED_PASSWORD>
  # [<ru>] это хэш сгенерированного пароля: <GENERATED_PASSWORD>
  # [<en>] you might consider changing this
  # [<ru>] возможно захотите изменить
  password: <GENERATED_PASSWORD_HASH>
