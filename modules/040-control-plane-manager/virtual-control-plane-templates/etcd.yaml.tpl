apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ${CPN_NAME}-kine
  namespace: ${NAMESPACE}
  labels:
    app: kine
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
    control-plane.deckhouse.io/cpn: ${CPN_NAME}
spec:
  serviceName: ${CPN_NAME}-kine
  replicas: 1
  selector:
    matchLabels:
      app: kine
      control-plane.deckhouse.io/cpn: ${CPN_NAME}
  template:
    metadata:
      labels:
        app: kine
        control-plane.deckhouse.io/vcp: ${VCP_NAME}
        control-plane.deckhouse.io/cpn: ${CPN_NAME}
    spec:
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: postgres
        image: ${IMAGE_POSTGRES}
        env:
        - {name: POSTGRES_USER, value: kine}
        - {name: POSTGRES_PASSWORD, value: kine}
        - {name: POSTGRES_DB, value: kine}
        - {name: PGDATA, value: /var/lib/postgresql/data/pgdata}
        ports:
        - {containerPort: 5432, name: postgres, protocol: TCP}
        volumeMounts:
        - {name: postgres-data, mountPath: /var/lib/postgresql/data}
        - {name: postgres-run, mountPath: /run/postgresql}
        - {name: postgres-tmp, mountPath: /tmp}
        securityContext:
          runAsUser: 0
          runAsGroup: 0
          allowPrivilegeEscalation: true
          readOnlyRootFilesystem: false
          seccompProfile:
            type: RuntimeDefault
        startupProbe:
          exec: {command: [pg_isready, -U, kine, -d, kine]}
          failureThreshold: 30
          periodSeconds: 10
          timeoutSeconds: 5
        readinessProbe:
          exec: {command: [pg_isready, -U, kine, -d, kine]}
          initialDelaySeconds: 5
          periodSeconds: 5
          failureThreshold: 6
          timeoutSeconds: 5
        livenessProbe:
          exec: {command: [pg_isready, -U, kine, -d, kine]}
          initialDelaySeconds: 30
          periodSeconds: 10
          failureThreshold: 6
          timeoutSeconds: 5
        resources:
          requests: {cpu: 100m, memory: 256Mi}
      - name: kine
        image: ${IMAGE_KINE}
        command:
        - kine
        - --endpoint=postgres://kine:kine@localhost:5432/kine?sslmode=disable
        - --listen-address=0.0.0.0:2379
        - --ca-file=/pki/etcd-ca.crt
        - --cert-file=/pki/etcd-server.crt
        - --key-file=/pki/etcd-server.key
        ports:
        - {containerPort: 2379, name: client, protocol: TCP}
        - {containerPort: 8080, name: metrics, protocol: TCP}
        volumeMounts:
        - {name: pki, mountPath: /pki, readOnly: true}
        securityContext:
          runAsNonRoot: false
          runAsUser: 0
          allowPrivilegeEscalation: false
          capabilities:
            drop: [ALL]
          readOnlyRootFilesystem: true
          seccompProfile:
            type: RuntimeDefault
        resources:
          requests: {cpu: 100m, memory: 128Mi}
          limits: {cpu: 500m, memory: 512Mi}
      volumes:
      - name: pki
        secret:
          secretName: d8-pki-virtual
      - name: postgres-run
        emptyDir: {}
      - name: postgres-tmp
        emptyDir: {}
  volumeClaimTemplates:
  - metadata:
      name: postgres-data
    spec:
      accessModes: [ReadWriteOnce]
      resources:
        requests:
          storage: 2Gi
---
apiVersion: v1
kind: Service
metadata:
  name: ${CPN_NAME}-kine
  namespace: ${NAMESPACE}
  labels:
    app: kine
    control-plane.deckhouse.io/cpn: ${CPN_NAME}
spec:
  clusterIP: None
  selector:
    app: kine
    control-plane.deckhouse.io/cpn: ${CPN_NAME}
  ports:
  - {name: client, port: 2379, targetPort: 2379}
  - {name: metrics, port: 8080, targetPort: 8080}
