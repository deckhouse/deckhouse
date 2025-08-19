---
title: "Canary deployment"
permalink: en/user/network/canary-deployment.html
---

Canary deployment is an application deployment strategy
that allows gradually rolling out a new application version to production.
This approach makes it possible to test new versions on a small portion of traffic,
minimizing risks and ensuring a smooth transition.
With Canary deployment, traffic can be shifted to the new version as confidence in its stability grows,
with the ability to quickly roll back to the old version if issues arise.
In Deckhouse Kubernetes Platform, Canary deployment can be implemented
using the [`ingress-nginx`](/modules/ingress-nginx/) module or the [`istio`](/modules/istio/) module (recommended).

## Example Canary deployment configuration with Ingress NGINX

To implement Canary deployment with Ingress NGINX,
annotations and rules are used to route a portion of traffic to the new application version.

### Creating a Deployment and Service for the stable version

Example manifest for the stable version:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-v1
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
      version: v1
  template:
    metadata:
      labels:
        app: my-app
        version: v1
    spec:
      containers:
      - name: app
        image: app:v1
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: app-service
spec:
  selector:
    app: my-app
  ports:
  - port: 80
    targetPort: 80
```

### Creating a Deployment and Service for the Canary version

Example manifest for the Canary version:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-v2
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
      version: v2
  template:
    metadata:
      labels:
        app: my-app
        version: v2
    spec:
      containers:
      - name: app
        image: app:v2
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: app-canary-service
spec:
  selector:
    app: my-app
    version: v2
  ports:
  - port: 80
    targetPort: 80
```

### Configuring Ingress for Canary deployment

For Canary deployment with Ingress NGINX, special annotations are used:

- `nginx.ingress.kubernetes.io/canary`: Enables Canary mode for the Ingress.
- `nginx.ingress.kubernetes.io/canary-weight`: Specifies the percentage of traffic to be routed to the Canary version.

Example Ingress manifest (10% of traffic will go to the Canary version (`app-canary-service`),
90% to the stable version (`app-service`)):

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-ingress
  annotations:
    nginx.ingress.kubernetes.io/canary: "true"
    nginx.ingress.kubernetes.io/canary-weight: "10" # 10% traffic to Canary.
spec:
  rules:
  - host: example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: app-canary-service
            port:
              number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-ingress-main
spec:
  rules:
  - host: example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: app-service
            port:
              number: 80
```

### Gradually increasing traffic to the Canary version

You can gradually increase the percentage of traffic to the Canary version
by changing the value of the `nginx.ingress.kubernetes.io/canary-weight` annotation.
For example, to send 50% of traffic to the Canary version, update the annotation as follows:

```yaml
nginx.ingress.kubernetes.io/canary-weight: "50"
```

### Rolling back or completing the Canary deployment

If the Canary version is stable, you can fully switch traffic to the new version
by removing the Canary annotations and updating the main Ingress.
If problems occur, you can reduce the percentage of traffic going to the Canary version
or disable it completely by setting `nginx.ingress.kubernetes.io/canary-weight: "0"`.

### Additional annotations for Canary deployment

- `nginx.ingress.kubernetes.io/canary-by-header`: Routes traffic to the Canary version based on the value of an HTTP header.
- `nginx.ingress.kubernetes.io/canary-by-cookie`: Routes traffic to the Canary version based on the value of a cookie.

Example using a header:

```yaml
nginx.ingress.kubernetes.io/canary-by-header: "canary"
nginx.ingress.kubernetes.io/canary-by-header-value: "true"
```

In this case, traffic will be routed to the Canary version if the request contains the header `canary: true`.

## Example Canary deployment configuration with Istio

{% alert level="info" %}
Istio is responsible only for flexible request routing based on special request headers (for example, cookies) or simple randomness.
The actual routing configuration and switching between Canary versions is managed by the CI/CD system.
{% endalert %}

It is assumed that the same namespace contains two Deployments with different application versions.
Pods of different versions have different labels (`version: v1` and `version: v2`).

Two custom resources need to be configured:

- [DestinationRule](../network/managing_request_between_service_istio.html#destinationrule-resource) describing
  how to identify different versions of your application (subsets).
- [VirtualService](../network/retry_istio.html#virtualservice-resource) describing
  how to distribute traffic between different application versions.

Example:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: productpage-canary
spec:
  host: productpage
  # Subsets are available only when accessing the host through a VirtualService from a Pod managed by Istio.
  # These subsets must be specified in the routes.
  subsets:
  - name: v1
    labels:
      version: v1
  - name: v2
    labels:
      version: v2
```

### Distribution based on the presence of a cookie

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: productpage-canary
spec:
  hosts:
  - productpage
  http:
  - match:
    - headers:
       cookie:
         regex: "^(.*;?)?(canary=yes)(;.*)?"
    route:
    - destination:
        host: productpage
        subset: v2 # Reference to a subset from DestinationRule.
  - route:
    - destination:
        host: productpage
        subset: v1
```

### Distribution based on probability

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: productpage-canary
spec:
  hosts:
  - productpage
  http:
  - route:
    - destination:
        host: productpage
        subset: v1 # Reference to a subset from DestinationRule.
      weight: 90 # Percentage of traffic that goes to Pods with the label `version: v1`.
  - route:
    - destination:
        host: productpage
        subset: v2
      weight: 10
```
