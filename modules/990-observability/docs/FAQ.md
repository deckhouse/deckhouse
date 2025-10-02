---
title: "Observability Module: FAQ"
description: "FAQ for the Observability module"
menuTitle: "FAQ"
---

## How to convert existing dashboards from GrafanaDashboardDefinition

To migrate from the old dashboard format (`GrafanaDashboardDefinition`) to the new ones (`ObservabilityDashboard`, `ClusterObservabilityDashboard`), you need to manually adapt the manifests. Note the following differences:

| Old Format                                   | New Format                                                                                                                                     |     |
| -------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- | --- |
| `spec.folder`                                | This field is removed. The folder is now specified using the annotation: `observability.deckhouse.io/category`                                 |     |
| Dashboard title is taken from the JSON title | The title is set via the annotation: `observability.deckhouse.io/title`. If the annotation is missing, the `title` field from the JSON is used |     |

### Conversion example

Old format:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: example-dashboard
spec:
  folder: "Apps"
  json: '{
    "title": "Example Dashboard",
    ...
  }'
```

New format (`ObservabilityDashboard`):

```yaml
apiVersion: observability.deckhouse.io/v1alpha1
kind: ObservabilityDashboard
metadata:
  name: example-dashboard
  namespace: my-namespace
  annotations:
    metadata.deckhouse.io/category: "Apps"
    metadata.deckhouse.io/title: "Example Dashboard"
spec:
  definition: |
    {
      "title": "Example Dashboard",
      ...
    }
```

New format (`ClusterObservabilityDashboard`):

```yaml
apiVersion: observability.deckhouse.io/v1alpha1
kind: ClusterObservabilityDashboard
metadata:
  name: example-dashboard
  annotations:
    metadata.deckhouse.io/category: "Apps"
    metadata.deckhouse.io/title: "Example Dashboard"
spec:
  definition: |
    {
      "title": "Example Dashboard",
      ...
    }
```

## How to grant access to metrics and dashboards in a specific namespace

To grant access to metrics and dashboards in a specific namespace, you need to create a `ClusterRole` and `RoleBinding` that define the user's permissions. Access to metrics and dashboards is granted separately:

- Metrics — access is checked via the `get` permission on the `metrics.observability.deckhouse.io` resource.
- Dashboards — access is checked via the following permissions on the `observabilitydashboards.observability.deckhouse.io` resource:
  - `get` — view dashboards;
  - `create` — create, update, and delete dashboards.

### Example of ClusterRole and RoleBinding for read-only access

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observability-viewer
rules:
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["metrics", "observabilitydashboards"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: bind-observability-viewer
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: observability-viewer
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
```

### Example of ClusterRole and RoleBinding for read and write access

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observability-editor
rules:
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["metrics", "observabilitydashboards"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["observabilitydashboards"]
    verbs: ["create", "update", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: bind-observability-editor
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: observability-editor
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
```

## How to grant access to system metrics and dashboards

To grant access to system metrics and dashboards, you need to create a `ClusterRole` and `ClusterRoleBinding` that define the user's permissions. Access to metrics and dashboards is granted separately:

- Metrics — access is checked via the `get` permission on the `clustermetrics.observability.deckhouse.io` resource.
- Dashboards — access is checked via the following permissions on the `clusterobservabilitydashboards.observability.deckhouse.io` resource:
  - `get` — view dashboards;
  - `create` — create, update, and delete dashboards.

### Example of ClusterRole and ClusterRoleBinding for read-only access

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observability-cluster-viewer
rules:
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["clustermetrics", "clusterobservabilitydashboards"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: bind-observability-cluster-viewer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: observability-cluster-viewer
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
```

### Example of ClusterRole and ClusterRoleBinding for read and write access

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observability-cluster-editor
rules:
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["clustermetrics", "clusterobservabilitydashboards"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["clusterobservabilitydashboards"]
    verbs: ["create", "update", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: bind-observability-cluster-editor
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: observability-cluster-editor
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
```

## How to grant full access to all metrics and dashboards

To grant full access to all metrics and dashboards in Deckhouse, create a `ClusterRole` with all necessary permissions and bind it to a user or group via `ClusterRoleBinding`.

### Example of ClusterRole

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observability-admin
rules:
  - apiGroups: ["observability.deckhouse.io"]
    resources:
      - metrics
      - clustermetrics
      - observabilitydashboards
      - clusterobservabilitydashboards
      - clusterobservabilitypropagateddashboards
    verbs: ["get", "list", "watch", "create", "update", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: bind-observability-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: observability-admin
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
```

> You can also use the built-in `cluster-admin` role, but it should be used with caution, as it grants full access to all cluster resources.

## How to grant access using RBAC 2.0

If the [experimental role model](https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/user-authz/#experimental-role-model) is enabled, permissions are assigned using `UserRole` and `ClusterUserRole` resources.

### Example of access to metrics and dashboards in a specific namespace

To grant a user access to the `myapp` namespace with permission to view metrics and dashboards, use the following manifest:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: myapp-developer
  namespace: myapp
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:use:role:user
  apiGroup: rbac.authorization.k8s.io
```

> This example grants broader permissions beyond just access to dashboards and metrics. See the [user-authz module documentation](https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/user-authz/#use-roles) for details on this role.

## How to read metrics outside of cluster

Please perform following steps to get access to metrics:

1. Enable external metrics access. Enable [spec.settings.externalMetricsAccess](/products/kubernetes-platform/modules/observability/stable/configuration.html#parameters-externalmetricsaccess) in observability module settings.
2. Create `ServiceAccount` for requests authorization:

   ```yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: metrics-access
      namespace: my-namespace
    ---
    apiVersion: v1
    kind: Secret
    metadata:
      name: metrics-access
      annotations:
        kubernetes.io/service-account.name: metrics-access
    type: kubernetes.io/service-account-token
   ```

3. Add a `Role` and `RoleBinding` to provide read metrics permission for a created `ServiceAccount`:

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: Role
   metadata:
     namespace: my-namespace
     name: metrics-access
   rules:
     - apiGroups: ["observability.deckhouse.io"]
       resources: ["metrics"]
       verbs: ["get", "watch", "list"]
     - apiGroups: [""]
       resources: ["namespaces"]
       verbs: ["get", "watch", "list"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: RoleBinding
   metadata:
     name: metrics-access
     namespace: my-namespace
   subjects:
     - kind: ServiceAccount
       name: metrics-access
       namespace: my-namespace
   roleRef:
     kind: Role
     name: metrics-access
     apiGroup: rbac.authorization.k8s.io
   ```

4. Get authorization token. A `Secret` containing an authorization token was created along with `ServiceAccount`.
   Token stored in `Secret` is saved with base64 encoding. This token may be used to access metrics.
   Use following command to get the token:

   ```bash
     kubectl -n my-namespace get secret metrics-access -ojsonpath='{ .data.token }' | base64 -d
   ```

   This token will be required on the next step to set up Grafana datasource.

5. Set up Grafana to access metrics
   You need to create a Prometheus datasource in external Grafana using following parameters:

   - `Name` - custom data source name.
   - `URL` - external metrics url. Use following URL to access metrics `https://observability.%publicDomainTemplate%/<prefix>/` with:
     - use `/metrics/` `<prefix>` to access main Prometheus instance
     - use `/metrics/longterm` `<prefix>` to access longterm Prometheus.
   - `HTTP Headers`:
     - `Header`: Authorization
     - `Value`: Bearer <TOKEN_VALUE>, with token obtained from `metrics-access` `Secret` in the previous step.

## How to write metrics outside of cluster

Please perform following steps to get access to metrics:

1. Enable external metrics access. Enable [spec.settings.externalMetricsAccess](/products/kubernetes-platform/modules/observability/stable/configuration.html#parameters-externalmetricsaccess) in observability module settings.
2. Create `ServiceAccount` for requests authorization.

   ```yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: metrics-access
      namespace: my-namespace
    ---
    apiVersion: v1
    kind: Secret
    metadata:
      name: metrics-access
      annotations:
        kubernetes.io/service-account.name: metrics-access
    type: kubernetes.io/service-account-token
   ```

3. Add a `Role` and `RoleBinding` to provide read metrics permission for a created `ServiceAccount`:

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: Role
   metadata:
     namespace: my-namespace
     name: metrics-access
   rules:
     - apiGroups: ["observability.deckhouse.io"]
       resources: ["metrics"]
       verbs: ["create"]
     - apiGroups: [""]
       resources: ["namespaces"]
       verbs: ["get", "watch", "list"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: RoleBinding
   metadata:
     name: metrics-access
     namespace: my-namespace
   subjects:
     - kind: ServiceAccount
       name: metrics-access
       namespace: my-namespace
   roleRef:
     kind: Role
     name: metrics-access
     apiGroup: rbac.authorization.k8s.io
   ```

4. Get authorization token. A `Secret` containing an authorization token was created along with `ServiceAccount`.
   Token stored in `Secret` is saved with base64 encoding. This token may be used to access metrics.
   Use following command to get the token:

   ```bash
     kubectl -n my-namespace get secret metrics-access -ojsonpath='{ .data.token }' | base64 -d
   ```

   This token will be required on the next step to set up Grafana datasource.

5. Send metrics using Prometheus Remote-Write [V1](https://prometheus.io/docs/specs/prw/remote_write_spec/) or [V2](https://prometheus.io/docs/specs/prw/remote_write_spec_2_0/) messages:
   - `URL`: `https://observability.%publicDomainTemplate%/api/v1/write`. [publicDomainTemplate details](https://deckhouse.io/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters-modules-publicdomaintemplate).
   - `HTTP Headers`:
     - `Header`: Authorization
     - `Value`: Bearer <TOKEN_VALUE>, где токен это токен полученный из `Secret`-а `metrics-access` на предыдущем шаге.

## How to read cluster metrics outside of cluster

Please perform following steps to get access to metrics:

1. Enable external metrics access. Enable [spec.settings.externalMetricsAccess](/products/kubernetes-platform/modules/observability/stable/configuration.html#parameters-externalmetricsaccess) in observability module settings.
2. Create `ServiceAccount` for requests authorization:

   ```yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: cluster-metrics-access
    ---
    apiVersion: v1
    kind: Secret
    metadata:
      name: cluster-metrics-access
      annotations:
        kubernetes.io/service-account.name: cluster-metrics-access
    type: kubernetes.io/service-account-token
   ```

3. Add a `Role` and `RoleBinding` to provide read metrics permission for a created `ServiceAccount`:

   ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRole
    metadata:
      name: observability-cluster-metrics-viewer
    rules:
      - apiGroups: ["observability.deckhouse.io"]
        resources: ["clustermetrics"]
        verbs: ["get", "list", "watch"]
    ---
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: bind-observability-cluster-metrics-viewer
    subjects:
      - kind: ServiceAccount
        name: cluster-metrics-access
        namespace: default
    roleRef:
      kind: ClusterRole
      name: observability-cluster-metrics-viewer
      apiGroup: rbac.authorization.k8s.io
   ```

4. Get authorization token. A `Secret` containing an authorization token was created along with `ServiceAccount`.
   Token stored in `Secret` is saved with base64 encoding. This token may be used to access metrics.
   Use following command to get the token:

   ```bash
     kubectl -n my-namespace get secret cluster-metrics-access -ojsonpath='{ .data.token }' | base64 -d
   ```

   This token will be required on the next step to set up Grafana datasource.

5. Set up Grafana to access metrics
   You need to create a Prometheus datasource in external Grafana using following parameters:

   - `Name` - custom data source name.
   - `URL` - external metrics url. Use following URL to access metrics `https://observability.%publicDomainTemplate%/<prefix>/` with:
     - use `/metrics/` `<prefix>` to access main Prometheus instance
     - use `/metrics/longterm` `<prefix>` to access longterm Prometheus.
   - `HTTP Headers`:
     - `Header`: Authorization
     - `Value`: Bearer <TOKEN_VALUE>, with token obtained from `cluster-metrics-access` `Secret` in the previous step.
