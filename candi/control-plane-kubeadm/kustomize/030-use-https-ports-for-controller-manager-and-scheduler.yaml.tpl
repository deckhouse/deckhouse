---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    livenessProbe:
      httpGet:
        scheme: HTTPS
        port: 10257
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  containers:
  - name: kube-scheduler
    livenessProbe:
      httpGet:
        scheme: HTTPS
        port: 10259
