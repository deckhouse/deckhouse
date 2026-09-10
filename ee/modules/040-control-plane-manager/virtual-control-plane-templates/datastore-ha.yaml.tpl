apiVersion: managed-services.deckhouse.io/v1alpha1
kind: Postgres
metadata:
  name: ${DATASTORE_NAME}
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/virtual-control-plane: ${VCP_NAME}
spec:
  postgresClassName: default
  type: Cluster
  cluster:
    # Ignored, not Zonal/TransZonal: those constrain the PostgresClass zone list, while a
    # two-instance datastore only needs to sit on two different nodes.
    topology: Ignored
    # Availability, not Consistency: with two instances Consistency sets a synchronous standby, and
    # losing it blocks writes - the opposite of what HA is for. kine writes on every tenant object
    # change, so a synchronous round-trip would also sit on the hot path.
    replication: Availability
  users:
  - name: kine
    role: rw
    storeCredsToSecret: ${DATASTORE_CREDS_SECRET_NAME}
  databases:
  - name: kine
  instance:
    memory: {size: 1Gi}
    cpu: {cores: 1, coreFraction: 100}
    persistentVolumeClaim: {size: 2Gi}
