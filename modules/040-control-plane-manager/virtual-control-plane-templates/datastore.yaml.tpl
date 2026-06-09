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
  type: Standalone
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
