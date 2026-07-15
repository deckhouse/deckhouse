apiVersion: v1
kind: ServiceAccount
metadata:
  name: konnectivity-agent
  namespace: kube-system
  labels:
    heritage: deckhouse
    k8s-app: konnectivity-agent
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: konnectivity-agent
  namespace: kube-system
  labels:
    heritage: deckhouse
    k8s-app: konnectivity-agent
spec:
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      k8s-app: konnectivity-agent
  updateStrategy:
    rollingUpdate:
      maxUnavailable: 1
    type: RollingUpdate
  template:
    metadata:
      labels:
        k8s-app: konnectivity-agent
    spec:
      serviceAccountName: konnectivity-agent
      priorityClassName: system-cluster-critical
      hostNetwork: true
      tolerations:
      - operator: Exists
      containers:
      - name: konnectivity-agent
        image: ${IMAGE_KONNECTIVITY_AGENT}
        command:
        - /proxy-agent
        args:
        - --logtostderr=true
        - --ca-cert=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        - --proxy-server-host=${VCP_KONN_HOST}
        - --proxy-server-port=443
        - --admin-server-port=8133
        - --health-server-port=8134
        - --service-account-token-path=/var/run/secrets/tokens/konnectivity-agent-token
        livenessProbe:
          httpGet:
            port: 8134
            path: /healthz
          initialDelaySeconds: 15
        volumeMounts:
        - name: konnectivity-agent-token
          mountPath: /var/run/secrets/tokens
      volumes:
      - name: konnectivity-agent-token
        projected:
          sources:
          - serviceAccountToken:
              path: konnectivity-agent-token
              audience: system:konnectivity-server
