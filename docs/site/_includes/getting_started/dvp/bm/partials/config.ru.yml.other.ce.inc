# [<en>] Node group for static worker nodes.
# [<en>] https://deckhouse.io/modules/node-manager/cr.html#nodegroup
# [<ru>] Группа узлов для статических worker-узлов.
# [<ru>] https://deckhouse.ru/modules/node-manager/cr.html#nodegroup
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  staticInstances:
    count: 1
    labelSelector:
      matchLabels:
        role: worker
---
# [<en>] SSH credentials for connecting to static nodes.
# [<ru>] SSH-учётные данные для подключения к статическим узлам.
apiVersion: deckhouse.io/v1alpha1
kind: SSHCredentials
metadata:
  name: caps
spec:
  user: caps
  privateSSHKey: "`cat /dev/shm/caps-id | base64 -w0`"
---
# [<en>] Static worker node.
# [<ru>] Статический worker-узел.
apiVersion: deckhouse.io/v1alpha1
kind: StaticInstance
metadata:
  name: dvp-worker
  labels:
    role: worker
spec:
  address: <WORKER_NODE_IP>
  credentialsRef:
    kind: SSHCredentials
    name: caps
---
# [<en>] csi-nfs module settings.
# [<en>] https://deckhouse.io/modules/csi-nfs/configuration.html
# [<ru>] Настройки модуля csi-nfs.
# [<ru>] https://deckhouse.ru/modules/csi-nfs/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-nfs
spec:
  enabled: true
  version: 1
---
# [<en>] NFS StorageClass for VM disks and cluster component data.
# [<en>] https://deckhouse.io/modules/csi-nfs/cr.html#nfsstorageclass
# [<ru>] NFS StorageClass для дисков ВМ и данных компонентов кластера.
# [<ru>] https://deckhouse.ru/modules/csi-nfs/cr.html#nfsstorageclass
apiVersion: storage.deckhouse.io/v1alpha1
kind: NFSStorageClass
metadata:
  name: nfs-storage-class
spec:
  connection:
    host: <NFS_HOST>
    share: <NFS_SHARE>
    nfsVersion: "4.1"
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
  workloadNodes:
    nodeSelector:
      matchLabels:
        storage: "true"
---
# [<en>] virtualization module settings.
# [<en>] https://deckhouse.io/modules/virtualization/configuration.html
# [<ru>] Настройки модуля virtualization.
# [<ru>] https://deckhouse.ru/modules/virtualization/configuration.html
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  settings:
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 50G
        type: PersistentVolumeClaim
    # [<en>] Subnets from which IP addresses are assigned to virtual machines.
    # [<en>] These subnets **must not** overlap with node, service, or pod subnets.
    # [<ru>] Подсети, из которых назначаются IP-адреса виртуальным машинам.
    # [<ru>] Эти подсети **не должны** пересекаться с подсетями узлов, сервисов и подов.
    virtualMachineCIDRs:
    - <VIRTUAL_MACHINE_CIDRS>
  version: 1
---
# [<en>] Ingress NGINX Controller parameters.
# [<en>] https://deckhouse.io/modules/ingress-nginx/cr.html#ingressnginxcontroller
# [<ru>] Параметры контроллера Ingress NGINX.
# [<ru>] https://deckhouse.ru/modules/ingress-nginx/cr.html#ingressnginxcontroller
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  # [<en>] How traffic from the external network reaches the cluster.
  # [<ru>] Способ поступления трафика из внешнего мира.
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  # [<en>] Nodes where the Ingress controller will run. You might consider changing this.
  # [<ru>] Узлы, на которых будет находиться Ingress-контроллер. Возможно, захотите изменить.
  nodeSelector:
    node-role.kubernetes.io/control-plane: ""
  tolerations:
  - effect: NoSchedule
    key: node-role.kubernetes.io/control-plane
    operator: Exists
---
# [<en>] Project for isolating user workloads (multitenancy).
# [<en>] https://deckhouse.io/modules/multitenancy-manager/cr.html#project
# [<ru>] Проект для изоляции пользовательской нагрузки (мультитенантность).
# [<ru>] https://deckhouse.ru/modules/multitenancy-manager/cr.html#project
apiVersion: deckhouse.io/v1alpha2
kind: Project
metadata:
  name: test-project
spec:
  description: test-project
  projectTemplateName: default
  parameters:
    # [<en>] Project resource quotas.
    # [<ru>] Квоты ресурсов проекта.
    resourceQuota:
      requests:
        cpu: 20
        memory: 20Gi
      limits:
        cpu: 20
        memory: 20Gi
    # [<en>] Project administrators.
    # [<ru>] Администраторы проекта.
    administrators:
      - subject: User
        name: <USER_NAME>@deckhouse.io
---
# [<en>] User account for accessing the cluster (Dex / user-authn).
# [<en>] https://deckhouse.io/modules/user-authn/cr.html#user
# [<ru>] Учётная запись пользователя для доступа к кластеру (Dex / user-authn).
# [<ru>] https://deckhouse.ru/modules/user-authn/cr.html#user
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: <USER_NAME>
spec:
  # [<en>] User e-mail. Must match the value in Project.administrators and RoleBinding.subjects.
  # [<ru>] E-mail пользователя. Должен совпадать со значением в Project.administrators и RoleBinding.subjects.
  email: <USER_NAME>@deckhouse.io
  # [<en>] bcrypt hash of the password. Example for password '7sxgcc2tqn': echo -n '7sxgcc2tqn' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo. Generate your own or use this one for testing only.
  # [<ru>] Хэш пароля (bcrypt). Пример для пароля '7sxgcc2tqn': echo -n '7sxgcc2tqn' | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n' | base64 -w0; echo. Сгенерируйте свой или используйте этот только для тестирования.
  password: 'JDJhJDEwJE5yTlMuNWlLQndyRi9ncTEwWnpDZmVwdHJrTUFxa0JRTnFFbDl5U1lCV1JHMC5STTNvNmVT'
---
# [<en>] Binds the ClusterRole to the user in the project namespace (grants admin rights within test-project).
# [<en>] https://deckhouse.io/modules/user-authn/cr.html#user
# [<ru>] Связывает ClusterRole с пользователем в пространстве имён проекта (права администратора в пределах test-project).
# [<ru>] https://deckhouse.ru/modules/user-authn/cr.html#user
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  namespace: test-project
  name: admin
subjects:
  - kind: User
    name: <USER_NAME>@deckhouse.io
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  # [<en>] Role granting full access within the namespace (ResourceQuota, ServiceAccount, Role, RoleBinding, NetworkPolicy).
  # [<ru>] Роль с полным доступом в пределах пространства имён (ResourceQuota, ServiceAccount, Role, RoleBinding, NetworkPolicy).
  name: d8:use:role:admin
  apiGroup: rbac.authorization.k8s.io