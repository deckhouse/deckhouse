---
title: "Enabling Istio for applications"
permalink: en/user/network/app_istio_activation.html
---

Enabling Istio for applications is possible if the [istio](/modules/istio/) module
is enabled and configured in the cluster.
This is handled by the cluster administrator.

The essence of enabling Istio is adding a sidecar container to the application Pods, after which Istio can manage the traffic.

The recommended way to add sidecars is to use the sidecar injector.
Istio can "inject" a sidecar container into application Pods using the [Admission Webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) mechanism.
Labels and annotations are used to add sidecars:

- A label on a `namespace` marks your namespace for the sidecar injector component.
  After the label is applied, new Pods will have sidecar containers added:
  - `istio-injection=enabled`: Uses the global Istio version (`spec.settings.globalVersion` in the ModuleConfig resource);
  - `istio.io/rev=v1x16`: Uses a specific Istio version for this namespace.
- A Pod annotation `sidecar.istio.io/inject` (`"true"` or `"false"`) allows you to locally override the `sidecarInjectorPolicy`.
  These annotations only work in namespaces marked with one of the labels listed above.

It is also possible to add a sidecar to a specific Pod in a namespace
without the `istio-injection=enabled` or `istio.io/rev=vXxYZ` labels by setting the label `sidecar.istio.io/inject=true`.

The Istio proxy, which runs as a sidecar container, also consumes resources and adds overhead:

- Each request is DNATed into Envoy, which processes the request and creates another one.
  On the receiving side, the same happens.
- Each Envoy stores information about all Services in the cluster, which requires memory.
  The larger the cluster, the more memory Envoy consumes. The solution is the [Sidecar](/modules/istio/istio-cr.html#sidecar) custom resource.

It is also important to prepare the Ingress controller and the application's Ingress resources:

- Enable [`enableIstioSidecar`](/modules/ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) in the IngressNginxController resource.
- Add annotations to the application's Ingress resources:
  - `nginx.ingress.kubernetes.io/service-upstream: "true"`: The Ingress controller uses the Service's ClusterIP
    as the upstream instead of the Pod IPs. Traffic balancing between Pods is now handled by the sidecar proxy.
    Use this option only if your Service has a ClusterIP.
  - `nginx.ingress.kubernetes.io/upstream-vhost: "myservice.myns.svc"`: The Ingress controller's sidecar proxy
    makes routing decisions based on the `Host` header.
    Without this annotation, the controller will keep the site address header, for example, `Host: example.com`.
