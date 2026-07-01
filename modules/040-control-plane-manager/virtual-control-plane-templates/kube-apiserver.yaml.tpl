apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ${CPN_NAME}-kube-apiserver
  namespace: ${NAMESPACE}
  labels:
    app: kube-apiserver
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
    control-plane.deckhouse.io/cpn: ${CPN_NAME}
spec:
  serviceName: ${CPN_NAME}-kube-apiserver
  replicas: 1
  selector:
    matchLabels:
      app: kube-apiserver
      control-plane.deckhouse.io/cpn: ${CPN_NAME}
  template:
    metadata:
      labels:
        app: kube-apiserver
        control-plane.deckhouse.io/vcp: ${VCP_NAME}
        control-plane.deckhouse.io/cpn: ${CPN_NAME}
    spec:
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: kube-apiserver
        image: ${IMAGE_KUBE_APISERVER}
        command:
        - kube-apiserver
        - --etcd-servers=https://${CPN_NAME}-kine:2379
        - --etcd-cafile=/pki/etcd-ca.crt
        - --etcd-certfile=/pki/apiserver-etcd-client.crt
        - --etcd-keyfile=/pki/apiserver-etcd-client.key
        - --client-ca-file=/pki/ca.crt
        - --tls-cert-file=/pki/apiserver.crt
        - --tls-private-key-file=/pki/apiserver.key
        - --kubelet-client-certificate=/pki/apiserver-kubelet-client.crt
        - --kubelet-client-key=/pki/apiserver-kubelet-client.key
        - --service-account-key-file=/pki/sa.pub
        - --service-account-signing-key-file=/pki/sa.key
        - --service-account-issuer=https://kubernetes.default.svc.${CLUSTER_DOMAIN}
        - --requestheader-client-ca-file=/pki/front-proxy-ca.crt
        - --requestheader-allowed-names=front-proxy-client
        - --requestheader-extra-headers-prefix=X-Remote-Extra-
        - --requestheader-group-headers=X-Remote-Group
        - --requestheader-username-headers=X-Remote-User
        - --proxy-client-cert-file=/pki/front-proxy-client.crt
        - --proxy-client-key-file=/pki/front-proxy-client.key
        - --service-cluster-ip-range=${SERVICE_SUBNET_CIDR}
        - --authorization-mode=Node,RBAC
        - --allow-privileged=true
        - --secure-port=6443
        - --advertise-address=$(POD_IP)
        env:
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        ports:
        - {containerPort: 6443, name: https}
        volumeMounts:
        - {name: pki, mountPath: /pki, readOnly: true}
        readinessProbe:
          httpGet: {path: /readyz, port: 6443, scheme: HTTPS}
          initialDelaySeconds: 10
          periodSeconds: 5
        livenessProbe:
          httpGet: {path: /livez, port: 6443, scheme: HTTPS}
          initialDelaySeconds: 30
          periodSeconds: 10
        resources:
          requests: {cpu: 250m, memory: 512Mi}
      volumes:
      - name: pki
        secret:
          secretName: d8-pki-virtual
