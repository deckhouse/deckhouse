---
title: "The admission-policy-engine module: FAQ"
---

## How to extend Pod Security Standards policies?

> Pod Security Standards respond to the `security.deckhouse.io/pod-policy: restricted` or `security.deckhouse.io/pod-policy: baseline` label.

To extend the Pod Security Standards policy by adding your checks to existing checks, you need to:
- Create a constraint template for the check (a `ConstraintTemplate` resource).
- Bind it to the `restricted` or `baseline` policy.

Example of the `ConstraintTemplate` for checking a  repository URL of a container image:

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

Example of binding a check to the `restricted` policy:

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

The example demonstrates the configuration of checking the repository address in the `image` field for all Pods created in the namespace having the `security.deckhouse.io/pod-policy : restricted`  label. A Pod will not be created if the address in the `image` field of the Pod does not start with `mycompany.registry.com`.

The [Gatekeeper documentation](https://open-policy-agent.github.io/gatekeeper/website/docs/howto) may find more info about templates and policy language.

Find more examples of checks for policy extension in the [Gatekeeper Library](https://github.com/open-policy-agent/gatekeeper-library/tree/master/src/general).

## What if there are multiple policies (operational or security) that are applied to the same object?

In that case the object's specification have to fulfil all the requirements imposed by the policies.

For example, consider the following two security policies:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: foo
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          name: test
  policies:
    readOnlyRootFilesystem: true
    requiredDropCapabilities:
    - MKNOD
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: bar
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          name: test
  policies:
    requiredDropCapabilities:
    - NET_BIND_SERVICE
```

Then, in order to fulfill the requirements of the above security policies, the following settings must be set in a container specification:

```yaml
    securityContext:
      capabilities:
        drop:
          - MKNOD
          - NET_BIND_SERVICE
      readOnlyRootFilesystem: true
```

## Verification of image signatures

{% alert level="warning" %}This feature is available in Enterprise Edition only.{% endalert %}

The module implements a function for checking the signatures of container images signed using [Cosign](https://docs.sigstore.dev/cosign/key_management/signing_with_self-managed_keys/#:~:text=To%20generate%20a%20key%20pair,prompted%20to%20provide%20a%20password.&text=Alternatively%2C%20you%20can%20use%20the,%2C%20ECDSA%2C%20and%20ED25519%20keys). Checking the signatures of container images allows you to ensure their integrity (that the image has not been modified since its creation) and authenticity (that the image was created by a trusted source). You can enable container image signature verification in the cluster using the [policies.verifyImageSignatures](cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) parameter of the SecurityPolicy resource.

{% offtopic title="How to sign an image..." %}
Steps to sign an image:
- Generate keys: `cosign generate-key-pair`
- Sign the image: `cosign sign --key <key> <image>`

For more information on working with Cosign, you can check the [documentation](https://docs.sigstore.dev/cosign/key_management).
{% endofftopic %}

Example of SecurityPolicy for configuring the signature verification of container images:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: verify-image-signatures
spec:
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          kubernetes.io/metadata.name: default
  policies:
    verifyImageSignatures:
      - reference: docker.io/myrepo/*
        publicKeys:
        - |-
          -----BEGIN PUBLIC KEY-----
          MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
          5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
          -----END PUBLIC KEY-----
      - reference: company.registry.com/*
        dockerCfg: zxc==
        publicKeys:
        - |-
          -----BEGIN PUBLIC KEY-----
          MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
          5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
          -----END PUBLIC KEY-----
```

Policies do not affect the creation of pods whose container image addresses do not match those described in the `reference` parameter. If the address of any Pod container image matches those described in the `reference` policies, and the image is not signed or the signature does not correspond to the keys specified in the policy, the creation of the pod will be prohibited.

Example of an error output when creating a Pod with a container image that has not passed the signature verification:

```console
[verify-image-signatures] Image signature verification failed: nginx:1.17.2
```
