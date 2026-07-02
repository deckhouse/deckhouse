apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: ${CPN_NAME}-postgres
  namespace: ${NAMESPACE}
  labels:
    app: postgres
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
    control-plane.deckhouse.io/cpn: ${CPN_NAME}
spec:
  serviceName: ${CPN_NAME}-postgres
  replicas: 1
  selector:
    matchLabels:
      app: postgres
      control-plane.deckhouse.io/cpn: ${CPN_NAME}
  template:
    metadata:
      labels:
        app: postgres
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
      volumes:
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
