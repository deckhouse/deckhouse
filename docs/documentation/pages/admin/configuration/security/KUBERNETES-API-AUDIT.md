---
title: Kubernetes API event audit
permalink: en/admin/configuration/security/events/kubernetes-api-audit.html
description: "Configure Kubernetes API audit logging in Deckhouse Kubernetes Platform. API server event tracking, audit policy configuration, and security event analysis."
---

The [Kubernetes auditing](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/) feature allows you to track requests
to the API server and analyze events occurring in the cluster.
Auditing can be useful for troubleshooting unexpected behavior and for meeting security requirements.

Kubernetes supports audit configuration via the [Audit policy](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy) mechanism,
which allows you to define logging rules for target operations.
By default, audit results are written to the `/var/log/kube-audit/audit.log` file.

## Built-in audit policies

Deckhouse Kubernetes Platform (DKP) creates a default set of audit policies that log:

- Create, update, and delete operations on resources.
- Requests made on behalf of service accounts from the `kube-system` and `d8-*` system namespaces.
- Access to resources in the `kube-system` and `d8-*` system namespaces.

These policies are enabled by default.
To disable them, set the [`basicAuditPolicyEnabled`](/modules/control-plane-manager/configuration.html#parameters-apiserver-basicauditpolicyenabled) parameter to `false`.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    apiserver:
      auditPolicyEnabled: true
      basicAuditPolicyEnabled: false
```

## Configuring a custom audit policy

To create an advanced Kubernetes API audit policy, follow these steps:

1. Enable the [`auditPolicyEnabled`](/modules/control-plane-manager/configuration.html#parameters-apiserver-auditpolicyenabled) parameter
   in the `control-plane-manager` module configuration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     version: 1
     settings:
       apiserver:
         auditPolicyEnabled: true
   ```

1. Create the `kube-system/audit-policy` Secret containing the policy YAML file encoded in Base64:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: audit-policy
     namespace: kube-system
   data:
     audit-policy.yaml: <Base64>
   ```

   Example `audit-policy.yaml` content with a minimal working configuration:

   ```yaml
   apiVersion: audit.k8s.io/v1
   kind: Policy
   rules:
   - level: Metadata
     omitStages:
     - RequestReceived
   ```

   For more information on possible contents of `audit-policy.yaml`, refer to the following sources:

   - [Kubernetes official documentation](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy)
   - [GCE helper script code snippet](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862)

## How to generate the contents of the `audit-policy.yaml` file

[Kubernetes Documentation](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy)
[Policy Resource Field Structure](https://kubernetes.io/docs/reference/config-api/apiserver-audit.v1/#audit-k8s-io-v1-PolicyRule)

A Kubernetes audit policy is defined in a YAML file and consists of a set of rules that determine which events and at what level of detail will be logged.
The file has the following structure:

```yaml
apiVersion: audit.k8s.io/v1   # API version used for the audit policy
kind: Policy                  # Resource type — constant Policy
rules:                          # Set of rules for auditing
  - level:                      # Log detail level. Required field
    users:                      # List of users whose actions are logged
    userGroups:                 # User groups (e.g., system:serviceaccounts)
    verbs:                      # Actions/operations that are logged (create, update, delete, etc.)
    resources:                  # Kubernetes resources for which the rule applies
    namespaces:                  # Namespaces that the rule covers
```

The `rules` array describes the audit rules.
Each rule contains the following fields:

**level** — The detail level of the logged event.
Possible values ​​(from most to least detailed):
- **None** — do not log at all
- **Metadata** — only request metadata (who, when, what, where; without object contents)
- **Request** — also stores the request body (for update requests only)
- **RequestResponse** — stores both the request body and the response content

**users** — a list of usernames the rule applies to (e.g., `["admin"]`)
For service accounts, the name typically looks like `system:serviceaccount:<namespace>:<serviceaccount-name>`.

For regular users, the name depends on the authentication system settings.

For `deckhouse.io/v1` `Users` objects, `email` is used as the name.

**userGroups** — user groups (e.g., `["system:authenticated"]`)
After authentication, kube-apiserver assigns a list of groups to each user (e.g., all authenticated users are part of `system:authenticated`, service accounts are in additional groups).  
If a request comes from a user who is a member of at least one of the groups specified in userGroups, the rule is applied to that request.

Kubernetes built-in groups:
- `system:authenticated` — everyone who has been authenticated.
- `system:unauthenticated` — requests from anonymous users.
- `system:serviceaccounts` — all service accounts from all namespaces.
- `system:serviceaccounts:<namespace>` — service accounts in a specific namespace.

**verbs** — List of API operations (get, list, create, delete, etc.)

**resources** — Array of target resources:
- **group** — API group (e.g., "apps", "batch", """ for core)
- **resources** — Resource types (e.g., "[pods", "deployments"])
A full list of resources and their groups can be obtained using the `kubectl api-resources` command

**namespaces** — Array of namespaces where the rule applies

**nonResourceURLs** — A set of URL paths to audit. The * character is allowed, but only as a full, final step of the path.
Examples:
- `/metrics`  — log requests to apiserver metrics
- `/healthz*` — log all health requests

### Examples

#### Log all requests from all authenticated users

```yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: Metadata
    userGroups: ["system:authenticated"]
```

{% alert level="warning" %}
**Warning!**   
Not recommended for production environments.  
A high event flow may result in increased load on the control-plane disk subsystem of cluster nodes.
{% endalert %}

Example log entry:

```json
{
  "stage": "ResponseComplete",
  "requestURI": "/healthz",
  "verb": "get",
  "user": {
    "username": "kube-apiserver-kubelet-client",
    "groups": [
      "kubeadm:cluster-admins",
      "system:authenticated"    //  <- everyone who has been authenticated
    ]
  },
  ...
}
```

#### Logging all requests not from service accounts

```yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: Metadata
    userGroups: ["system:authenticated"]
    users: []
    omitStages: []
    # Exclude service accounts via a deny rule
  - level: None
    userGroups: ["system:serviceaccounts"]
```

Example log entry:

```json
{
  "user": {
    "username": "user@example.com",
    "groups": ["admins","system:authenticated"]
  },
  ...
}
```

#### Logging all operations with standard resources

```yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: Metadata
    resources:
      - group: "apps"
        resources: ["deployments", "daemonsets", "statefulsets", "replicasets"]
      - group: "batch"
        resources: ["jobs", "cronjobs"]
      - group: ""
        resources: ["*"]
```

## Working with the audit log file

On DKP master nodes, it is assumed that a log collection tool (`log-shipper`, `promtail`, or `filebeat`) is installed
to monitor the `/var/log/kube-audit/audit.log` file.

The log rotation settings for this file are predefined and cannot be changed:

- Maximum file size: 1000 MB.
- Maximum retention period: 30 days.

Depending on the policy configuration and the volume of requests to the API server,
the number of log entries can be very large.
In such cases, the retention period may be reduced to less than 30 minutes.

{% alert level="warning" %}
Unsupported options or typos in the configuration file may cause the API server to fail to start.
{% endalert %}

If the API server fails to start, take the following steps:

1. Manually remove the `--audit-log-*` parameters from the `/etc/kubernetes/manifests/kube-apiserver.yaml` manifest.
1. Restart the API server with the following command:

   ```shell
   docker stop $(docker ps | grep kube-apiserver- | awk '{print $1}')
   
   # Alternative option (depending on the CRI in use).
   crictl stopp $(crictl pods --name=kube-apiserver -q)
   ```

1. After restarting, fix the Secret or delete it with the following command:

   ```shell
   d8 k -n kube-system delete secret audit-policy
   ```

## Redirecting the audit log file to stdout

By default, the audit log is saved to the `/var/log/kube-audit/audit.log` file on master nodes.
If necessary, you can redirect its output to the `kube-apiserver` process stdout instead of a file
by setting the [`apiserver.auditLog.output`](/modules/control-plane-manager/configuration.html#parameters-apiserver-auditlog-output) parameter in the [`control-plane-manager`](/modules/control-plane-manager/) module to `Stdout`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    apiserver:
      auditPolicyEnabled: true
      auditLog:
        output: Stdout
```

In this case, the log will be available in the `kube-apiserver` container stdout.

Then, using the [built-in DKP logging mechanism](../../../configuration/logging/delivery.html),
you can configure log collection and forwarding to your own security system.
