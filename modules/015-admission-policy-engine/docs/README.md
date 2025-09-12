---
title: "The admission-policy-engine module"
description: Deckhouse admission-policy-engine module enforces the security policies in a Kubernetes cluster according to the Kubernetes Pod Security Standards.
---

This module enforces the security policies in the cluster according to the Kubernetes [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) using the [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/) solution.

The Pod Security Standards define three different policies to broadly cover the security spectrum. These policies are cumulative and range from highly-permissive to highly-restrictive.

{% alert level="info" %}
The module does not apply policies to system namespaces.
{% endalert %}

List of policies available for use:
- `Privileged` — Unrestricted policy. Provides the widest possible permission level;
- `Baseline` — Minimally restrictive policy which prevents known privilege escalations. Allows for the default (minimally specified) Pod configuration;
- `Restricted` — Heavily restricted policy. Follows the most current Pod hardening best practices.

You can read more about each policy variety in the [Kubernetes documentation](https://kubernetes.io/docs/concepts/security/pod-security-standards/#profile-details).

The type of cluster policy to use by default is determined based on the following criteria:
- If a Deckhouse version **lower than v1.55** is being installed, the `Privileged` default policy is applied to all non-system namespaces;
- If a Deckhouse version starting with **v1.55** is being installed, the `Baseline` default policy is applied to all non-system namespaces;

**Note** that upgrading Deckhouse in a cluster to v1.55 does not automatically result in a default policy change.

Default policy can be overridden either globally ([in the module settings](configuration.html#parameters-podsecuritystandards-defaultpolicy)) or on a per-namespace basis (using the `security.deckhouse.io/pod-policy=<POLICY_NAME>` label for the corresponding namespace).

Example of the command to set the `Restricted` policy for all Pods in the `my-namespace` Namespace.

```bash
d8 k label ns my-namespace security.deckhouse.io/pod-policy=restricted
```

By default, Pod Security Standards policies have their enforcement actions set to "Deny" which means any workload pods not compliant to the selected policy won't be able to run. This behavior can be adjusted either for the whole cluster or per namespace. For setting PSS enforcement action cluster-wide check [configuration](configuration.html#parameters-podsecuritystandards-enforcementaction). In case you want to override default enforcement action for a namespace, set label `security.deckhouse.io/pod-policy-action =<POLICY_ACTION>` to the corresponding namespace. The list of possible enforcement actions consists of the following values: "dryrun", "warn", "deny".

Below is an example of setting the "warn" PSS policy mode for all pods in the `my-namespace` namespace:

```bash
d8 k label ns my-namespace security.deckhouse.io/pod-policy-action=warn
```

The policies define by the module can be expanded. Examples of policy extensions can be found in the [FAQ](faq.html).

### Operation policies

The module provides a set of operating policies and best practices for the secure operation of your applications.
We recommend you deploy the following minimum set of operating policies:

```yaml
---
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

To apply the policy, it will be sufficient to set the label `operation-policy.deckhouse.io/enabled: "true"` on the desired namespace.
The above policy is generic and recommended by Deckhouse team. Similarly, you can configure your own policy with the necessary settings.

> **Warning**. The `allowPrivilegeEscalation` and `allowPrivileged` parameters default to `false` — even if not explicitly set. This means containers cannot run in privileged mode or escalate privileges by default. To allow this behavior, set the corresponding parameter to `true`.

- ### OperationPolicy pods knobs 

- `spec.policies.pods` — object. Pod-level operation controls.
  - `denyTolerations` — object.
    - `enabled` (boolean, default: false): enable the check.
    - `enforcementAction` (string, default: "Warn"): action on violation. Allowed: `Warn`, `Deny`, `Dryrun`.
    - `forbiddenKeys` (string[], default: `["node-role.kubernetes.io/master", "node-role.kubernetes.io/control-plane"]`): taint keys that Pods are not allowed to tolerate (`spec.tolerations[*].key`).
    - `exemptNamespaces` (string[], default: `["kube-system", "d8-system", "d8-admission-policy-engine", "gatekeeper-system"]`): namespaces exempt from this check.

Notes:
- `denyTolerations` validates only toleration keys (not operator/value/effect).
- Enforcement for these knobs is local to the knob and does not depend on the top-level `spec.enforcementAction`.

### Notes

- DELETE operations are now handled by Gatekeeper by default.

### Custom example: Block Node deletion without a label

You can create your own Gatekeeper policy to block Node deletion unless a special label is present. Example below uses oldObject to check labels on the Node being deleted:

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

### Security policies

The module allows defining security policies for making sure the workload running in the cluster meets certain security requirements.

An example of a security policy:

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
          enforce: mypolicy
```

To apply the policy, it will be sufficient to set the label `enforce: "mypolicy"` on the desired namespace.

### Modifying Kubernetes resources

The module allows you to use the [Gatekeeper Custom Resources](gatekeeper-cr.html) to modify objects in the cluster, such as
- [AssignMetadata](gatekeeper-cr.html#assignmetadata) — defines changes to the `metadata` section of a resource.
- [Assign](gatekeeper-cr.html#assign) — any change outside the `metadata` section.
- [ModifySet](gatekeeper-cr.html#modifyset) — adds or removes entries from a list, such as the arguments to a container.
- [AssignImage](gatekeeper-cr.html#assignimage) — to change the `image` parameter of the resource.

You can read more about the available options in the [gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/) documentation.
