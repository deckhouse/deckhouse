---
title: "The ingress-nginx module: FAQ"
---

## How do I limit access to the application in the cluster to ingress controllers only?

Add the  kube-rbac-proxy container to the application Pod to allow only ingress Pods to access your application in the cluster:

### An example of the corresponding Kubernetes Deployment
{% raw %}
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-namespace
spec:
  selector:
    matchLabels:
      app: my-app
  replicas: 1
  template:
    metadata:
      labels:
        app: my-app
    spec:
      serviceAccountName: my-sa
      containers:
      - name: my-cool-app
        image: mycompany/my-app:v0.5.3
        args:
        - "--listen=127.0.0.1:8080"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 443
            scheme: HTTPS
      - name: kube-rbac-proxy
        image: flant/kube-rbac-proxy:v0.1.0 # it is recommended to use a proxy from our repository
        args:
        - "--secure-listen-address=0.0.0.0:443"
        - "--config-file=/etc/kube-rbac-proxy/config-file.yaml"
        # User verification certificate, refers to the standard Kubernetes client CA
        # (available in every Pod)
        - "--client-ca-file=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
        - "--v=2"
        - "--logtostderr=true"
        # The user authentication and authorization are not possible if the kube-apiserver is not available.
        # Stale Cache stores the results of successful authorization and is used only if the apiserver is not available.
        - "--stale-cache-interval=1h30m"
        ports:
        - containerPort: 443
          name: https
        volumeMounts:
        - name: kube-rbac-proxy
          mountPath: /etc/kube-rbac-proxy
      volumes:
      - name: kube-rbac-proxy
        configMap:
          name: kube-rbac-proxy
```
{% endraw %}

The application only accepts localhost (127.0.0.1) requests. That means that an unsecured connection can only be established to it from within the Pod.
At the same time, the proxy listens on 0.0.0.0 and intercepts all external traffic to the Pod.

## How do I provide minimum rights to the Service Account?

The proxy needs permissions to create `TokenReview` and `SubjectAccessReview` to authenticate and authorize users using the kube-apiserver.

Our clusters have a [built-in ClusterRole](https://github.com/deckhouse/deckhouse/blob/main/modules/020-deckhouse/templates/common/rbac/kube-rbac-proxy.yaml) called **d8-rbac-proxy** that is ideal for this kind of situation.
You don't need to create it yourself! Just attach it to the ServiceAccount of your Deployment.
{% raw %}
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

### The Kube-RBAC-Proxy configuration
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-rbac-proxy
data:
  config-file.yaml: |+
    excludePaths:
    - /healthz # no authorization for liveness probes is required
    upstreams:
    - upstream: http://127.0.0.1:8081/ # the destination address
      path: / # the path to the proxy to forward requests to the upstream
      authorization:
        resourceAttributes:
          namespace: my-namespace
          apiGroup: apps
          apiVersion: v1
          resource: deployments
          subresource: http
          name: my-app
```
{% endraw %}
According to the configuration, the user must have access to the `my-app` Deployment and its `http` subresource in the `my-namespace` namespace.

Such permissions have the following RBAC form:
{% raw %}
```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kube-rbac-proxy:my-app
  namespace: my-namespace
rules:
- apiGroups: ["apps"]
  resources: ["deployments/http"]
  resourceNames: ["my-app"]
  verbs: ["get", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kube-rbac-proxy:my-app
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kube-rbac-proxy:my-app
subjects:
# All user certificates of ingress-controllers are issued for one specific group
- kind: Group
  name: ingress-nginx:auth
```

You also need to add the following parameters to the ingress of the resource:
```yaml
nginx.ingress.kubernetes.io/backend-protocol: HTTPS
nginx.ingress.kubernetes.io/configuration-snippet: |
  proxy_ssl_certificate /etc/nginx/ssl/client.crt;
  proxy_ssl_certificate_key /etc/nginx/ssl/client.key;
  proxy_ssl_protocols TLSv1.2;
  proxy_ssl_session_reuse on;
```
{% endraw %}
[Here](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#x509-client-certs) you can read more about how certificate authentication works.


## How do I configure MetalLB to be accessible from the internal network only?

Below is an example of a MetalLB config with access from the internal network only.
```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  loadBalancer:
    sourceRanges:
    - 192.168.0.0/24
```

## How to add extra log fields to a nginx-controller?
```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  additionalLogFields:
    my-cookie: "$cookie_MY_COOKIE"
```


## How to enable HorizontalPodAutoscaling for IngressNginxController?

> **Note!** HPA mode is possible only for controllers with inlet: `LoadBalancer` or `LoadBalancerWithProxyProtocol`.

HPA is set with attributes `minReplicas` and `maxReplicas` in a [IngressNginxController CR](cr.html#ingressnginxcontroller).

`hpa-scaler` Deployment will be created with the HPA resource, which is observing custom metric `prometheus-metrics-adapter-d8-ingress-nginx-cpu-utilization-for-hpa`.

If CPU utilization > 50% HPA-controller scales `hpa-scaler` Deployment with a new replica (with respect of `minReplicas` and `maxReplicas`).

`hpa-scaler` Deployment has HardPodAntiAffinity and it will order a new Node (inside it's NodeGroup), where one more ingress-controller will be set.


## How to use IngressClass with IngressClassParameters

Since version 1.1 IngressNginxController Deckhouse creates an IngressClass object. If you want to use your own IngressClass
with your customized IngressClassParameters, you need to add the label `ingress-class.deckhouse.io/external: "true"`

```yaml
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  labels:
    ingress-class.deckhouse.io/external: "true"
  name: my-super-ingress
spec:
  controller: ingress-nginx.deckhouse.io/my-super-ingress
  parameters:
    apiGroup: elbv2.k8s.aws
    kind: IngressClassParams
    name: awesome-class-cfg
```

In this case Deckhouse will not create an IngressClass object and will use your own.
