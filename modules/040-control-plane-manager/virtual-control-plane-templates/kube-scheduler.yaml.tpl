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
      containers:
      - name: kube-scheduler
        image: ${IMAGE_KUBE_SCHEDULER}
        command:
        - kube-scheduler
        - --kubeconfig=/kubeconfig/scheduler.conf
        - --authentication-kubeconfig=/kubeconfig/scheduler.conf
        - --authorization-kubeconfig=/kubeconfig/scheduler.conf
        - --leader-elect=true
        volumeMounts:
        - {name: pki, mountPath: /pki, readOnly: true}
        - {name: kubeconfig, mountPath: /kubeconfig, readOnly: true}
        resources:
          requests: {cpu: 100m, memory: 128Mi}
      volumes:
      - name: pki
        secret:
          secretName: d8-pki-virtual
      - name: kubeconfig
        secret:
          secretName: d8-kubeconfig-virtual
