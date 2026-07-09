---
title: "The admission-policy-engine module"
description: Deckhouse admission-policy-engine module enforces the security policies in a Kubernetes cluster according to the Kubernetes Pod Security Standards.
---

The `admission-policy-engine` module implements support for admission security policies in a Kubernetes cluster.

Admission policies are rules applied to objects (e.g., Pod and Service) at the time of their creation or modification in the cluster (but not during their operation), based on the information provided in their manifest. These policies are aimed at formalizing parameters that are allowed or prohibited in object manifests.

Policies are divided into three categories:

- [Pod Security Standards](#pod-security-standards): Policies that comply with the relevant [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/).
- [Operational policies](#operational-policies): Policies for creating additional requirements for objects by validating the values of parameters that are **not directly related** to security (for example, a list of allowed prefixes for container images, an image download policy, a list of required container images, etc.).
- [Security policies](#security-policies): Policies for creating additional requirements on objects by validating the values of security-related parameters (e.g., container access to the host’s IPC or PID namespaces, privilege lists for containers, etc.).

{% alert level="info" %}
These policies complement each other. If multiple policies are applied to a single namespace, objects are validated against each of them. If even one policy is violated, the object will not be created.
{% endalert %}

In addition to policies that prohibit using parameters different from the set requirements, the module supports the [SecurityPolicyException](#security-policy-exceptions) resource, which allows creating fine-grained exceptions from security policy checks. With this resource, you can allow using specific parameters for individual pods or containers without changing security policies applied to the entire namespace.

## How validation failure messages are displayed

Depending on how pods are created, there are differences in how the API generates messages regarding validation failures (violations of established policies):

- If a pod is created directly, the validation error is returned in the API response indicating a validation failure (policy violation).
- If pods are created via Deployment, the required number of ReplicaSets is created, which in turn attempt to create the pods. In this case, the validation error is not returned in the API response but is displayed in the namespace events or the corresponding ReplicaSet events.

## Pod validation when policies are modified or new ones are added

For all three policy categories (Pod Security Standards, operational, and security policies), there is no provision for automatically recreating existing pods when changing existing policies or adding new ones. Pods that existed prior to changes being made to the policy in use or prior to a new policy being added will continue to run until they are restarted. Upon restart, they will be validated against the new rules.

The `admission-policy-engine` module provides alerts (`kind: ClusterObservabilityAlert`) for such cases, notifying you of pods in the namespace that violate policies after an existing policy is modified or a new one is added.

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

## Pod Security Standards

[Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) (`PSS`) is an official Kubernetes standard that defines three security levels for pods, limiting their privileges. Restrictions are enforced by prohibiting the setting of certain parameters in the pod manifest.

A layered structure is used — each higher level of protection uses all the rules of the previous level and adds its own.

The following protection levels are regulated:

- `Privileged`: An unrestricted policy with the widest possible level of permissions (no restrictions).
- `Baseline`: A minimally restrictive policy that prevents the most known and popular ways of privilege escalation. Allows using the standard (minimally specified) pod configuration.
- `Restricted`: A policy with significant restrictions. Imposes the strictest requirements on pods.

{% alert level="info" %}
In the Deckhouse Kubernetes Platform, these policies are implemented using Gatekeeper and enforced by the admission controllers of the `admission-policy-engine` module, rather than the Kubernetes [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/) controller. Only the policy descriptions are taken from Kubernetes.
{% endalert %}

You can read more about each set of policies and their restrictions in the [Kubernetes documentation](https://kubernetes.io/docs/concepts/security/pod-security-standards/#profile-details).

Configuring PSS policies for namespaces is done by setting a special label `security.deckhouse.io/pod-policy=<POLICY_NAME>` on the corresponding namespace.
The default policy can be overridden globally ([in the module settings](configuration.html#parameters-podsecuritystandards-defaultpolicy)).

{% alert level="info" %}
The module does not apply policies to system namespaces.
{% endalert %}

{% alert level="info" %}
When the [`multitenancy-manager` module](/modules/multitenancy-manager/) is enabled, it creates its own OperationPolicy objects (for example, in the `default` namespace). These are not affected by the [`podSecurityStandards`](configuration.html#parameters-podsecuritystandards) settings.
{% endalert %}

Example of setting the `Restricted` policy for all pods in the `my-namespace` namespace:

```bash
d8 k label ns my-namespace security.deckhouse.io/pod-policy=restricted
```

Additionally, it is possible to configure the policy enforcement mode.
The following modes are supported:

- `deny`: Prohibit starting pods that do not satisfy the policy.
- `warn`: Start pods that do not satisfy the policy, but issue a warning.
- `dryrun`: Start pods that do not satisfy the policy, do not issue a warning to the user, but record violations in security reports.

Configuring the policy enforcement mode is done by setting the label `security.deckhouse.io/pod-policy-action=<POLICY_ACTION>` on the corresponding namespace.
To set the policy enforcement mode globally, use the [`enforcementaction`](configuration.html#parameters-podsecuritystandards-enforcementaction) parameter.

Example of setting the "warn" mode for PSS policies for all pods in the `my-namespace` namespace:

```bash
d8 k label ns my-namespace security.deckhouse.io/pod-policy-action=warn
```

## Operational policies

Operational policies are rules aimed at achieving application security best practices, but not directly related to the validation of classic security-related parameters (for example, a list of allowed prefixes for container images, an image download policy, a list of required container images, etc.).

Operational policies are described using the [`OperationPolicy`](/modules/admission-policy-engine/cr.html#operationpolicy) custom resource.
In this resource, each parameter is responsible for a separate check applied to resources.
Using the OperationPolicy custom resource allows you to define additional requirements for the resources being created (high-level declarative operational policies) without explicitly interacting with Gatekeeper.

We recommend setting the following minimum set of operational policies:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: common
spec:
  enforcementAction: Deny
  policies:
    allowedRepos:
      - myrepo.example.com
      - registry.deckhouse.ru
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
          custom-operation-policy/enabled: "true"
```

Policy application is implemented through settings located in the `spec.match` parameter.

When specifying:

```yaml
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          custom-operation-policy/enabled: "true"
```

To apply the above policy, it is sufficient to add the label `custom-operation-policy/enabled: "true"` to the desired namespace.
Unlike `PSS`, the label name can be anything. Only a match between the label in the policy selector and the corresponding namespace is required.

You can read more detailed information about using selectors in the [selector setup description](/modules/admission-policy-engine/faq.html#how-to-configure-policy-selectors).

It is also possible to specify the action to be applied for the policy.
The `spec.enforcementAction` parameter is used for this.
The following modes are supported:

- `Deny`: Prohibit starting pods that do not satisfy the policy.
- `Warn`: Start pods that do not satisfy the policy, but issue a warning.
- `Dryrun`: Start pods that do not satisfy the policy, do not issue a warning to the user, but record violations in security reports.

Based on this example, you can create your own policy with the necessary settings.

## Security policies

Security policies are rules aimed at achieving application security best practices by validating the values of security-related parameters.

Security policies are described using the [`SecurityPolicy`](/modules/admission-policy-engine/cr.html#securitypolicy) custom resource.
In this resource, each parameter is responsible for a separate check applied to resources.
Using this resource, it is possible to construct a security policy similar to a PSS policy of any level.
Using the custom SecurityPolicy resource allows you to define additional requirements for the resources being created (high-level declarative security policies) without explicitly interacting with Gatekeeper.

Example of a security policy:

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
          security-policy: mypolicy
```

{% alert level="warning" %}
The `allowPrivilegeEscalation` and `allowPrivileged` parameters default to `false` — even if not explicitly specified. This means that containers will not be able to run in privileged mode or escalate privileges. To allow such behavior, set the parameter to `true`.
{% endalert %}

Policy application is implemented through settings located in the `spec.match` parameter.

When specifying:

```yaml
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          security-policy: mypolicy
```

To apply the above policy, it is sufficient to add the label `security-policy: mypolicy` to the desired namespace.
Unlike `PSS`, the label name can be anything. Only a match between the label in the policy selector and the corresponding namespace is required.

You can read more detailed information about using selectors in the [selector setup description](/modules/admission-policy-engine/faq.html#how-to-configure-policy-selectors).

It is also possible to specify the action to be applied for the policy.
The `spec.enforcementAction` parameter is used for this.
The following modes are supported:

- `Deny`: Prohibit starting pods that do not satisfy the policy.
- `Warn`: Start pods that do not satisfy the policy, but issue a warning.
- `Dryrun`: Start pods that do not satisfy the policy, do not issue a warning to the user, but record violations in security reports.

## Security policy exceptions

[SecurityPolicyException](cr.html#securitypolicyexception) is a resource that lets you create fine-grained exceptions from security policy checks for individual pods and containers. It allows you to avoid excluding an entire namespace from checks and instead define only the necessary exceptions from a specific rule for a pod or container.

### Adding exceptions

To add exceptions for a pod or container, do the following:

1. Create a [SecurityPolicyException](cr.html#securitypolicyexception) object describing the required exceptions.

   It is recommended that you describe the reason for each exception in the rule's `metadata` field (for example, `metadata.description`). This makes auditing and maintenance easier.

2. In the pod template (usually via `spec.template.metadata.labels` in a Deployment, StatefulSet, or DaemonSet resource), add one of the following labels referencing the exception:
   - `security.deckhouse.io/security-policy-exception: <exception-name>`: Exception for the entire pod.
   - `security.deckhouse.io/security-policy-exception.container.<container-name>: <exception-name>`: Exception for a specific container.

Priority when selecting an exception for a container:

1. The label `security.deckhouse.io/security-policy-exception.container.<container-name>` is checked first.
1. If the container-specific label is absent, the exception from `security.deckhouse.io/security-policy-exception` is used.

{% alert level="warning" %}
If a container-specific label is set for a container but it points to an invalid or non-existent SecurityPolicyException object, it still has priority over the global label and may lead to pod placement denial.
{% endalert %}

### Configuration example

For this example, consider a pod that requires:

- Permission to use the [`hostNetwork`](/products/kubernetes-platform/documentation/v1/user/security/pod-settings.html#hostnetwork) parameter for the entire pod.
- Permission to use the [`privileged`](/products/kubernetes-platform/documentation/v1/user/security/pod-settings.html#privileged) parameter only for the `sample-init` container.

Without the SecurityPolicyException resource, allowing these parameters would require implementing a custom security policy where these settings could be allowed for any pod in the cluster.

With SecurityPolicyException, it is enough to create only the following resources:

- Exception to allow the `hostNetwork` parameter:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: SecurityPolicyException
  metadata:
    name: allow-hostnetwork-pod
  spec:
    network:
      hostNetwork:
        allowedValue: true
        metadata:
          description: >-
            Pod requires host network mode for node-level network diagnostics.
  ```

- Exception to allow the `privileged` parameter in the `sample-init` container:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: SecurityPolicyException
  metadata:
    name: allow-privileged-init-container
  spec:
    securityContext:
      privileged:
        allowedValue: true
        metadata:
          description: >-
            Container init requires privileged mode to access host-level networking features.
  ```

After that, the corresponding labels need to be added to the pod template:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example
spec:
  template:
    metadata:
      labels:
        # General exception applicable to the entire pod.
        security.deckhouse.io/security-policy-exception: allow-hostnetwork-pod
        # Exception applicable to the sample-init container.
        security.deckhouse.io/security-policy-exception.container.sample-init: allow-privileged-init-container
    spec:
      hostNetwork: true
    ...
    containers:
      - name: sample-init
        securityContext:
          privileged: true
```

## Modifying Kubernetes resources

The module allows you to use the [Gatekeeper Custom Resources](gatekeeper-cr.html) to modify objects in the cluster, such as:

- [AssignMetadata](gatekeeper-cr.html#assignmetadata) — defines changes to the `metadata` section of a resource.
- [Assign](gatekeeper-cr.html#assign) — any change outside the `metadata` section.
- [ModifySet](gatekeeper-cr.html#modifyset) — adds or removes entries from a list, such as the arguments to a container.
- [AssignImage](gatekeeper-cr.html#assignimage) — to change the `image` parameter of the resource.

You can read more about the available options in the [gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/) documentation.
