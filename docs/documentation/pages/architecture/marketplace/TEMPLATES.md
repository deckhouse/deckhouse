---
title: Templates
permalink: en/architecture/marketplace/templates.html
description: "Helm templates of an Application package: template values, object naming, container images and registry access, Ingress and application endpoints, labels, and workload health."
---

{% raw %}

The `templates/` directory of the package contains Helm templates. Deckhouse Platform (DP) renders them and applies the result with Nelm, so the templates can use [Nelm annotations](nelm-annotations.html) to control the deployment order, resource lifecycle, and readiness tracking. This page describes the data DP passes to the templates and the rules the templates must follow.

## Template values

DP renders the templates with three value roots:

| Root | Contents |
|---|---|
| `.Values` | Application values: settings with default values, internal values, and values set by hooks (see [how DP builds values](settings.html#how-dp-builds-values)). The values are not nested under the package name: a setting is available as `.Values.<SETTING_NAME>` |
| `.Application` | Instance and package parameters |
| `.Platform` | DP global parameters (read-only) |

### .Application

| Path | Description |
|---|---|
| `.Application.Instance.Name` | Instance name, that is, the name of the Application resource |
| `.Application.Instance.Namespace` | Namespace the application is installed into |
| `.Application.Package.Name` | Package name (the `name` field of `package.yaml`) |
| `.Application.Package.Version` | Package version |
| `.Application.Package.Images` | Container images of the package: a map of image names to image references with a digest (see [Container images](#container-images)) |
| `.Application.Package.Registry` | Access parameters of the repository the package is installed from (see [Container images](#container-images)) |
| `.Application.Settings` | Effective settings: `Application.spec.settings` with default values applied |

### .Platform

`.Platform` contains the DP global parameters. The following parameters are useful in application templates:

| Path | Description |
|---|---|
| `.Platform.applications.publicDomainTemplate` | DNS name template for application Ingresses (see [Ingress and HTTPS](#ingress-and-https)) |
| `.Platform.applications.ingressClass` | IngressClass for application Ingresses. Default: `nginx` |
| `.Platform.applications.https.mode` | HTTPS mode for applications: `CertManager` (default), `CustomCertificate`, `Disabled`, or `OnlyInURI` |
| `.Platform.applications.https.certManager.clusterIssuerName` | cert-manager ClusterIssuer for application certificates. Default: `letsencrypt` |
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
- The `d8a-` prefix is reserved for applications: the `d8a-prefix.deckhouse.io` admission policy forbids users to create, change, and delete objects whose names start with `d8a-`. Deleting Pods is allowed.
- DP adds the same prefix to the names of objects that hooks create, patch, or delete. Therefore, a hook can address an object rendered by the templates by its suffix only (see [Hooks](hooks.html#object-names)).

Don't use `.Release.Name` in object names: the Helm release of an application is named `<NAMESPACE>.<INSTANCE_NAME>`, and the dot is not allowed in the names of many resource kinds.

Keep suffixes short: the instance name can be up to 24 characters long, and the full object name must fit the Kubernetes limits (see [Naming constraints](concepts.html#naming-constraints)).

## Labels and object protection

DP adds the following labels to the metadata of every rendered object:

| Label | Value |
|---|---|
| `heritage` | `deckhouse` |
| `packages.deckhouse.io/package` | Package name |
| `packages.deckhouse.io/instance` | Instance name |
| `health.deckhouse.io/package` | `<NAMESPACE>.<INSTANCE_NAME>` |

The labels are added only to the metadata of the rendered objects, not to Pod templates. Define the labels used in Pod selectors in the templates yourself.

Objects with the `heritage: deckhouse` label are protected by the `label-objects.deckhouse.io` admission policy, and objects with the `d8a-` name prefix are protected by the `d8a-prefix.deckhouse.io` policy. As a result:

- Users can't change or delete objects rendered by the templates. To change them manually, for example, when debugging, switch the application to [maintenance mode](lifecycle.html#maintenance-mode).
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

`.Application.Package.Registry` contains the following fields:

| Field | Description |
|---|---|
| `repository` | Repository address from the PackageRepository resource |
| `dockercfg` | Base64-encoded Docker configuration for accessing the repository. If the PackageRepository uses `login` and `password`, DP builds the configuration from them |
| `scheme` | Protocol for accessing the repository: `HTTP` or `HTTPS` |
| `ca` | CA certificate of the repository, if specified |

## Ingress and HTTPS

Build host names and TLS settings of application Ingresses from the `.Platform.applications` parameters:

- `publicDomainTemplate` — DNS name template with two `%s` placeholders. The first one is replaced with the instance name, the second one with the namespace. For example, with the `%s.%s.apps.example.com` template, the `grafana` instance in the `monitoring` namespace gets the `grafana.monitoring.apps.example.com` host name. If the parameter is not set, don't create Ingresses.
- `ingressClass` — IngressClass of the Ingresses.
- `https.mode` — HTTPS mode. In the `CertManager` mode, request a certificate from the ClusterIssuer specified in `https.certManager.clusterIssuerName`. In the `Disabled` and `OnlyInURI` modes, don't configure TLS on the Ingress.

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

The package skeleton created by `d8 package bootstrap app` contains helpers for these parameters in `templates/_helpers/`: `public_domain`, `ingress_class`, `https_mode`, `https_ingress_tls_enabled`, and `https_cert_manager_cluster_issuer_name`.

## Application endpoints

To show the addresses of an Ingress in the `status.urls` field of the Application (for example, the web interface shows them as links to the application), add the `packages.deckhouse.io/application-endpoint-description` annotation to the Ingress:

- The annotation value is the endpoint description. The `"true"` value adds the endpoint without a description, and the `"false"` value excludes the Ingress.
- DP builds a URL for every `host` and `path` pair of `spec.rules`. The scheme is `https` if the host is listed in `spec.tls`, and `http` otherwise. A rule without paths gives the `<SCHEME>://<HOST>/` URL. Rules without `host` are skipped.
- Only Ingresses of the `networking.k8s.io` API group rendered by the templates are taken into account.
- DP updates `status.urls` after every successful apply of the templates.

Example status of an Application with the Ingress from the previous section:

```yaml
status:
  urls:
    - url: https://myapp.my-namespace.apps.example.com/
      description: Web UI
```

## Restrictions

Templates must follow the [Application constraints](concepts.html#application-constraints):

- Create only namespaced objects. Don't set `metadata.namespace`: DP creates the objects in the namespace of the Application, and `d8 package verify` reports an error for objects with this field (the `instance-namespace` rule).
- Don't create CustomResourceDefinition objects. DP doesn't install CRDs from the `crds/` directory of the chart.

By default, `d8 package verify` also requires that:

- every Deployment and StatefulSet has a PodDisruptionBudget whose selector matches the labels of its Pods (the `pdb` rule);
- every Deployment, StatefulSet, and DaemonSet has a VerticalPodAutoscaler with a policy for each container (the `vpa` rule). The VerticalPodAutoscaler kind is available only if the `vertical-pod-autoscaler` module is enabled, so render it under the `.Capabilities.APIVersions.Has "autoscaling.k8s.io/v1/VerticalPodAutoscaler"` condition, as the package skeleton does;
- Services refer to container ports by name in `targetPort` (the `service-port` rule).

You can lower the severity of these rules in `.pkglint.yaml` (see [Linting](application-development.html#linting)).

## Workload health

DP determines whether the application is running from its Deployments and StatefulSets only:

- The `Scaled` condition of the Application becomes `True` when all Deployments and StatefulSets rendered by the templates have rolled out and have the desired number of ready replicas. DaemonSets, Jobs, CronJobs, and Pods are not taken into account.
- A Deployment whose rollout exceeds `spec.progressDeadlineSeconds` sets `Scaled` to `False` with the `Degraded` reason.
- The first installation is considered complete (`Installed=True`) only after the manifests are applied and `Scaled` becomes `True`.

{% endraw %}
{% alert level="warning" %}
The package must contain at least one Deployment or StatefulSet. Otherwise, the `Scaled` condition stays `Unknown`, and the application never reaches the `Installed` and `Ready` states.
{% endalert %}
