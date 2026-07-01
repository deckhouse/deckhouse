apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ${CPN_NAME}-kube-controller-manager
  namespace: ${NAMESPACE}
  labels:
    app: kube-controller-manager
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
    control-plane.deckhouse.io/cpn: ${CPN_NAME}
spec:
  serviceName: ${CPN_NAME}-kube-controller-manager
  replicas: 1
  selector:
    matchLabels:
      app: kube-controller-manager
      control-plane.deckhouse.io/cpn: ${CPN_NAME}
  template:
    metadata:
      labels:
        app: kube-controller-manager
        control-plane.deckhouse.io/vcp: ${VCP_NAME}
        control-plane.deckhouse.io/cpn: ${CPN_NAME}
    spec:
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: kube-controller-manager
        image: ${IMAGE_KUBE_CONTROLLER_MANAGER}
        command:
        - kube-controller-manager
        - --kubeconfig=/pki/controller-manager.conf
        - --authentication-kubeconfig=/pki/controller-manager.conf
        - --authorization-kubeconfig=/pki/controller-manager.conf
        - --client-ca-file=/pki/ca.crt
        - --cluster-signing-cert-file=/pki/ca.crt
        - --cluster-signing-key-file=/pki/ca.key
        - --root-ca-file=/pki/ca.crt
        - --service-account-private-key-file=/pki/sa.key
        - --use-service-account-credentials=true
        - --leader-elect=true
        - --service-cluster-ip-range=${SERVICE_SUBNET_CIDR}
        - --controllers=*,bootstrapsigner,tokencleaner
        volumeMounts:
        - {name: pki, mountPath: /pki, readOnly: true}
        resources:
          requests: {cpu: 100m, memory: 128Mi}
      volumes:
      - name: pki
        secret:
          secretName: d8-pki-virtual
