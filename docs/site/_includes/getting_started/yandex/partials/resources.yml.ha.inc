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
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: nlb
  nodeSelector:
    node-role.deckhouse.io/worker: ""
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
  # [<ru>] возможно, захотите изменить
  password: <GENERATED_PASSWORD_HASH>
