# Rendered for HA only. A PodDisruptionBudget over a single replica blocks node drains instead of
# protecting anything.
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: ${VCP_NAME}-kube-apiserver
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/virtual-control-plane: ${VCP_NAME}
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app: kube-apiserver
      control-plane.deckhouse.io/vcp: ${VCP_NAME}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: ${VCP_NAME}-kube-controller-manager
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/virtual-control-plane: ${VCP_NAME}
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app: kube-controller-manager
      control-plane.deckhouse.io/vcp: ${VCP_NAME}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: ${VCP_NAME}-kube-scheduler
  namespace: ${NAMESPACE}
  labels:
    heritage: deckhouse
    control-plane.deckhouse.io/virtual-control-plane: ${VCP_NAME}
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app: kube-scheduler
      control-plane.deckhouse.io/vcp: ${VCP_NAME}
