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
- If pods are created via a Deployment and the pods potentially violate a policy, the creation of such Deployment will be rejected.
- If pods are created via a Deployment that already existed before the policy was enabled, the required number of ReplicaSets is created, which in turn attempt to create the pods. In this case, the validation error is not returned in the API response but is displayed in the namespace events or the corresponding ReplicaSet events.

## Controller-level validation

Policies validate pod-creating controllers at `CREATE` and `UPDATE` time. This provides early feedback when a workload is created or updated, before any Pod is launched, and also surfaces creation denials when deploying via CI (CI rarely creates Pods directly, and Pod creation errors from controllers are often not displayed).

{% alert level="warning" %}
By default, a controller that violates a policy with `enforcementAction: Deny` gets a warning instead of a denial.
The Pods the controller creates are still denied.
The module checks controllers in separate constraints named `d8-controllers-<NAME>` with `enforcementAction: warn`.
The `ControllerPolicyViolation` alert reports such controllers, one alert for each policy and namespace.
This is a temporary default for the transition to controller-level validation.
To check controllers with the action of the policy now, set
[`controllerValidationEnforcement`](configuration.html#parameters-podsecuritystandards-controllervalidationenforcement) to `Policy`.
In a future release, `Policy` will become the default.
{% endalert %}

The following objects are validated:

| API group | Resource kind                      | Operations                   |
| --------- | ---------------------------------- |------------------------------|
| (core)    | Pod                                | `CREATE`, `UPDATE`, `DELETE` |
| apps      | Deployment, StatefulSet, DaemonSet | `CREATE`, `UPDATE`, `DELETE` |
| (core)    | ReplicationController              | `CREATE`, `UPDATE`, `DELETE` |
| batch     | Job, CronJob                       | `CREATE`, `UPDATE`, `DELETE` |
| (core)    | pods/exec, pods/attach             | `CONNECT`                    |

`DELETE` is intercepted so that a policy can forbid deleting a resource:
a rule that inspects `input.review.operation` only runs if the request reaches Gatekeeper.

{% alert level="warning" %}
The webhook uses `failurePolicy: Fail`.
While Gatekeeper is unavailable, both creating and deleting the resource kinds listed above is blocked.
To restore operations, bring the `gatekeeper-controller-manager` deployment back up.
{% endalert %}

{% alert level="warning" %}
Controller-level checks run against the **original pod template** (`spec.template`), not against the manifest that results from all mutations.
Pod-level validation, in contrast, happens after all mutating actions — for example, after LimitRange adds default `resources`, or a mutating webhook adds `securityContext`.
Keep this in mind when using security policies.
{% endalert %}

### labelSelector semantics for controllers

When building policies that are applied based on object labels (`match.labelSelector`), keep in mind that selection is performed against the `metadata.labels` of the reviewed object.
For Pods these are the pod's labels.
For controllers (Deployment, StatefulSet, and others) these are the controller's **top-level** `metadata.labels`, not the pod template's `metadata.labels` (`spec.template.metadata.labels`).

As a result, a selector that matches the pods a controller creates does not by itself trigger a controller-level check,
and an exclusion selector (`NotIn`/`DoesNotExist`) that excludes those pods may not take effect at the controller level.
In both cases the controller itself must be matched by the `labelSelector`.

This also applies to the `security.deckhouse.io/skip-pss-check` label, which excludes an object from the Pod Security Standards constraints.
The label is read from the reviewed object's own `metadata.labels`, so a workload that carries it only on the pod template
(`spec.template.metadata.labels`) is exempt at the Pod level but still checked at the controller level.
To exempt such a workload completely, set the label on the controller's top-level `metadata.labels` as well.
Unlike SecurityPolicyException, this cannot be resolved from the pod template: the exclusion is evaluated by Gatekeeper's `match.labelSelector`,
before any policy code runs.

### SecurityPolicyException label resolution for controllers

For controller kinds, SecurityPolicyException (SPE) labels and annotations are taken from the **pod template's metadata** (`spec.template.metadata`), not from the controller's top-level metadata. This ensures SPEs apply to the pods the controller creates, not to the controller object itself.

### Configuring controller-level validation

Controller-level validation is configured by four settings with different scopes:

1. The [`controllerValidation`](configuration.html#parameters-podsecuritystandards-controllervalidation) module parameter
   enables or disables controller-level validation for every constraint that reads a pod spec.
   Despite living in the `podSecurityStandards` section, it applies to Pod Security Standards, OperationPolicy and SecurityPolicy alike.
1. The [`controllerValidationEnforcement`](configuration.html#parameters-podsecuritystandards-controllervalidationenforcement)
   module parameter sets what happens to a controller that violates a denying policy:
   `Warn` (default) accepts it with a warning, `Policy` denies it.
   It applies to the Pod Security Standards and to every OperationPolicy and SecurityPolicy without `spec.controllerValidation.mode`.
1. The [`controllerValidationExcludedPolicies`](configuration.html#parameters-podsecuritystandards-controllervalidationexcludedpolicies)
   module parameter lists the Pod Security Standards rules that are checked on Pods only.
   It applies to the Pod Security Standards only.
1. The `spec.controllerValidation` section of an [OperationPolicy](cr.html#operationpolicy-v1alpha1-spec-controllervalidation)
   or a [SecurityPolicy](cr.html#securitypolicy-v1alpha1-spec-controllervalidation) configures that policy.
   `mode: Enforce` checks controllers with the action of the policy, `mode: Warn` only warns about them,
   and `mode: Disabled` checks only Pods. `Enforce` and `Warn` check controllers even when the module parameter disables it,
   and a policy without `mode` follows the module parameters.
   `excludedPolicies` lists the rules of the policy that are checked on Pods only;
   the module exclusion list does not apply to a policy.

Exclude a rule from controller-level validation when a mutating webhook makes the Pod compliant
while the controller's pod template is not.
For example, the Vertical Pod Autoscaler sets `resources.requests` on the Pod, while the pod template may set `limits` only.
A container that sets no `resources` at all already passes the controller-level check in the
[lenient mode](#lenient-mode-for-fields-that-may-be-filled-in-before-the-pod-is-created),
so the exclusion matters when `resources` are set partially.
The following OperationPolicy checks the image registry on controllers and the resource requests on Pods only:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: requests-set-by-vpa
spec:
  enforcementAction: Deny
  controllerValidation:
    mode: Enforce
    excludedPolicies:
      - requiredResources
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          operation-policy.deckhouse.io/enabled: "true"
  policies:
    allowedRepos:
      - registry.example.com
    requiredResources:
      requests:
        - cpu
        - memory
```

A Deployment whose pod template sets `limits` but no `requests` is accepted, and its Pods are denied unless the Vertical Pod Autoscaler sets the requests.

The rules of the Pod Security Standards are excluded in the module settings.
They are named as the parameters of the SecurityPolicy objects the standards are published with,
see [Rules of the standards](#rules-of-the-standards).
The following example checks `allowPrivilegeEscalation` and `requiredDropCapabilities` on Pods only:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: admission-policy-engine
spec:
  version: 1
  settings:
    podSecurityStandards:
      controllerValidationExcludedPolicies:
        - allowPrivilegeEscalation
        - requiredDropCapabilities
```

Some rules are checked by one constraint and are excluded together: listing any rule of a group checks the whole group on Pods only.
The groups are:

- `allowHostPID` and `allowHostIPC`;
- `allowHostNetwork` and `allowedHostPorts`;
- `fsGroup`, `runAsUser`, `runAsGroup` and `supplementalGroups`;
- `allowedUnsafeSysctls` and `forbiddenSysctls`.

`allowedCapabilities` and `requiredDropCapabilities` are excluded one by one.

When a rule is checked on Pods only:

- controllers (Deployment, StatefulSet, DaemonSet, Job, CronJob, ReplicationController) are not checked against it on creation or update;
- SecurityPolicyException labels on `spec.template.metadata.labels` of controllers are not resolved for it, since it is not applied to controllers;
- Pods are still checked against it at launch time.

### Lenient mode for fields that may be filled in before the pod is created

Some constraints use a **lenient mode** for fields that may be absent from a controller's pod template and still be present on the Pod the controller creates. This prevents denying a workload whose pods would in fact be compliant.

The following constraints skip violations for absent fields when reviewing controllers, while still enforcing them when a field is **explicitly set** to a disallowed value. The decision is made per container: a container that omits the field is treated leniently even if another container in the same pod template sets it.

| Constraint | Skipped fields (when absent on controllers) | What may fill the field in |
|---|---|---|
| `D8RequiredResources` | `container.resources` (when neither `limits` nor `requests` is set) | LimitRange |
| `D8AllowedUsers` | `runAsUser`, `runAsGroup`, `fsGroup`, `supplementalGroups` (with `MustRunAs` / `MustRunAsNonRoot`) | A mutating webhook, including a Gatekeeper `Assign` mutator |

For example, a Deployment without `resources` in its pod template is **not** denied by `D8RequiredResources`, because a LimitRange in the namespace may add default `requests` and `limits`. If `resources` is set partially (for example, only `limits.memory` but not `limits.cpu`), the violation is still enforced.

`D8RequiredResources` relies on an in-tree component. `D8AllowedUsers` does not: in a cluster without a matching mutator it gets no controller-level enforcement of these fields, and the violation surfaces at Pod creation instead. Pod Security Admission is not one of the sources listed above — it only validates and never modifies an object.

`D8AllowedSeccompProfiles` needs no lenient mode: a container that sets no profile in any source is allowed on a Pod as well, so controllers and Pods already behave the same.

### Constraints with limited controller-level coverage

Some checks depend on data that only exists after mutation, so they cannot deny a controller even though they deny its pods.

`D8AutomountServiceAccountToken` detects the token mount either from `spec.automountServiceAccountToken: true` or from a container that mounts `/var/run/secrets/kubernetes.io/serviceaccount`. The ServiceAccount admission plugin adds that mount to the Pod, not to the controller's pod template. A workload that simply omits `automountServiceAccountToken` therefore passes the controller-level check, and the denial appears when its pods are created. To get the denial at the controller level, set `automountServiceAccountToken: false` explicitly in the pod template.

## Pod validation when policies are modified or new ones are added

For all three policy categories (Pod Security Standards, operational, and security policies), there is no provision for automatically recreating existing pods when changing existing policies or adding new ones. Pods that existed prior to changes being made to the policy in use or prior to a new policy being added will continue to run until they are restarted. Upon restart, they will be validated against the new rules.

The `admission-policy-engine` module provides alerts for such cases.
Each alert fires once for each policy and namespace with violating objects:

| Alert | What it reports |
|---|---|
| `PodSecurityStandardsViolation` | Running pods that violate a Pod Security Standard |
| `OperationPolicyViolation` | Existing objects that violate an OperationPolicy |
| `SecurityPolicyViolation` | Existing objects that violate a SecurityPolicy |
| `ControllerPolicyViolation` | Pod-creating controllers that violate a policy with `enforcementAction: Deny` and are only warned about during the transition period, see [Controller-level validation](#controller-level-validation) |

The `policy` label of an alert names the policy:
the name of the OperationPolicy or SecurityPolicy, or `d8-pod-security-baseline` or `d8-pod-security-restricted` for the Pod Security Standards.
The `namespace` label is the namespace of the violating objects.

An alert is an ObservabilityAlert resource in the namespace of the violating objects, so the users of the namespace see it.
An alert for cluster-scoped objects is a ClusterObservabilityAlert resource.

To list the alerts in a namespace, run the following command, where `<NAMESPACE>` is the namespace:

```bash
d8 k -n <NAMESPACE> get observabilityalerts
```

Output example:

<!-- markdownlint-disable MD031 -->
```console
NAME                                                        SEVERITY   STATUS   DURATION   SUMMARY                                                                                     AGE
PodSecurityStandardsViolation-07243a74fddd8234-1792744609   3          Firing   1h         Pods in the `app` namespace violate the `d8-pod-security-baseline` Pod Security Standard.   1h
SecurityPolicyViolation-8f21cd962274a252-1792744609         3          Firing   1h         Objects in the `app` namespace violate the `restricted-app` SecurityPolicy.                 1h
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

To list the alerts in every namespace and the alerts for cluster-scoped objects, run the following commands:

```bash
d8 k get observabilityalerts -A
d8 k get clusterobservabilityalerts
```

To view an alert, run the following command:

```bash
d8 k -n <NAMESPACE> get observabilityalert <ALERT_NAME> -o yaml
```

The description of the alert contains a Prometheus query that lists the violating objects and the rules they break.

## Pod Security Standards

[Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) (`PSS`) is an official Kubernetes standard that defines three security levels for pods, limiting their privileges. Restrictions are enforced by prohibiting the setting of certain parameters in the pod manifest.

A layered structure is used — each higher level of protection uses all the rules of the previous level and adds its own.

The following protection levels are regulated:

- `Privileged`: An unrestricted policy with the widest possible level of permissions (no restrictions).
- `Baseline`: A minimally restrictive policy that prevents the most known and popular ways of privilege escalation. Allows using the standard (minimally specified) pod configuration.
- `Restricted`: A policy with significant restrictions. Imposes the strictest requirements on pods.

{% alert level="info" %}
In the Deckhouse Platform, these policies are implemented using Gatekeeper and enforced by the admission controllers of the `admission-policy-engine` module, rather than the Kubernetes [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/) controller. Only the policy descriptions are taken from Kubernetes.
{% endalert %}

You can read more about each set of policies and their restrictions in the [Kubernetes documentation](https://kubernetes.io/docs/concepts/security/pod-security-standards/#profile-details).

Configuring PSS policies for namespaces is done by setting a special label `security.deckhouse.io/pod-policy=<POLICY_NAME>` on the corresponding namespace.
The default policy can be overridden globally ([in the module settings](configuration.html#parameters-podsecuritystandards-defaultpolicy)).

{% alert level="info" %}
In namespaces named `d8-*` and `kube-*`, the `restricted` standard applies in the `warn` mode,
whatever the default policy and the namespace labels.
A violation is recorded in the audit and shown in the web interface Deckhouse Platform, and the workload still starts.
If the module that owns the namespace has enabled policy checks for it, the standard is also applied there
with the enforcement action configured for that namespace.
The `security.deckhouse.io/pod-policy-action` label cannot lower the action below `warn` in these namespaces:
with `dryrun`, the user still receives warnings.
In such a namespace each violation is reported by both checks, so the API server response and the audit contain it twice.
The module dashboard counts such a violation once.
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

Example of setting the `warn` mode for PSS policies for all pods in the `my-namespace` namespace:

```bash
d8 k label ns my-namespace security.deckhouse.io/pod-policy-action=warn
```

### Rules of the standards

The module publishes the rules of the standards as two SecurityPolicy objects,
`d8-pod-security-baseline` and `d8-pod-security-restricted`.
The rules are written in the format of the [SecurityPolicy](cr.html#securitypolicy) parameters,
the same ones you use in your own security policies.
`d8-pod-security-restricted` lists every rule a `Restricted` namespace is checked against, the `Baseline` rules included.
Two values in the rules have no counterpart in the SecurityPolicy description:
`""` and `undefined` in `seccompProfiles.allowedProfiles` allow a container that sets no seccomp profile,
and `type: ""` in `seLinux` allows a container that sets no SELinux type.

To view the rules of the `Restricted` standard, run the following command:

```bash
d8 k get securitypolicy d8-pod-security-restricted -o yaml
```

The `spec.policies` section lists the rules, and the `spec.controllerValidation` section shows the controller-level validation settings
taken from the module configuration.

The objects describe the standards and enforce nothing by themselves.
The module applies the standards to namespaces by the `security.deckhouse.io/pod-policy` label and the module settings,
and `spec.match` of the objects shows the application namespaces the standard applies to with the default enforcement mode.
The objects are managed by the module and cannot be modified.
Configure the standards in the [module settings](configuration.html#parameters-podsecuritystandards).
If a SecurityPolicy of your own is already named `d8-pod-security-baseline` or `d8-pod-security-restricted`,
the module keeps your policy and does not publish that standard. The standard is still enforced.
To see its rules, rename your policy.

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

## Availability of the module components

The module is on the critical path of the cluster: while its validating webhook is unavailable, the API server rejects the requests the webhook intercepts.

### Why the validating webhook is a critical component

The `gatekeeper-controller-manager` deployment serves the ValidatingWebhookConfiguration named `d8-admission-policy-engine-config`. Its main webhook is configured with `failurePolicy: Fail`, which means that a request the API server cannot deliver to the webhook is rejected rather than admitted.

While the webhook is unavailable, no object bypasses the policies, but the objects the webhook intercepts cannot be created or changed during that time either.

While no replica of `gatekeeper-controller-manager` is available, the following stops working in the namespaces the webhook covers:

- Operations on the objects listed in [Controller-level validation](#controller-level-validation), including `d8 k exec` and `d8 k attach`, as well as `d8 k debug`
- Creation, modification and deletion of Role, RoleBinding and Gatekeeper constraints
- `d8 k exec` and `d8 k attach` in namespaces whose names start with `d8-` and `kube-`, which a separate webhook with `failurePolicy: Fail` intercepts

The mutating webhook is configured differently: its `failurePolicy` is `Ignore`, and an unavailable deployment only means that mutations are not applied.

Disabling the module unblocks the cluster, but the control goes with it: no policy is enforced any more, and an object a policy used to forbid is created without hindrance. The FAQ describes [what to do](faq.html#what-to-do-if-the-validating-webhook-is-unavailable) while the webhook is unavailable.

### Which objects are excluded from validation

The exclusions define which objects can be created and changed while the webhook is unavailable.

The webhooks of the module do not validate the following:

- Objects in namespaces with the `heritage: deckhouse` label, which the main webhook excludes through `namespaceSelector`
- Objects with the `gatekeeper.sh/operation: webhook` label that Gatekeeper sets on its own pods, which the webhooks exclude through `objectSelector`
- Requests from service accounts of the `d8-virtualization` namespace

Namespaces that also carry the `security.deckhouse.io/enable-security-policy-check` label, including `d8-admission-policy-engine`, are validated by a separate webhook.

The exclusion by the `gatekeeper.sh/operation: webhook` label is required in the namespace of the module. Without this exclusion, the webhook could not recover from its own outage. Once the last replica is gone, the API server would reject the creation of a replacement pod, since no replica is available to validate it.

`d8 k exec` and `d8 k attach` are not covered by the exclusion by label. The object of such a request cannot have labels, and the API server never calls a webhook with `objectSelector` for it. In the other namespaces these requests are intercepted by a separate webhook without `objectSelector`, which excludes the same namespaces as the main webhook.

The exclusions do not apply to the webhook that intercepts `d8 k exec` and `d8 k attach` in the `d8-*` and `kube-*` namespaces: it has neither `namespaceSelector` nor `objectSelector`, so `d8 k exec` and `d8 k attach` into the pods of the module are blocked during an outage as well.
