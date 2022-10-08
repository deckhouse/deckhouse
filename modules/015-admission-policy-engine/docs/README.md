---
title: "admission-policy-engine"
---

Provides security policies in the cluster with [gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/)

## Pod Security Standards

The Pod Security Standards define three different policies to broadly cover the security spectrum. These policies are cumulative and range from highly-permissive to highly-restrictive.

- Privileged - Unrestricted policy, providing the widest possible level of permissions (used by default).
- Baseline - Minimally restrictive policy which prevents known privilege escalations. Allows the default (minimally specified) Pod configuration.
- Restricted - Heavily restricted policy, following current Pod hardening best practices.

You can read more about each set of policies [here](https://kubernetes.io/docs/concepts/security/pod-security-standards/)

To apply these policies you need to put a label on the desired namespace:
- `security.deckhouse.io/pod-policy=baseline`
- `security.deckhouse.io/pod-policy=restricted`

For example:

```bash
kubectl label ns my-namespace security.deckhouse.io/pod-policy=restricted
```

Will set the `Restricted` policy for all Pods in the Namespace `my-namespace`
