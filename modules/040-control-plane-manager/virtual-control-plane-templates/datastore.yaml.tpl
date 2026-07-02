apiVersion: managed-services.deckhouse.io/v1alpha1
kind: Postgres
metadata:
  name: d8-datastore-virtual
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/vcp: ${VCP_NAME}
spec:
  postgresClassName: default
  type: Standalone
  users:
  - name: kine
    role: rw
    storeCredsToSecret: d8-datastore-creds-virtual
  databases:
  - name: kine
  instance:
    memory: {size: 1Gi}
    cpu: {cores: 1, coreFraction: 100}
    persistentVolumeClaim: {size: 2Gi}
