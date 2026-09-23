apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ${CPN_NAME}-kube-scheduler
  namespace: ${NAMESPACE}
  labels:
    app: kube-scheduler
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
    control-plane.deckhouse.io/cpn: ${CPN_NAME}
spec:
  serviceName: ${CPN_NAME}-kube-scheduler
  replicas: 1
  selector:
    matchLabels:
      app: kube-scheduler
      control-plane.deckhouse.io/cpn: ${CPN_NAME}
  template:
    metadata:
      labels:
        app: kube-scheduler
        control-plane.deckhouse.io/vcp: ${VCP_NAME}
        control-plane.deckhouse.io/cpn: ${CPN_NAME}
    spec:
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      # Replicas live in separate single-replica StatefulSets, one per ControlPlaneNode, so spreading
      # them keys off the VCP-wide label. With a single ControlPlaneNode only one pod matches.
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                app: kube-scheduler
                control-plane.deckhouse.io/vcp: ${VCP_NAME}
            topologyKey: kubernetes.io/hostname
      nodeSelector: ${VCP_NODE_SELECTOR}
      tolerations: ${VCP_TOLERATIONS}
      containers:
      - name: kube-scheduler
        image: ${IMAGE_KUBE_SCHEDULER}
        command:
        - kube-scheduler
        - --kubeconfig=/kubeconfig/scheduler.conf
        - --authentication-kubeconfig=/kubeconfig/scheduler.conf
        - --authorization-kubeconfig=/kubeconfig/scheduler.conf
        - --leader-elect=true
        ports:
        - {containerPort: 10259, name: https-metrics, protocol: TCP}
        volumeMounts:
        - {name: pki, mountPath: /pki, readOnly: true}
        - {name: kubeconfig, mountPath: /kubeconfig, readOnly: true}
        resources:
          requests: {cpu: 100m, memory: 128Mi}
      volumes:
      - name: pki
        secret:
          secretName: ${PKI_SECRET_NAME}
      - name: kubeconfig
        secret:
          secretName: ${KUBECONFIG_SECRET_NAME}
