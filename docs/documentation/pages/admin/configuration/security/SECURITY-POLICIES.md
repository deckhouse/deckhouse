---
title: Security policies
permalink: en/admin/configuration/security/policies.html
description: "Configure security policies in Deckhouse Kubernetes Platform using Gatekeeper and Pod Security Standards. Policy enforcement, compliance, and cluster security management."
---

Deckhouse Kubernetes Platform (DKP) lets you manage application security in the cluster using a set of admission policies.
These are rules that apply to objects (such as Pods and Services) at the time of their creation and modification in the cluster (but not during their operation), based on the information provided in their manifests. These policies are designed to formalize the parameters that are permitted or prohibited in object manifests. Support for admission policies in the DKP cluster is implemented using the [`admission-policy-engine`](/modules/admission-policy-engine/) module.

In the DKP policies are divided into three categories:

- [Pod Security Standards](#applying-pod-security-standards): Policies that comply with the relevant [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/).
- [Operational policies](#operational-policies): Policies for creating additional requirements for objects by validating the values of parameters that are **not directly related** to security (for example, a list of allowed prefixes for container images, an image download policy, a list of required container images, etc.).
- [Security policies](#security-policies): Policies for creating additional requirements on objects by validating the values of security-related parameters (for example, container access to the host’s IPC or PID namespaces, privilege lists for containers, etc.).

{% alert level="info" %}
These policies complement each other. If multiple policies are applied to a single namespace, objects are validated against each of them. If even one policy is violated, the object will not be created.
{% endalert %}

## How validation failure messages are displayed

Depending on how pods are created, there are differences in how the API generates messages regarding validation failures (violations of established policies):

- If a pod is created directly, the validation error is returned in the API response indicating a validation failure (policy violation).
- If pods are created via Deployment, the required number of ReplicaSets is created, which in turn attempt to create the pods. In this case, the validation error is not returned in the API response but is displayed in the namespace events or the corresponding ReplicaSet events.

## Pod validation when policies are modified or added

For all three policy categories (Pod Security Standards, operational, and security policies), there is no provision for automatically recreating existing pods when changing existing policies or adding new ones. Pods that existed prior to changes being made to the policy in use or prior to a new policy being added will continue to run until they are restarted. Upon restart, they will be validated against the new rules.

In DKP, there are alerts (ClusterObservabilityAlert resources) for such cases, notifying you of pods in the namespace that violate policies after an existing policy is modified or a new one is added.

To get a list of alerts, use the command:

```bash
d8 k get clusterobservabilityalerts
```

Output example:

<!-- markdownlint-disable MD031 -->
```console
NAME                                                  SEVERITY   STATUS   DURATION   SUMMARY                          AGE
SecurityPolicyViolation-f3a77d1dd2175402-1777370195   1          Firing   5h         Alerting PrometheusUnavailable   5h1m
OperationPolicyViolation-9b21d0c871796913-1777370435  1          Firing   6h         Alerting PrometheusUnavailable   6h1m
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

To view information about a specific alert, use the following command:

```bash
d8 k get clusterobservabilityalert OperationPolicyViolation-9b21d0c871796913-1777370435 -oyaml
```

{% offtopic title="Example of an alert for a violation of the Pod Security Standards policy..." %}

```yaml
kind: ClusterObservabilityAlert
apiVersion: alerts.observability.deckhouse.io/v1alpha1
metadata:
  name: PodSecurityStandardsViolation-91e71759e048a397-1777369535
  resourceVersion: "7454828154578800069"
  creationTimestamp: 2026-04-28T09:45:35Z
  labels:
    d8_component: gatekeeper
    d8_module: admission-policy-engine
    prometheus: deckhouse
alert:
  labels:
    alertname: PodSecurityStandardsViolation
    d8_component: gatekeeper
    d8_module: admission-policy-engine
    prometheus: deckhouse
    severity_level: "3"
  annotations:
    description: |-
      You have configured [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/), and one or more running pods are violating these standards.

      To identify violating pods:

      - Run the following Prometheus query:

        ```prometheus
        count by (violating_namespace, violating_name, violation_msg) (
          d8_gatekeeper_exporter_constraint_violations{
            violation_enforcement="deny",
            violating_namespace=~".*",
            violating_kind="Pod",
            source_type="PSS"
          }
        )
        ```

      - Alternatively, check the admission-policy-engine Grafana dashboard.
    plk_markup_format: markdown
    plk_protocol_version: "1"
    summary: At least one pod violates the configured cluster pod security standards.
  expr: (count(d8_gatekeeper_exporter_constraint_violations{source_type="PSS",violating_kind="Pod",violating_namespace=~".*",violation_enforcement="deny"}))
    > 0
  created_by: observability
  rule_group_name: admission-policy-engine-audit-0
status:
  alertStatus: Firing
  silencedBy: []
  startsAt: 2026-04-28T09:45:35Z
  resolvedAt: null
  duration: 20h40m1.015261771s
```

{% endofftopic %}

{% offtopic title="Example of an alert for a violation of an operational policy..." %}

```yaml
kind: ClusterObservabilityAlert
apiVersion: alerts.observability.deckhouse.io/v1alpha1
metadata:
  name: OperationPolicyViolation-9b21d0c871796913-1777370435
  resourceVersion: "7454831929456594373"
  creationTimestamp: 2026-04-28T10:00:35Z
  labels:
    d8_component: gatekeeper
    d8_module: admission-policy-engine
    prometheus: deckhouse
alert:
  labels:
    alertname: OperationPolicyViolation
    d8_component: gatekeeper
    d8_module: admission-policy-engine
    prometheus: deckhouse
    severity_level: "3"
  annotations:
    description: >-
      You have configured operation policies for the cluster, and one or more
      existing objects are violating these policies.


      To identify violating objects:


      - Run the following Prometheus query:

        ```prometheus
        count by (violating_namespace, violating_kind, violating_name, violation_msg) (
          d8_gatekeeper_exporter_constraint_violations{
            violation_enforcement="deny",
            source_type="OperationPolicy"
          }
        )
        ```

      - Alternatively, check the admission-policy-engine Grafana dashboard.
    plk_markup_format: markdown
    plk_protocol_version: "1"
    summary: At least one object violates the configured cluster operation policies.
  expr: (count(d8_gatekeeper_exporter_constraint_violations{source_type="OperationPolicy",violation_enforcement="deny"}))
    > 0
  created_by: observability
  rule_group_name: admission-policy-engine-audit-0
status:
  alertStatus: Firing
  silencedBy: []
  startsAt: 2026-04-28T10:00:35Z
  resolvedAt: null
  duration: 20h23m41.023025059s
```

{% endofftopic %}

{% offtopic title="Example of an alert for a security policy violation..." %}

```yaml
kind: ClusterObservabilityAlert
apiVersion: alerts.observability.deckhouse.io/v1alpha1
metadata:
  name: SecurityPolicyViolation-f3a77d1dd2175402-1777370195
  resourceVersion: "7454830922622307781"
  creationTimestamp: 2026-04-28T09:56:35Z
  labels:
    d8_component: gatekeeper
    d8_module: admission-policy-engine
    prometheus: deckhouse
alert:
  labels:
    alertname: SecurityPolicyViolation
    d8_component: gatekeeper
    d8_module: admission-policy-engine
    prometheus: deckhouse
    severity_level: "3"
  annotations:
    description: >-
      You have configured security policies for the cluster, and one or more
      existing objects are violating these policies.


      To identify violating objects:


      - Run the following Prometheus query:

        ```prometheus
        count by (violating_namespace, violating_kind, violating_name, violation_msg) (
          d8_gatekeeper_exporter_constraint_violations{
            violation_enforcement="deny",
            source_type="SecurityPolicy"
          }
        )
        ```

      - Alternatively, check the admission-policy-engine Grafana dashboard.
    plk_markup_format: markdown
    plk_protocol_version: "1"
    summary: At least one object violates the configured cluster security policies.
  expr: (count(d8_gatekeeper_exporter_constraint_violations{source_type="SecurityPolicy",violation_enforcement="deny"}))
    > 0
  created_by: observability
  rule_group_name: admission-policy-engine-audit-0
status:
  alertStatus: Firing
  silencedBy: []
  startsAt: 2026-04-28T09:56:35Z
  resolvedAt: null
  duration: 20h29m21.015479019s
```

{% endofftopic %}

## Applying Pod Security Standards

DKP supports three security policy levels:

- `privileged`: An unrestricted policy with the broadest possible permissions.
- `baseline`: A minimally restrictive policy that prevents the most well-known and common privilege escalation techniques.
  Allows the use of a standard (minimally specified) Pod configuration.
- `restricted`: A highly restrictive policy with the strictest requirements for Pods.

{% alert level="info" %}
In the Deckhouse Kubernetes Platform, these policies are implemented using Gatekeeper and enforced by the admission controllers of the `admission-policy-engine` module, rather than the Kubernetes [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/) controller. Only the policy descriptions are taken from Kubernetes.
{% endalert %}

### Default policy

The default policy is determined as follows:

- In DKP versions prior to v1.55, the default policy is `privileged`.
- Starting from DKP v1.55, the default policy is `baseline`.

{% alert level="info" %}
When upgrading DKP to v1.55 or later, the default policy will not change automatically.
{% endalert %}

### Assigning a policy

You can assign a policy in the following ways:

- Globally, using the [`settings.podSecurityStandards.defaultPolicy`](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-defaultpolicy) parameter of the [`admission-policy-engine`](/modules/admission-policy-engine/) module.
- Per namespace, using the `security.deckhouse.io/pod-policy=<POLICY_NAME>` label.

  Example command to assign the `restricted` policy to all Pods in the `my-namespace` namespace:

  ```shell
  d8 k label ns my-namespace security.deckhouse.io/pod-policy=restricted
  ```

### Enforcement modes

Allowed policy enforcement modes:

- `deny`: Blocks actions from being executed.
- `dryrun`: Does not affect execution and used for debugging.
  Event information can be viewed in Grafana or in the console using `kubectl`.
- `warn`: Works like `dryrun` but also displays a warning with the reason the action would have been denied in `deny` mode.

By default, Pod Security Standards policies in DKP are enforced in `deny` mode.
In this mode, application Pods that do not comply with the policies cannot be run in the cluster.

As with policy assignment, enforcement mode can be set:

- Globally, using the [`settings.podSecurityStandards.enforcementAction`](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-enforcementaction) parameter of the [`admission-policy-engine`](/modules/admission-policy-engine/) module.
- Per namespace, using the `security.deckhouse.io/pod-policy-action=<POLICY_ACTION>` label.

  Example command to set the `warn` mode for all Pods in the `my-namespace` namespace:

  ```shell
  d8 k label ns my-namespace security.deckhouse.io/pod-policy-action=warn
  ```

### Extending a policy

You can extend the `baseline` and `restricted` policies using Gatekeeper templates
by adding extra checks to the existing ones.

To extend a policy:

1. Create a validation template using a `ConstraintTemplate`.
1. Apply the template to the `baseline` or `restricted` policy.

Example template for validating the container image repository address:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8sallowedrepos
spec:
  crd:
    spec:
      names:
        kind: K8sAllowedRepos
      validation:
        openAPIV3Schema:
          type: object
          properties:
            repos:
              type: array
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package d8.pod_security_standards.extended

        violation[{"msg": msg}] {
          container := input.review.object.spec.containers[_]
          satisfied := [good | repo = input.parameters.repos[_] ; good = startswith(container.image, repo)]
          not any(satisfied)
          msg := sprintf("container <%v> has an invalid image repo <%v>, allowed repos are %v", [container.name, container.image, input.parameters.repos])
        }

        violation[{"msg": msg}] {
          container := input.review.object.spec.initContainers[_]
          satisfied := [good | repo = input.parameters.repos[_] ; good = startswith(container.image, repo)]
          not any(satisfied)
          msg := sprintf("container <%v> has an invalid image repo <%v>, allowed repos are %v", [container.name, container.image, input.parameters.repos])
        }
```

Example of applying the template to the `restricted` policy:

```yaml
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sAllowedRepos
metadata:
  name: prod-repo
spec:
  match:
    kinds:
      - apiGroups: [""]
        kinds: ["Pod"]
    namespaceSelector:
      matchLabels:
        security.deckhouse.io/pod-policy: restricted
  parameters:
    repos:
      - "mycompany.registry.com"
```

In this example, the repository address in the `image` field of all Pods in namespaces
labeled `security.deckhouse.io/pod-policy: restricted` is checked.
If the `image` address of a created Pod does not start with `mycompany.registry.com`, the Pod will not be created.

Helpful resources for creating extended policies:

- [Custom policy examples](#custom-gatekeeper-policy-examples).
- [Gatekeeper documentation](https://open-policy-agent.github.io/gatekeeper/website/docs/howto/) contains information
  on templates and policy language.
- [Gatekeeper library](https://github.com/open-policy-agent/gatekeeper-library/tree/master/src/general) contains examples
  of validation templates.

## Operational policies

DKP provides a mechanism for creating operational policies using the [OperationPolicy](/modules/admission-policy-engine/cr.html#operationpolicy).
Operational policies define requirements for cluster objects such as allowed repositories, required resources, probes, and more.

The DKP development team recommends applying the following minimal operational policy:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: common
spec:
  policies:
    allowedRepos:
      - myrepo.example.com
      - registry.deckhouse.io
    requiredResources:
      limits:
        - memory
      requests:
        - cpu
        - memory
    disallowedImageTags:
      - latest
    requiredProbes:
      - livenessProbe
      - readinessProbe
    maxRevisionHistoryLimit: 3
    imagePullPolicy: Always
    priorityClassNames:
    - production-high
    - production-low
    checkHostNetworkDNSPolicy: true
    checkContainerDuplicates: true
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          operation-policy.deckhouse.io/enabled: "true"
```

This policy defines basic operational requirements for cluster objects,
including allowed container registries, required resources and probes, restrictions on using images with the `latest` tag,
allowed priority classes, and other settings that improve application security and stability.

To assign this operational policy, add the `operation-policy.deckhouse.io/enabled=true` label to the target namespace:

```shell
d8 k label ns my-namespace operation-policy.deckhouse.io/enabled=true
```

## Security policies

Using the [SecurityPolicy](/modules/admission-policy-engine/cr.html#securitypolicy), you can create security policies that define container behavior restrictions in the cluster, such as host network access, privileges, AppArmor usage, and more.

{% alert level="info" %}
To learn what each Pod security setting (for example, `hostNetwork`, `hostPID`, `hostIPC`, `hostPath`, `automountServiceAccountToken`, `capabilities`, `seccompProfile`, etc.) is responsible for, how to choose the correct value, and how it works in practice, see the [Pod security settings](../../../user/security/pod-settings.html) page.
{% endalert %}

Example security policy:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: mypolicy
spec:
  enforcementAction: Deny
  policies:
    allowHostIPC: true
    allowHostNetwork: true
    allowHostPID: false
    allowPrivileged: false
    allowPrivilegeEscalation: false
    allowedFlexVolumes:
    - driver: vmware
    allowedHostPorts:
    - max: 4000
      min: 2000
    allowedProcMount: Unmasked
    allowedAppArmor:
    - unconfined
    allowedUnsafeSysctls:
    - kernel.*
    allowedVolumes:
    - hostPath
    - projected
    fsGroup:
      ranges:
      - max: 200
        min: 100
      rule: MustRunAs
    readOnlyRootFilesystem: true
    requiredDropCapabilities:
    - ALL
    runAsGroup:
      ranges:
      - max: 500
        min: 300
      rule: RunAsAny
    runAsUser:
      ranges:
      - max: 200
        min: 100
      rule: MustRunAs
    seccompProfiles:
      allowedLocalhostFiles:
      - my_profile.json
      allowedProfiles:
      - Localhost
    supplementalGroups:
      ranges:
      - max: 133
        min: 129
      rule: MustRunAs
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          enforce: mypolicy
```

To assign this security policy, add the `enforce: "mypolicy"` label to the target namespace.

### Partial policy enforcement

To enforce specific security policies without disabling the entire predefined set, follow these steps:

1. Add the `security.deckhouse.io/pod-policy: privileged` label to the target namespace to disable the built-in policy set.
1. Create a [SecurityPolicy](/modules/admission-policy-engine/cr.html#securitypolicy) that matches the `baseline` or `restricted` level. In the `policies` section, specify only the security settings you need.
1. Add an extra label to the namespace matching the `namespaceSelector` in the SecurityPolicy.

Example SecurityPolicy configuration for the `baseline` level:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: baseline
spec:
  enforcementAction: Deny
  policies:
    allowHostIPC: false
    allowHostNetwork: false
    allowHostPID: false
    allowPrivilegeEscalation: true
    allowPrivileged: false
    allowedAppArmor:
      - runtime/default
      - localhost/*
    allowedCapabilities:
      - AUDIT_WRITE
      - CHOWN
      - DAC_OVERRIDE
      - FOWNER
      - FSETID
      - KILL
      - MKNOD
      - NET_BIND_SERVICE
      - SETFCAP
      - SETGID
      - SETPCAP
      - SETUID
      - SYS_CHROOT
    allowedHostPaths: []
    allowedHostPorts:
      - max: 0
        min: 0
    allowedProcMount: Default
    allowedUnsafeSysctls:
      - kernel.shm_rmid_forced
      - net.ipv4.ip_local_port_range
      - net.ipv4.ip_unprivileged_port_start
      - net.ipv4.tcp_syncookies
      - net.ipv4.ping_group_range
      - net.ipv4.ip_local_reserved_ports
      - net.ipv4.tcp_keepalive_time
      - net.ipv4.tcp_fin_timeout
      - net.ipv4.tcp_keepalive_intvl
      - net.ipv4.tcp_keepalive_probes
    seLinux:
      - type: ""
      - type: container_t
      - type: container_init_t
      - type: container_kvm_t
      - type: container_engine_t
    seccompProfiles:
      allowedProfiles:
        - RuntimeDefault
        - Localhost
        - undefined
        - ''
      allowedLocalhostFiles:
        - '*'
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          operation-policy.deckhouse.io/baseline-enabled: "true"
```

Example SecurityPolicy configuration for the `restricted` level:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: restricted
spec:
  enforcementAction: Deny
  policies:
    allowHostIPC: false
    allowHostNetwork: false
    allowHostPID: false
    allowPrivilegeEscalation: false
    allowPrivileged: false
    allowedAppArmor:
      - runtime/default
      - localhost/*
    allowedCapabilities:
      - NET_BIND_SERVICE
    allowedHostPaths: []
    allowedHostPorts:
      - max: 0
        min: 0
    allowedProcMount: Default
    allowedUnsafeSysctls:
      - kernel.shm_rmid_forced
      - net.ipv4.ip_local_port_range
      - net.ipv4.ip_unprivileged_port_start
      - net.ipv4.tcp_syncookies
      - net.ipv4.ping_group_range
      - net.ipv4.ip_local_reserved_ports
      - net.ipv4.tcp_keepalive_time
      - net.ipv4.tcp_fin_timeout
      - net.ipv4.tcp_keepalive_intvl
      - net.ipv4.tcp_keepalive_probes
    allowedVolumes:
      - configMap
      - csi
      - downwardAPI
      - emptyDir
      - ephemeral
      - persistentVolumeClaim
      - projected
      - secret
    requiredDropCapabilities:
      - ALL
    runAsUser:
      rule: MustRunAsNonRoot
    seLinux:
      - type: ""
      - type: container_t
      - type: container_init_t
      - type: container_kvm_t
      - type: container_engine_t
    seccompProfiles:
      allowedProfiles:
        - RuntimeDefault
        - Localhost
      allowedLocalhostFiles:
        - '*'
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          operation-policy.deckhouse.io/restricted-enabled: "true"
```

## Gatekeeper custom resources

Gatekeeper offers advanced capabilities for modifying Kubernetes resources using mutation policies.
These policies are defined through the following custom resources:

- [AssignMetadata](/modules/admission-policy-engine/gatekeeper-cr.html#assignmetadata): Modifies the `metadata` section of a resource.
- [Assign](/modules/admission-policy-engine/gatekeeper-cr.html#assign): Modifies fields other than `metadata`.
- [ModifySet](/modules/admission-policy-engine/gatekeeper-cr.html#modifyset): Adds or removes values from a list,
  such as container run arguments.
- [AssignImage](/modules/admission-policy-engine/gatekeeper-cr.html#assignimage): Modifies the `image` field of a resource.

For more on modifying Kubernetes resources using mutation policies, refer to the [Gatekeeper documentation](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/).

## Custom Gatekeeper policy examples

Here you can find examples of Gatekeeper policies that let you extend the standard cluster security mechanism.

### Preventing deletion of a node without a specified label

{% alert level="info" %}
The `DELETE` operations are handled by Gatekeeper by default.
{% endalert %}

You can create a Gatekeeper policy to prevent a node deletion unless it has a special label assigned.

In the following example, the `oldObject` field is used to check labels on the node being deleted:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: d8customnodedeleteguard
spec:
  crd:
    spec:
      names:
        kind: D8CustomNodeDeleteGuard
      validation:
        openAPIV3Schema:
          type: object
          properties:
            requiredLabelKey:
              type: string
            requiredLabelValue:
              type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package d8.custom

        is_delete { input.review.operation == "DELETE" }
        is_node { input.review.kind.kind == "Node" }

        has_required_label {
          key := input.parameters.requiredLabelKey
          val := input.parameters.requiredLabelValue
          obj := input.review.oldObject
          obj.metadata.labels[key] == val
        }

        violation[{"msg": msg}] {
          is_delete
          is_node
          not has_required_label
          msg := sprintf("Node deletion is blocked. Add label %q=%q to proceed.", [input.parameters.requiredLabelKey, input.parameters.requiredLabelValue])
        }
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: D8CustomNodeDeleteGuard
metadata:
  name: require-node-delete-label
spec:
  enforcementAction: warn
  match:
    kinds:
      - apiGroups: [""]
        kinds: ["Node"]
  parameters:
    requiredLabelKey: "admission.deckhouse.io/allow-delete"
    requiredLabelValue: "true"
```

### Preventing exec and attach operations to specific Pods

The `admission-policy-engine` module's webhook routes `CONNECT` requests for `pods/exec` and `pods/attach` through Gatekeeper.
This allows custom policies to allow or deny `kubectl exec` and `kubectl attach` operations.

#### Built-in policy for `heritage: deckhouse` label on Pods

To protect system components managed by Deckhouse,
the `admission-policy-engine` module includes a built-in policy `D8DenyExecHeritage`
that forbids running `kubectl exec` and `kubectl attach` operations to all Pods with the `heritage: deckhouse` label.

This policy doesn't apply to the following users
who are allowed to run `kubectl exec` and `kubectl attach` operations to Pods labeled with `heritage: deckhouse`:

- `system:sudouser`
- service accounts from `d8-*` namespaces (`system:serviceaccount:d8-*`)
- service accounts from `kube-*` namespaces (`system:serviceaccount:kube-*`)

#### Custom policy example

You can create your own Gatekeeper policy to deny `kubectl exec` and `kubectl attach` operations in specific namespaces.
In the following example, `input.review.operation` and `input.review.resource.resource` are used to check for `CONNECT` operations:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: d8customdenyexec
spec:
  crd:
    spec:
      names:
        kind: D8CustomDenyExec
      validation:
        openAPIV3Schema:
          type: object
          properties:
            forbiddenNamespaces:
              type: array
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package d8.custom

        is_connect {
          input.review.operation == "CONNECT"
        }

        # requestSubResource is preferred, but fall back to subResource for older APIs
        subresource_is(sub) {
          sr := object.get(input.review, "requestSubResource", input.review.subResource)
          sr == sub
        }

        is_exec_or_attach {
          input.review.resource.resource == "pods"
          subresource_is("exec")
        }

        is_exec_or_attach {
          input.review.resource.resource == "pods"
          subresource_is("attach")
        }

        is_forbidden_namespace {
          ns := input.review.namespace
          ns == input.parameters.forbiddenNamespaces[_]
        }

        violation[{"msg": msg}] {
          is_connect
          is_exec_or_attach
          is_forbidden_namespace
          msg := sprintf("Exec/attach is forbidden in namespace %q", [input.review.namespace])
        }
---
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: D8CustomDenyExec
metadata:
  name: deny-exec-in-namespaces
spec:
  enforcementAction: deny
  match:
    kinds:
      - apiGroups: ["*"]
        kinds: ["*"]
    scope: Namespaced
  parameters:
    forbiddenNamespaces:
      - production
      - staging
```

Key data and checks for `CONNECT` validation:

- Use `input.review.operation == "CONNECT"` to check for `CONNECT` operations.
- User information is available in `input.review.userInfo.username` and `input.review.userInfo.groups`.
- The namespace is available in `input.review.namespace`.

## Image signature verification

{% alert level="warning" %}
Available in the following DKP editions: SE+, EE.

Cosign versions up to v2 are supported. Versions v3 and above are not supported.
{% endalert %}

DKP supports container image signature verification using [Cosign](https://docs.sigstore.dev/cosign/key_management/signing_with_self-managed_keys/).  
Container image signature verification allows you to ensure their integrity (that the image has not been modified since its creation) and authenticity (that the image was created by a trusted source). You can enable container image signature verification in the cluster using the [policies.verifyImageSignatures](/modules/admission-policy-engine/cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) parameter of the SecurityPolicy.  

Images are signed by creating a special tag in the container registry that contains the image signature.  
The signature is generated for the digest (hash) of your image.  
If your image is `my-repo/app:latest` with the hash `sha256:abc123EXAMPLE`, the tag `my-repo/app:sha256-abc123EXAMPLE.sig` will appear in the image store.

Therefore, the image signing process consists of calculating and publishing an additional tag to the container registry, without modifying the existing image.  
After signing the image, there is no need to push it to the image store again. You only need to log in to the container registry with write access.

To sign an image with Cosign, do the following:

1. Make sure that Cosign version 2 or lower is used:

   Check the version: `cosign version`.

   ```shell
   cosign version
   ```

1. Generate a key pair (public and private):

   ```shell
   cosign generate-key-pair
   ```

1. Sign the image in the container registry using the generated private key:

   ```shell
   cosign sign --key <KEY> <REGISTRY_IMAGE_PATH>
   ```

   Here:
   - `<REGISTRY_IMAGE_PATH>` is the path to the image that needs to be specified at startup, for example: registry.private.com/labs/application/image:latest.

To enable container image signature verification in a DKP cluster:

1. Use the [`policies.verifyImageSignatures`](/modules/admission-policy-engine/cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures)
   parameter in SecurityPolicy and specify the generated public key.

   Example SecurityPolicy configuration for verifying signatures of container images in `registry.private.ru`, located under `/labs/application/`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: SecurityPolicy
   metadata:
     name: verify-image-test
   spec:
     enforcementAction: Deny
     match:
       namespaceSelector:
         labelSelector:
           matchLabels:
             example-security-policy/enabled: true
     policies:
       allowHostIPC: true
       allowHostNetwork: true
       allowHostPID: false
       allowPrivilegeEscalation: true
       allowPrivileged: false
       allowRbacWildcards: true
       verifyImageSignatures:
         - publicKeys:
             - |-
               -----BEGIN PUBLIC KEY-----
               ...
               -----END PUBLIC KEY-----
           reference: registry.private.ru/labs/application/*
   ```

   The label name specified in `match.namespaceSelector.labelSelector.matchLabels` can be any name. It only needs to match between the policy selector and the corresponding namespace.

   More details about selector usage are available in the [selector setup description](/modules/admission-policy-engine/docs/faq.html#how-to-configure-policy-selectors).

1. Create an [OperationPolicy](/modules/admission-policy-engine/cr.html#operationpolicy) that restricts running pods from third-party registries:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: OperationPolicy
   metadata:
     name: test-operation-policy
   spec:
     enforcementAction: Deny
     match:
       namespaceSelector:
         labelSelector:
           matchLabels:
             example-operation-policy/enabled: "true"
     policies:
       allowedRepos:
       - registry.private.ru
   ```

   The label name specified in `match.namespaceSelector.labelSelector.matchLabels` can be any name. It only needs to match between the policy selector and the corresponding namespace.

   More details about selector usage are available in the [selector setup description](/modules/admission-policy-engine/docs/faq.html#how-to-configure-policy-selectors).

1. Add a label to the namespace where signature verification should be enabled (specify your namespace):

   ```shell
   d8 k label ns <NAMESPACE> example-security-policy/enabled=true
   ```

1. Add a label to the namespace where running pods from third-party registries should be restricted (specify your namespace):

   ```shell
   d8 k label ns <NAMESPACE> example-operation-policy/enabled=true
   ```

1. To verify how image signing works, deploy pods in the namespace with signed and unsigned images (specify your namespace):

   ```shell
   d8 k -n <NAMESPACE> run signed-pod --image=<SIGNED_IMAGE>
   d8 k -n <NAMESPACE> run unsigned-pod --image=<UNSIGNED_IMAGE>
   ```

According to this policy, if any container image address matches the `reference` parameter value and the image is unsigned,
or the signature does not match the specified keys, pod creation will be denied.

Example error output when creating a pod with an image that fails signature verification:

```console
[verify-image-signatures] Image signature verification failed: nginx:1.17.2
```

## Using alternative security policy management tools

If you use an alternative solution for security policy management in a DKP cluster
(for example, [Kyverno](https://kyverno.io/docs/introduction/)), configure exceptions for the following namespaces:

- `kube-system`
- all namespaces with the `d8-*` prefix (for example, `d8-system`)

Without these exceptions, policies may block or disrupt system component operations.
