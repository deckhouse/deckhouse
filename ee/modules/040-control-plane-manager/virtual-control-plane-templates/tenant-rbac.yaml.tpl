apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kube-apiserver-kubelet-api-admin
  labels:
    heritage: deckhouse
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:kubelet-api-admin
subjects:
- kind: User
  name: kube-apiserver-kubelet-client
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kubeadm:cluster-admins
  labels:
    heritage: deckhouse
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: Group
  name: kubeadm:cluster-admins
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: d8:vcp:konnectivity-server
  labels:
    heritage: deckhouse
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: User
  name: konnectivity-server
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: d8:vcp:deckhouse
  labels:
    heritage: deckhouse
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: deckhouse
  namespace: d8-system
---
# Scrape identity for kube-apiserver, kube-controller-manager and kube-scheduler /metrics.
apiVersion: v1
kind: ServiceAccount
metadata:
  name: d8-metrics-scraper
  namespace: kube-system
  labels:
    heritage: deckhouse
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: d8:vcp:metrics-scraper
  labels:
    heritage: deckhouse
rules:
- nonResourceURLs: ["/metrics"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: d8:vcp:metrics-scraper
  labels:
    heritage: deckhouse
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:vcp:metrics-scraper
subjects:
- kind: ServiceAccount
  name: d8-metrics-scraper
  namespace: kube-system
