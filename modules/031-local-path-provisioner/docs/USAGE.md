---
title: "The local-path-provisioner module: configuration examples"
---

## Example CR `LocalPathProvisioner`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-system
spec:
  nodeGroups:
  - system
  path: "/opt/local-path-provisioner"
```

Notes:

- This example will create `localpath-system` storage class which **must** be used by pods for everything to work
- Volumes created by provisioner will have delete retention policy which is hardcoded ([issue](https://github.com/deckhouse/deckhouse/issues/360))
- If provisioner will be delete before claims folders will not be deleted from node
- Note that in example `system` node is used which probably will have some taints so pods **must** have corresponding tolerations

### StatefulSet spread accross system nodes

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: demo
---
# this provisioner will create "demo" storage class, which MUST be used in statefulset for everything to work
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: demo
spec:
  nodeGroups:
  # we are going to store data on a system nodes, so pods MUST have corresponding tolerations
  - system
  # path on a node where data will be stored, actual path will be "/mnt/kubernetes/demo/pvc-{guid}_{namespace}_{volumeclaimtemplates_name}-{statefulset_metadata_name}-{number}"
  path: /mnt/kubernetes/demo/
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  namespace: demo
  name: demo
  labels:
    app: demo
spec:
  serviceName: demo
  replicas: 2
  selector:
    matchLabels:
      app: demo
  template:
    metadata:
      labels:
        app: demo
    spec:
      # stage and prod nodes may have different taints
      tolerations:
      # stage
      - key: dedicated.deckhouse.io
        operator: Equal
        value: system
        effect: NoSchedule
      # prod
      - key: dedicated.deckhouse.io
        operator: Equal
        value: system
        effect: NoExecute
      # enforce pods to be created on different nodes, which will force local path provisioner to create volumes also on a different nodes
      affinity:
        podAntiAffinity:
          # should work in prod (x2 nodes) and stage (x1 node), for strict mode use "requiredDuringSchedulingIgnoredDuringExecution"
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 1
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - demo
              topologyKey: kubernetes.io/hostname
      containers:
      - name: demo
        image: nginx:alpine
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 80
        volumeMounts:
        - name: demo
          mountPath: /usr/share/nginx/html
  volumeClaimTemplates:
  - metadata:
      name: demo
    spec:
      accessModes:
      - ReadWriteOnce
      # storage class created by local path provisioner MUST be used here
      storageClassName: demo
      resources:
        requests:
          storage: 128Mi
```
