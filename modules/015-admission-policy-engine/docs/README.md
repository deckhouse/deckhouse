---
title: "The admission-policy-engine module"
description: Deckhouse admission-policy-engine module enforces the security policies in a Kubernetes cluster according to the Kubernetes Pod Security Standards.
---

The `admission-policy-engine` module implements support for admission security policies in a Kubernetes cluster.

Admission policies are rules applied to objects (e.g., `Pod` and `Service`) at the time of their creation or modification in the cluster (but not during their operation), based on the information provided in their manifest. These policies are aimed at formalizing parameters that are allowed or prohibited in object manifests.

Policies are divided into three categories:
- `Pod Security Standards`;
- Security policies;
- Operational policies.


## Pod Security Standards

[Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) (`PSS`) is an official Kubernetes standard that defines three security levels for pods, limiting their privileges. Restrictions are enforced by prohibiting the setting of certain parameters in the pod manifest.

A layered structure is used — each higher level of protection uses all the rules of the previous level and adds its own.

The following protection levels are regulated:
- `Privileged` — an unrestricted policy with the widest possible level of permissions (no restrictions);
- `Baseline` — a minimally restrictive policy that prevents the most known and popular ways of privilege escalation. Allows using the standard (minimally specified) pod configuration;
- `Restricted` — a policy with significant restrictions. Imposes the strictest requirements on pods.

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
  - `deny` - prohibit starting pods that do not satisfy the policy;
  - `warn` - start pods that do not satisfy the policy, but issue a warning;
  - `dryrun` - start pods that do not satisfy the policy, do not issue a warning to the user, but record violations in security reports.

Configuring the policy enforcement mode is done by setting the label `security.deckhouse.io/pod-policy-action=<POLICY_ACTION>` on the corresponding namespace.
To set the policy enforcement mode globally, use the [`enforcementaction`](configuration.html#parameters-podsecuritystandards-enforcementaction) parameter.

Example of setting the "warn" mode for PSS policies for all pods in the `my-namespace` namespace:

```bash
d8 k label ns my-namespace security.deckhouse.io/pod-policy-action=warn
```

## Operational policies

Operational policies are rules aimed at achieving application security best practices, but not directly related to the validation of classic security-related parameters.

Operational policies are described using the [`OperationPolicy`](/modules/admission-policy-engine/cr.html#operationpolicy) custom resource.
In this resource, each parameter is responsible for a separate check applied to resources.

We recommend setting the following minimum set of operational policies:

```yaml
---
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
  - `Deny` - prohibit starting pods that do not satisfy the policy;
  - `Warn` - start pods that do not satisfy the policy, but issue a warning;
  - `Dryrun` - start pods that do not satisfy the policy, do not issue a warning to the user, but record violations in security reports.

Based on this example, you can create your own policy with the necessary settings.

## Security policies

Security policies are rules aimed at achieving application security best practices by validating the values of security-related parameters.

Security policies are described using the [`SecurityPolicy`](/modules/admission-policy-engine/cr.html#securitypolicy) custom resource.
In this resource, each parameter is responsible for a separate check applied to resources.
Using this resource, it is possible to construct a security policy similar to a PSS policy of any level.

Example of a security policy:

```yaml
---
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
  - `Deny` - prohibit starting pods that do not satisfy the policy;
  - `Warn` - start pods that do not satisfy the policy, but issue a warning;
  - `Dryrun` - start pods that do not satisfy the policy, do not issue a warning to the user, but record violations in security reports.


## Modifying Kubernetes resources

The module allows you to use the [Gatekeeper Custom Resources](gatekeeper-cr.html) to modify objects in the cluster, such as
- [AssignMetadata](gatekeeper-cr.html#assignmetadata) — defines changes to the `metadata` section of a resource.
- [Assign](gatekeeper-cr.html#assign) — any change outside the `metadata` section.
- [ModifySet](gatekeeper-cr.html#modifyset) — adds or removes entries from a list, such as the arguments to a container.
- [AssignImage](gatekeeper-cr.html#assignimage) — to change the `image` parameter of the resource.

You can read more about the available options in the [gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/) documentation.
