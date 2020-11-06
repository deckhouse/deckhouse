{{- $millicpu := $.allocatableMilliCpuControlPlane | default 512 -}}
{{- $memory := $.allocatableMemoryControlPlane | default 536870912 -}}
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    resources:
      requests:
        cpu: "{{ div (mul $millicpu 20) 100 }}m"
        memory: "{{ div (mul $memory 20) 100 }}"
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  containers:
  - name: kube-scheduler
    resources:
      requests:
        cpu: "{{ div (mul $millicpu 10) 100 }}m"
        memory: "{{ div (mul $memory 10) 100 }}"
---
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
    resources:
      requests:
        cpu: "{{ div (mul $millicpu 35) 100 }}m"
        memory: "{{ div (mul $memory 35) 100 }}"
---
apiVersion: v1
kind: Pod
metadata:
  name: etcd
  namespace: kube-system
spec:
  containers:
  - name: etcd
    resources:
      requests:
        cpu: "{{ div (mul $millicpu 35) 100 }}m"
        memory: "{{ div (mul $memory 35) 100 }}"
