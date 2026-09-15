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
    # Zonal and TransZonal constrain the PostgresClass zone list; node-level spread is enough here.
    topology: Ignored
    # Gives three instances. The synchronous standby is ANY 1 of them, so a second standby keeps
    # writes flowing while one is down; the cost is a round trip on every commit.
    replication: ConsistencyAndAvailability
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
