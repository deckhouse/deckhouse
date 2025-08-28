---
title: "The admission-policy-engine module: Custom Resources (by Gatekeeper)"
---

## Mutation Custom Resources

{% alert level="info" %}
The `reinvocationPolicy: IfNeeded` is used in MutatingWebhookConfiguration. More details [in the Kubernetes documentation.](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#reinvocation-policy)
{% endalert %}  

[Reference](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/#mutation-crds)

Provide a configurable set of policies for modifying Kubernetes resources at the time they are deployed.

### AssignMetadata

[Reference](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/#assignmetadata)

Allows you to modify the `Metadata` section of a resource.
At the moment, Gatekeeper only allows **adding** `labels` and `annotations` objects. Modification of existing objects is not provided.

{% alert level="info" %}
Using `*` in `spec.match.kinds` is not allowed. If `*` is specified, the mutation will not be applied. Instead, you must explicitly list the target resources (`kinds`) along with their corresponding `apiGroups`.
{% endalert %}

Example 1. Adding the `owner` label with the value `admin` to all namespaces:

```yaml
apiVersion: mutations.gatekeeper.sh/v1
kind: AssignMetadata
metadata:
  name: demo-annotation-owner
spec:
  match:
    scope: Namespaced
  location: "metadata.labels.owner"
  parameters:
    assign:
      value: "admin"
```

Example 2. Adding a label in a specific namespace and only to selected resources:

```yaml
apiVersion: mutations.gatekeeper.sh/v1
kind: AssignMetadata
metadata:
  name: set-labels-<your_namespace>
spec:
  match:
    scope: Namespaced
    namespaces: ["<your_namespace>"]
    kinds:
    - apiGroups: [""]
      kinds: ["Pod"] # The use of "*" is not allowed.
  location: "metadata.labels.<your_label_name>"
  parameters:
    assign:
      value: <your_label_value>
```

### Assign

<!-- 
[Reference](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/#assignmetadata) 
There is no link in the Gatekeeper documentation for this CR
-->

Allows you to modify fields outside the `Metadata` section.

An example of setting `imagePullPolicy` for all containers to `Always` in all namespaces except the `system` namespace:

```yaml
apiVersion: mutations.gatekeeper.sh/v1
kind: Assign
metadata:
  name: demo-image-pull-policy
spec:
  applyTo:
  - groups: [""]
    kinds: ["Pod"]
    versions: ["v1"]
  match:
    scope: Namespaced
    kinds:
    - apiGroups: ["*"]
      kinds: ["Pod"]
    excludedNamespaces: ["system"]
  location: "spec.containers[name:*].imagePullPolicy"
  parameters:
    assign:
      value: Always
```

### ModifySet

[Reference](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/#modifyset)

Allows you to add and remove items from a list, such as arguments for running a container.
New values are added to the end of the list.

An example of removing the `--alsologtostderr` argument from all containers in a pod:

```yaml
apiVersion: mutations.gatekeeper.sh/v1
kind: ModifySet
metadata:
  name: remove-err-logging
spec:
  applyTo:
  - groups: [""]
    kinds: ["Pod"]
    versions: ["v1"]
  location: "spec.containers[name: *].args"
  parameters:
    operation: prune
    values:
      fromList:
        - --alsologtostderr
```

### AssignImage

[Reference](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/#assignimage)

Allows you to make changes to the `image` parameter of a resource.

An example of changing the `image` parameter to the value `my.registry.io/repo/app@sha256:abcde67890123456789abc345678901a`:

```yaml
apiVersion: mutations.gatekeeper.sh/v1alpha1
kind: AssignImage
metadata:
  name: assign-container-image
spec:
  applyTo:
  - groups: [ "" ]
    kinds: [ "Pod" ]
    versions: [ "v1" ]
  location: "spec.containers[name:*].image"
  parameters:
    assignDomain: "my.registry.io"
    assignPath: "repo/app"
    assignTag: "@sha256:abcde67890123456789abc345678901a"
  match:
    source: "All"
    scope: Namespaced
    kinds:
    - apiGroups: [ "*" ]
      kinds: [ "Pod" ]
```
