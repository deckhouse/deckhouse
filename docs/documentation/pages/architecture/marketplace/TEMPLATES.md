---
title: Templates
permalink: en/architecture/marketplace/templates.html
description: "Helm templates of an Application package: template values, object naming, container images and container registry access, Ingress, HTTPRoute, ServiceAccount and application endpoints, credentials in the web interface, labels, and workload health."
---

{% raw %}

The `templates/` directory of the package contains Helm templates. Deckhouse Platform (DP) renders them and applies the result with Nelm, so the templates can use [Nelm annotations](nelm-annotations.html) to control the deployment order, resource lifecycle, and readiness tracking. This page describes the data DP passes to the templates and the rules the templates must follow.

## Template values

DP renders the templates with three value roots:

| Root | Contents |
|---|---|
| `.Values` | Application values: settings with default values, internal values, and values set by hooks (for details, refer to ["How DP builds values"](settings.html#how-dp-builds-values)). The values are not nested under the package name: a setting is available as `.Values.<SETTING_NAME>` |
| `.Application` | Instance and package parameters |
| `.Platform` | DP global parameters (read-only) |

### Root .Application

The `.Application` root contains instance and package parameters.

| Path | Description |
|---|---|
| `.Application.Instance.Name` | Instance name, that is, the name of the Application resource |
| `.Application.Instance.Namespace` | Namespace the application is installed into |
| `.Application.Package.Name` | Package name (the `name` field of `package.yaml`) |
| `.Application.Package.Version` | Package version |
| `.Application.Package.Images` | Container images of the package: a map of image names to image references with a digest (for details, refer to ["Container images"](#container-images)) |
| `.Application.Package.Registry` | Access parameters of the repository the package is installed from (for details, refer to ["Container images"](#container-images)) |
| `.Application.Settings` | Effective settings: `Application.spec.settings` with default values applied |

### Root .Platform

`.Platform` contains the DP global parameters. The following parameters are useful in application templates:

| Path | Description |
|---|---|
| `.Platform.applications.publicDomainTemplate` | DNS name template for application Ingresses (for details, refer to ["Ingress and HTTPS"](#ingress-and-https)) |
| `.Platform.applications.ingressClass` | IngressClass for application Ingresses. Default: `nginx` |
| `.Platform.applications.https.mode` | HTTPS mode for applications: `CertManager` (default), `CustomCertificate`, `Disabled`, or `OnlyInURI` |
| `.Platform.applications.https.certManager.clusterIssuerName` | The ClusterIssuer of the [`cert-manager`](/modules/cert-manager/) module for application certificates. Default: `letsencrypt` |
| `.Platform.discovery.clusterDomain` | Cluster domain, for example, `cluster.local` |
| `.Platform.discovery.kubernetesVersion` | Kubernetes version of the cluster |
| `.Platform.deckhouseVersion` | DP version |
| `.Platform.deckhouseEdition` | DP edition |

The administrator configures the `.Platform.applications` parameters for all applications of the cluster in the [`applications`](../../reference/api/global.html#parameters-applications) section of the global settings.

## Object names

Name every object in the templates as `d8a-<INSTANCE_NAME>-<SUFFIX>`:

```yaml
metadata:
  name: d8a-{{ .Application.Instance.Name }}-server
```

The `d8 package verify` command reports an error (the `instance-prefix` rule) for every rendered object whose name doesn't start with `d8a-{{ .Application.Instance.Name }}-`. For Job and CronJob objects, the suffix after this prefix must be at most 23 characters long (the `job-name` rule).

The prefix serves the following purposes:

- Several instances of the package can be installed into the same namespace without name conflicts.
- The `d8a-` prefix is reserved for applications: the `d8a-prefix.deckhouse.io` admission policy forbids users to create, change, and delete objects whose names start with `d8a-`. Deleting Pods and changing Secrets with the `packages.deckhouse.io/initial-secret` label are allowed (for details, refer to ["Credentials in the web interface"](#credentials-in-the-web-interface)).
- DP adds the same prefix to the names of objects that hooks create, patch, or delete. Therefore, a hook can address an object rendered by the templates by its suffix only (for details, refer to ["Hooks"](hooks.html#object-names)).

Don't use `.Release.Name` in object names: the Helm release of an application is named `<NAMESPACE>.<INSTANCE_NAME>`, and the dot is not allowed in the names of many resource kinds.

Keep suffixes short: the instance name can be up to 24 characters long, and the full object name must fit the Kubernetes limits (for details, refer to ["Naming constraints"](concepts.html#naming-constraints)).

## Labels and object protection

DP adds the following labels to the metadata of every rendered object:

| Label | Value |
|---|---|
| `heritage` | `deckhouse` |
| `packages.deckhouse.io/package` | Package name |
| `packages.deckhouse.io/instance` | Instance name |
| `health.deckhouse.io/package` | `<NAMESPACE>.<INSTANCE_NAME>` |

The labels are added only to the metadata of the rendered objects, not to Pod templates. Define the labels used in Pod selectors in the templates yourself.

When the Application is deleted, DP finds the objects that the instance has left in the namespace by the `packages.deckhouse.io/instance` label (for details, refer to ["Orphan resources"](lifecycle.html#orphan-resources)).

Objects with the `heritage: deckhouse` label are protected by the `label-objects.deckhouse.io` admission policy, and objects with the `d8a-` name prefix are protected by the `d8a-prefix.deckhouse.io` policy. As a result:

- Users can't change or delete objects rendered by the templates. The exception is Secrets with the `packages.deckhouse.io/initial-secret` label: users can change them, but not create or delete them (for details, refer to ["Credentials in the web interface"](#credentials-in-the-web-interface)). To change them manually, for example, when debugging, switch the application to [maintenance mode](lifecycle.html#maintenance-mode).
- Workloads of the application can't change objects rendered by the templates either.
- The `d8a-prefix.deckhouse.io` policy doesn't apply to service accounts whose names start with `d8a-`. Workloads that run under such service accounts can manage objects with the `d8a-` prefix they create themselves, while these objects stay protected from users.

## Container images

Reference the container images of the package through `.Application.Package.Images`. The key is the name of the image directory in `images/` converted to camelCase (for example, `images/my-server` becomes `myServer`). The value is a full image reference with a digest, for example, `registry.example.com/packages/myapp@sha256:...`:

```yaml
containers:
  - name: server
    image: {{ index .Application.Package.Images "server" }}
```

The `d8 package render` command uses the directory names as is, so use single-word names for image directories: then the same key works both in local rendering and in the cluster.

To pull images from the repository, create an image pull Secret from `.Application.Package.Registry.dockercfg` and reference it in `imagePullSecrets`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: d8a-{{ .Application.Instance.Name }}-registrysecret
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: {{ .Application.Package.Registry.dockercfg }}
```

The section `.Application.Package.Registry` contains the following fields:

| Field | Description |
|---|---|
| `name` | Name of the PackageRepository resource |
| `repository` | Repository address from the PackageRepository resource |
| `dockercfg` | Base64-encoded Docker configuration for accessing the repository. If the PackageRepository uses `login` and `password`, DP builds the configuration from them |
| `scheme` | Protocol for accessing the repository as specified in the PackageRepository resource, for example, `HTTPS`. DP passes the value as is: it can be in any case or empty, which means HTTPS |
| `ca` | CA certificate of the repository, if specified |

## Ingress and HTTPS

Build host names and TLS settings of application Ingresses from the `.Platform.applications` parameters:

- `publicDomainTemplate`: DNS name template with two `%s` placeholders. The first one is replaced with the instance name, the second one with the namespace. For example, with the `%s.%s.apps.example.com` template, the `grafana` instance in the `monitoring` namespace gets the `grafana.monitoring.apps.example.com` host name. If the parameter is not set, don't create Ingresses.
- `ingressClass`: IngressClass of the Ingresses.
- `https.mode`: HTTPS mode. In the `CertManager` mode, request a certificate from the ClusterIssuer specified in `https.certManager.clusterIssuerName`. In the `Disabled` and `OnlyInURI` modes, don't configure TLS on the Ingress.

Example Ingress:

```yaml
{{- $applications := .Platform.applications }}
{{- if $applications.publicDomainTemplate }}
{{- $host := printf $applications.publicDomainTemplate .Application.Instance.Name .Application.Instance.Namespace }}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: d8a-{{ .Application.Instance.Name }}-web
  annotations:
    packages.deckhouse.io/application-endpoint-description: "Web UI"
    {{- if eq $applications.https.mode "CertManager" }}
    cert-manager.io/cluster-issuer: {{ $applications.https.certManager.clusterIssuerName }}
    {{- end }}
spec:
  ingressClassName: {{ $applications.ingressClass }}
  {{- if eq $applications.https.mode "CertManager" }}
  tls:
    - hosts:
        - {{ $host }}
      secretName: d8a-{{ .Application.Instance.Name }}-web-tls
  {{- end }}
  rules:
    - host: {{ $host }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: d8a-{{ .Application.Instance.Name }}-server
                port:
                  name: http
{{- end }}
```

The package skeleton created by `d8 package bootstrap app` contains helpers for these parameters in `templates/_helpers/`:

- `public_domain`
- `ingress_class`
- `https_mode`
- `https_ingress_tls_enabled`
- `https_cert_manager_cluster_issuer_name`

## Application endpoints

To show the addresses of an application in the [`status.urls`](../../reference/api/cr.html#application-v1alpha1-status-urls) field of the Application (for example, the web interface shows them as links to the application), add the `packages.deckhouse.io/application-endpoint-description` annotation to an Ingress, to an HTTPRoute if the application is published through the [`alb`](/modules/alb/) module, or to a [ServiceAccount](#serviceaccount-endpoints):

- The annotation value is the endpoint description. The `"true"` value adds the endpoint without a description, and the `"false"` value excludes the object.
- Only objects rendered by the templates are taken into account: Ingress objects of the `networking.k8s.io` API group, HTTPRoute objects of the `gateway.networking.k8s.io` API group, and ServiceAccount objects (`apiVersion: v1`).
- Deckhouse Platform updates `status.urls` after every successful apply of the templates.

For an Ingress, Deckhouse Platform builds a URL for every `host` and `path` pair of `spec.rules`. The scheme is `https` if the host is listed in `spec.tls`, and `http` otherwise. A rule without paths gives the `<SCHEME>://<HOST>/` URL. Rules without `host` are skipped.

Example status of an Application with the Ingress from the previous section:

```yaml
status:
  urls:
    - url: https://myapp.my-namespace.apps.example.com/
      description: Web UI
```

### HTTPRoute endpoints

For an HTTPRoute, Deckhouse Platform builds a URL for every host name of `spec.hostnames` and every path of `spec.rules`:

- The path is the value of a `PathPrefix` or `Exact` match. A rule without matches and a match without a path give the `/` path. Matches of the `RegularExpression` type are skipped.
- If `spec.hostnames` is empty, Deckhouse Platform takes the host names of the listeners the HTTPRoute is attached to.
- The scheme is `https` if the HTTPRoute is attached to an `HTTPS` listener that serves the host name, and `http` otherwise. Deckhouse Platform takes into account only the listeners of ListenerSet objects rendered by the templates. For an HTTPRoute attached directly to a Gateway or to a ListenerSet created outside the templates, the scheme is `http`.

Example ListenerSet and HTTPRoute:

```yaml
{{- $applications := .Platform.applications }}
{{- if $applications.publicDomainTemplate }}
{{- $host := printf $applications.publicDomainTemplate .Application.Instance.Name .Application.Instance.Namespace }}
apiVersion: gateway.networking.k8s.io/v1
kind: ListenerSet
metadata:
  name: d8a-{{ .Application.Instance.Name }}-web
spec:
  parentRef:
    name: <GATEWAY_NAME>
    namespace: <GATEWAY_NAMESPACE>
  listeners:
    - name: web
      port: 443
      protocol: HTTPS
      hostname: {{ $host }}
      tls:
        mode: Terminate
        certificateRefs:
          - name: d8a-{{ .Application.Instance.Name }}-web-tls
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: d8a-{{ .Application.Instance.Name }}-web
  annotations:
    packages.deckhouse.io/application-endpoint-description: "Web UI"
spec:
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: ListenerSet
      name: d8a-{{ .Application.Instance.Name }}-web
      sectionName: web
  hostnames:
    - {{ $host }}
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: d8a-{{ .Application.Instance.Name }}-server
          port: 80
{{- end }}
```

where:

- `<GATEWAY_NAME>` and `<GATEWAY_NAMESPACE>` are the name and namespace of the Gateway object that the cluster administrator provides for publishing applications;
- `d8a-{{ .Application.Instance.Name }}-web-tls` is a Secret with the TLS certificate for the host name, for example, issued by cert-manager. For details, refer to the [`alb` module documentation](/modules/alb/).

For this HTTPRoute, Deckhouse Platform adds the same endpoint to `status.urls` as for the example Ingress: `https://myapp.my-namespace.apps.example.com/` with the `Web UI` description.

### ServiceAccount endpoints

If the application gives users a ServiceAccount token for accessing the Kubernetes API, add the annotation to this ServiceAccount. For a ServiceAccount, Deckhouse Platform adds the in-cluster address of the Kubernetes API, `https://kubernetes.default.svc/`, to `status.urls`. This address is the same for all ServiceAccounts and is reachable only from inside the cluster.

Example ServiceAccount:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: d8a-{{ .Application.Instance.Name }}-api
  annotations:
    packages.deckhouse.io/application-endpoint-description: "Kubernetes API"
```

For this ServiceAccount, Deckhouse Platform adds the `https://kubernetes.default.svc/` endpoint with the `Kubernetes API` description to `status.urls`. To show the ServiceAccount token next to this endpoint in the web interface, bind a Secret with the token to the ServiceAccount (for details, refer to ["ServiceAccount token"](#serviceaccount-token)).

## Credentials in the web interface

The web interface can show the credentials for signing in next to an application endpoint, for example, the login and password of the administrator account that the application creates during installation. To do this, add a Secret with the login and password to the templates and bind this Secret to the endpoint:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: d8a-{{ .Application.Instance.Name }}-web-credentials
  labels:
    packages.deckhouse.io/application-instance: {{ .Application.Instance.Name }}
    packages.deckhouse.io/application-ingress: d8a-{{ .Application.Instance.Name }}-web
    packages.deckhouse.io/initial-secret: "true"
type: Opaque
data:
  login: {{ "admin" | b64enc }}
  password: {{ .Values.internal.adminPassword | b64enc }}
```

The web interface finds the credentials as follows:

- The `packages.deckhouse.io/application-instance` label with the instance name is required. The web interface looks for credentials only among the Secrets with this label in the namespace of the application. Deckhouse Platform doesn't add this label to the objects, and the `packages.deckhouse.io/instance` label that Deckhouse Platform adds (for details, refer to ["Labels and object protection"](#labels-and-object-protection)) is not taken into account.
- The `packages.deckhouse.io/application-ingress` label binds the Secret to an Ingress from the templates by its name. For an application published through the [`alb`](/modules/alb/) module, use the `packages.deckhouse.io/application-alb` label with the name of an HTTPRoute from the templates instead, and for a ServiceAccount token, the `packages.deckhouse.io/application-serviceaccount` label with the name of a ServiceAccount from the templates. The web interface shows the credentials next to the endpoints with the description of this object. A label value can't be longer than 63 characters, so the name of the Ingress, HTTPRoute, or ServiceAccount must fit this limit.
- Instead of the label, you can set the endpoint description in the `packages.deckhouse.io/application-endpoint-description` annotation of the Secret. If both are set, the description of the Ingress, HTTPRoute, or ServiceAccount is used.
- The web interface shows the credentials only if the application has endpoints with a matching description. Endpoints without a description don't get credentials, so set a description in the annotation of the Ingress, HTTPRoute, or ServiceAccount instead of `"true"`.
- The `packages.deckhouse.io/initial-secret` label marks the Secret as one-time credentials. Deckhouse Platform creates such a Secret once and doesn't update it afterwards: it adds the `werf.io/resource-policy: skip-update` annotation to the Secret (an existing policy is kept, for example, `skip-delete` becomes `skip-delete,skip-update`). The admission policies let users change such a Secret, so the web interface can mark it as shown, and users can change the credentials. Without this label, the policies deny the change, and the credentials can be requested again. Deckhouse Platform checks only that the label is present, its value doesn't matter.

Put the login into the `login` key and the password into the `password` key. The web interface shows every key of `data` as a separate field named after the key, so don't store other data in this Secret.

In the web interface, endpoints with credentials get a button for getting the login and password. The credentials can be obtained only once: after showing them, the web interface adds the `packages.deckhouse.io/showed-credentials: "true"` annotation to the Secret and doesn't offer this Secret again. This restriction applies only to the web interface: users with permission to read Secrets in the namespace can still read the Secret.

Since Deckhouse Platform doesn't update a Secret with the `packages.deckhouse.io/initial-secret` label, changes to its template, including `data`, apply only when the Secret is created, for example, in a new instance.

Generate the credentials once and keep them unchanged: the web interface doesn't offer a Secret again even if its data changes. Don't generate the password in the template: functions such as `randAlphaNum` give a new value on every render (for details, refer to ["Known issues and troubleshooting"](nelm-annotations.html#known-issues-and-troubleshooting)). Instead, for example, generate it in a hook if the Secret doesn't exist yet, and otherwise take it from the snapshot of the existing Secret. Values set by hooks are kept only in memory, so the hook must restore the password after Deckhouse Platform restarts (for details, refer to ["Hooks"](hooks.html#values)).

### ServiceAccount token

To show a ServiceAccount token, add a Secret of the `kubernetes.io/service-account-token` type for the ServiceAccount from the [example above](#serviceaccount-endpoints):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: d8a-{{ .Application.Instance.Name }}-api-token
  labels:
    packages.deckhouse.io/application-instance: {{ .Application.Instance.Name }}
    packages.deckhouse.io/application-serviceaccount: d8a-{{ .Application.Instance.Name }}-api
    packages.deckhouse.io/initial-secret: "true"
  annotations:
    kubernetes.io/service-account.name: d8a-{{ .Application.Instance.Name }}-api
type: kubernetes.io/service-account-token
```

Kubernetes fills in the `ca.crt`, `namespace`, and `token` keys of this Secret: the CA certificate of the Kubernetes API, the namespace, and the ServiceAccount token. The web interface shows them as separate fields.

## Restrictions

Templates must follow the [Application constraints](concepts.html#application-constraints):

- Create only namespaced objects. Don't set `metadata.namespace`: DP creates the objects in the namespace of the Application, and `d8 package verify` reports an error for objects with this field (the `instance-namespace` rule).
- Don't create CustomResourceDefinition objects. DP doesn't install CRDs from the `crds/` directory of the chart.

By default, `d8 package verify` also requires that:

- Every Deployment and StatefulSet has a PodDisruptionBudget whose selector matches the labels of its Pods (the `pdb` rule).
- Every Deployment, StatefulSet, and DaemonSet has a VerticalPodAutoscaler with a policy for each container (the `vpa` rule). The VerticalPodAutoscaler kind is available only if the [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/) module is enabled, so render it under the `.Capabilities.APIVersions.Has "autoscaling.k8s.io/v1/VerticalPodAutoscaler"` condition, as the package skeleton does.
- Services refer to container ports by name in `targetPort` (the `service-port` rule).

You can lower the severity of these rules in `.pkglint.yaml` (for details, refer to ["Linting"](application-development.html#linting)).

## Replica limits

The user can change the number of replicas of a Deployment, StatefulSet, or ReplicaSet in the [`spec.resourceRequests`](../../reference/api/cr.html#application-v1alpha1-spec-resourcerequests) field of the Application. If the application can't run with an arbitrary number of replicas, add one of the following annotations to the workload in the templates:

- `packages.deckhouse.io/scale-disabled: "true"`: The number of replicas can't be changed.
- `packages.deckhouse.io/scale-down-disabled: "true"`: The number of replicas can't be lower than the number set in the templates.

The limits are counted from `spec.replicas` of the workload rendered from the templates (`1` if the field isn't set), not from the current number of replicas. For example, if the templates set `replicas: 2` and `scale-down-disabled`, the user can scale the workload to 4 replicas and back to 2, but not to 1. Only the `"true"` value enables a limit.

If a HorizontalPodAutoscaler from the templates manages the replicas of the workload, add the annotation to the HorizontalPodAutoscaler. Deckhouse Platform sets both `minReplicas` and `maxReplicas` of such a HorizontalPodAutoscaler to the requested number of replicas, and the annotations limit this number:

- `scale-disabled`: The number must be equal to both `minReplicas` and `maxReplicas`. If they differ, the number of replicas can't be changed.
- `scale-down-disabled`: The number can't be lower than `minReplicas` (`1` if the field isn't set).

Example:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: d8a-{{ .Application.Instance.Name }}-web
  annotations:
    packages.deckhouse.io/scale-down-disabled: "true"
spec:
  replicas: 2
  # ...
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: d8a-{{ .Application.Instance.Name }}-api
  annotations:
    packages.deckhouse.io/scale-down-disabled: "true"
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: d8a-{{ .Application.Instance.Name }}-api
  minReplicas: 2
  maxReplicas: 10
  # ...
```

If `spec.resourceRequests` requests a forbidden number of replicas, Deckhouse Platform doesn't apply the templates, and the objects of the application stay as they were. The Application itself is accepted, because Deckhouse Platform checks the limits when it renders the templates. The conditions of the Application get the `ScalingForbidden` reason, and the condition message names the workload, for example: `Deployment 'd8a-myapp-web' does not allow its replicas to go below 2`.

## Workload health

DP determines whether the application is running from its Deployments and StatefulSets only:

- The `Scaled` condition of the Application becomes `True` when all Deployments and StatefulSets rendered by the templates have rolled out and have the desired number of ready replicas. DaemonSets, Jobs, CronJobs, and Pods are not taken into account.
- A Deployment whose rollout exceeds `spec.progressDeadlineSeconds` sets `Scaled` to `False` with the `Degraded` reason.
- The first installation is considered complete (`Installed=True`) only after the manifests are applied and `Scaled` becomes `True`.

{% endraw %}
{% alert level="warning" %}
The [package](../../glossary.html#marketplace-package) must contain at least one Deployment or StatefulSet. Otherwise, the `Scaled` condition stays `Unknown`, and the application never reaches the `Installed` and `Ready` states.
{% endalert %}
