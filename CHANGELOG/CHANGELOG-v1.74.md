# Changelog v1.74

## Features


 - **[admission-policy-engine]** Validate CONNECT requests for pods/exec and pods/attach in the ValidatingWebhookConfiguration [#15872](https://github.com/deckhouse/deckhouse/pull/15872)
    Enables Gatekeeper constraints to act on CONNECT (kubectl exec) events; default behavior unchanged unless such constraints are created.

