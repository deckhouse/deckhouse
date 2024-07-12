---
title: "Developing Prometheus targets"
type:
  - instruction
search: Developing Prometheus targets, prometheus target
---

## General information

* The most common operation is adding a target for a new application (redis, rabbitmq, etc.). In most cases, you only need to copy one of the existing service monitors in the `applications` directory and edit the names.
* However, if you need to do something more complex or mere copying does not produce the expected result, refer to the [Prometheus Operator](../../modules/200-operator-prometheus/) module documentation.
* All existing targets are located in the `prometheus-targets` directory. They usually consist of a service monitor, some Prometheus exporter, and the necessary wrapping that binds them together.
* All internal ServiceMonitors and PodMonitors should be created in the namespace `d8-monitoring`.

## Best practices

### Set labels for Pod-oriented metrics

Most metrics stored in Prometheus either contain Pod-related data or information about the parameters of the application running in the Pod. We call these metrics ~Pod-oriented~. They include (predominantly but not exclusively) the following metric varieties:
* system metrics that reflect the performance parameters of the Pod (these are exported by kubelet);
* application metrics:
  * metrics of supported applications (redis, rabbitmq, etc.);
  * custom metrics.

All Pod-oriented labels have a label with the Pod name (generally, it is called `instance`; for the kubelet-generated metrics, this label is called `pod_name`, and `pod` - for kube-state-metrics-generated ones). However, Pod names are not convenient to work with, and we prefer to use `service` and `namespace` parameters. Thus:
* all Pod-oriented metrics have a `namespace` label;
* application and custom Pod-oriented metrics also have a `service` label that unites a group of Pods under one clear name.

### Authorize access to the exported metrics

We strongly recommend configuring metric exporters so that only authenticated and authorized users can access them.

For this, you can use the [kube-rbac-proxy](https://github.com/brancz/kube-rbac-proxy) Kubernetes proxy. It is written in Go and can authenticate the user with `TokenReview` or a client certificate.
Authorization is performed using `SubjectAccessReview` according to the RBAC rules defined for the user.

#### Below is an example of the Deployment of a secure exporter

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-exporter
  namespace: my-namespace
spec:
  selector:
    matchLabels:
      app: my-exporter
  replicas: 1
  template:
    metadata:
      labels:
        app: my-exporter
    spec:
      serviceAccountName: my-sa
      containers:
      - name: my-cool-app
        image: mycompany/my-cool-exporter:v0.5.3
        args:
        - "--listen=127.0.0.1:8081"
      - name: kube-rbac-proxy
        image: flant/kube-rbac-proxy:v0.1.0 # we recommend using the proxy version from our repository
        args:
        - "--secure-listen-address=0.0.0.0:8080"
        - "--config-file=/etc/kube-rbac-proxy/config-file.yaml"
        - "--v=2"
        - "--logtostderr=true"
        # If kube-apiserver is not available, you will not be able to authenticate and authorize users.
        # Stale Cache stores the results of successful authorization and is used if the apiserver is not available.
        - "--stale-cache-interval=1h30m"
        ports:
        - containerPort: 8080
          name: https-metrics
        volumeMounts:
        - name: kube-rbac-proxy
          mountPath: /etc/kube-rbac-proxy
      volumes:
      - name: kube-rbac-proxy
        configMap:
          name: kube-rbac-proxy
```

The metric exporter only accepts localhost (127.0.0.1) requests. That means that an unsecured connection can only be established to it from within the Pod.
At the same time, the proxy listens on 0.0.0.0 and intercepts all external traffic to the Pod.

### Eliminate unnecessary rights for Service Accounts

The proxy only needs permissions to create `TokenReview` and `SubjectAccessReview` to authenticate and authorize users using kube-apiserver.

Our clusters have a [built-in **d8-rbac-proxy** ClusterRole](https://github.com/deckhouse/deckhouse/blob/main/modules/002-deckhouse/templates/common/rbac/kube-rbac-proxy.yaml) that is ideal for this kind of situation.
You don't need to create it yourself! You just need to attach it to the ServiceAccount of your Deployment.

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-sa
  namespace: my-namespace
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-namespace:my-sa:d8-rbac-proxy
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:rbac-proxy
subjects:
- kind: ServiceAccount
  name: my-sa
  namespace: my-namespace
```

### Configure Kube-RBAC-Proxy

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-rbac-proxy
data:
  config-file.yaml: |+
    upstreams:
    - upstream: http://127.0.0.1:8081/metrics # proxy destination
      path: /metrics # a source path to use for proxying requests upstream
      authorization:
        resourceAttributes:
          namespace: my-namespace
          apiGroup: apps
          apiVersion: v1
          resource: deployments
          subresource: prometheus-metrics
          name: my-exporter
```

According to the configuration, the user must have access to the `my-exporter` Deployment and its `prometheus-metrics` subresource in the `my-namespace` namespace.

Such permissions have the following RBAC form:

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kube-rbac-proxy:my-exporter
  namespace: my-namespace
rules:
- apiGroups: ["apps"]
  resources: ["deployments/prometheus-metrics"]
  resourceNames: ["my-exporter"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kube-rbac-proxy:my-exporter
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kube-rbac-proxy:my-exporter
subjects:
- kind: User
  name: my-user
```

Now the `my-user` user can collect metrics from your Pod.
