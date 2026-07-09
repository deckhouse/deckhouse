apiVersion: v1
kind: Namespace
metadata:
  name: d8-system
  labels:
    heritage: deckhouse
    extended-monitoring.deckhouse.io/enabled: ""
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: deckhouse
  namespace: d8-system
  labels:
    heritage: deckhouse
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: deckhouse
  labels:
    heritage: deckhouse
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: deckhouse
  namespace: d8-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: install-data
  namespace: d8-system
  labels:
    heritage: deckhouse
data:
  version: "virtual-control-plane"
---
apiVersion: v1
kind: Secret
metadata:
  name: deckhouse-registry
  namespace: d8-system
  labels:
    heritage: deckhouse
    app: registry
    name: deckhouse-registry
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: eyJhdXRocyI6eyJyZWdpc3RyeS5kZWNraG91c2UuaW8iOnt9fX0=
  address: cmVnaXN0cnkuZGVja2hvdXNlLmlv
  path: ZGVja2hvdXNlL2Nl
  scheme: aHR0cHM=
  ca: ""
  clusterIsBootstrapped: ZmFsc2U=
  imagesRegistry: cmVnaXN0cnkuZGVja2hvdXNlLmlvL2RlY2tob3VzZS9jZQ==
---
apiVersion: v1
kind: Secret
metadata:
  name: registry-config
  namespace: d8-system
  annotations:
    version: "1"
  labels:
    heritage: deckhouse
    app: registry
    name: registry-deckhouse-config
    type: registry-config
type: registry/config
data:
  mode: VW5tYW5hZ2Vk
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckhouse
  namespace: d8-system
  labels:
    heritage: deckhouse
    app.kubernetes.io/managed-by: Helm
  annotations:
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
spec:
  replicas: 1
  revisionHistoryLimit: 0
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: deckhouse
  template:
    metadata:
      labels:
        app: deckhouse
      annotations:
        kubectl.kubernetes.io/default-container: deckhouse
    spec:
      hostNetwork: true
      dnsPolicy: Default
      serviceAccountName: deckhouse
      automountServiceAccountToken: true
      priorityClassName: system-cluster-critical
      tolerations:
      - operator: Exists
      imagePullSecrets:
      - name: deckhouse-registry
      securityContext:
        runAsUser: 0
        runAsGroup: 0
        runAsNonRoot: false
      volumes:
      - name: tmp
        emptyDir:
          medium: Memory
      - name: kube
        emptyDir:
          medium: Memory
      - name: downloaded
        hostPath:
          path: /var/lib/deckhouse/downloaded
          type: DirectoryOrCreate
      - name: dev-dir
        hostPath:
          path: /dev
          type: Directory
      containers:
      - name: deckhouse
        image: "${IMAGE_DECKHOUSE}"
        imagePullPolicy: IfNotPresent
        command:
        - /usr/bin/deckhouse-controller
        - start
        workingDir: /deckhouse
        ports:
        - name: self
          containerPort: 4222
        - name: custom
          containerPort: 4223
        readinessProbe:
          httpGet:
            path: /readyz
            port: 4222
          initialDelaySeconds: 5
          periodSeconds: 5
          failureThreshold: 120
        env:
        - name: DECKHOUSE_BUNDLE
          value: Minimal
        - name: DECKHOUSE_POD
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: DECKHOUSE_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: LOG_LEVEL
          value: Info
        - name: LOG_TYPE
          value: json
        - name: HELM_HOST
          value: 127.0.0.1:44434
        - name: ADDON_OPERATOR_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: ADDON_OPERATOR_LISTEN_ADDRESS
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        - name: ADDON_OPERATOR_LISTEN_PORT
          value: "4222"
        - name: ADDON_OPERATOR_ADMISSION_SERVER_LISTEN_PORT
          value: "4223"
        - name: ADDON_OPERATOR_CRD_EXTRA_LABELS
          value: heritage=deckhouse
        - name: ADDON_OPERATOR_CONFIG_MAP
          value: deckhouse
        - name: ADDON_OPERATOR_PROMETHEUS_METRICS_PREFIX
          value: deckhouse_
        - name: ADDON_OPERATOR_APPLIED_MODULE_EXTENDERS
          value: EditionEnabled,Static,DynamicallyEnabled,KubeConfig,DeckhouseVersion,KubernetesVersion,Bootstrapped,ScriptEnabled,ModuleDependency
        - name: MODULES_DIR
          value: /deckhouse/modules:/deckhouse/downloaded
        - name: DOWNLOADED_MODULES_DIR
          value: /deckhouse/downloaded
        - name: EXTERNAL_MODULES_DIR
          value: /deckhouse/downloaded
        - name: HELM_HISTORY_MAX
          value: "3"
        - name: KUBE_CLIENT_QPS
          value: "-1"
        - name: KUBE_CLIENT_BURST
          value: "-1"
        - name: OBJECT_PATCHER_KUBE_CLIENT_QPS
          value: "-1"
        - name: OBJECT_PATCHER_KUBE_CLIENT_BURST
          value: "-1"
        - name: HELM_MONITOR_KUBE_CLIENT_QPS
          value: "-1"
        - name: HELM_MONITOR_KUBE_CLIENT_BURST
          value: "-1"
        volumeMounts:
        - name: tmp
          mountPath: /tmp
        - name: tmp
          mountPath: /run
        - name: kube
          mountPath: /.kube
        - name: downloaded
          mountPath: /deckhouse/downloaded
        - name: dev-dir
          mountPath: /dev
        securityContext:
          privileged: true
          readOnlyRootFilesystem: true
          runAsUser: 0
          runAsGroup: 0
          runAsNonRoot: false
