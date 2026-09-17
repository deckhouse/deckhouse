# Frozen Helm output

These files are the InfrastructureCluster and credentials Secret the node-manager chart rendered
before the rendering moved into node-controller. `helm_parity_test.go` compares what the
controller renders today against them, which is the whole evidence that the move changed no bytes.

## Where they came from

Rendered by the node-manager chart as it stood before this migration:
`modules/040-node-manager/templates/capi_cluster.yaml` fed each provider's `capi/cluster.yaml`
and `capi/credentials.yaml` through the `capi_infrastructure_cluster` define in
`templates/node-group/_capi_bootstrap_secret.tpl`. All of those are deleted now. Each provider's
`inputs.yaml` records the values the golden beside it was rendered from, and the test compares
that file against its own fixture on every run, so the inputs cannot drift away from the goldens
unnoticed.

## Checking a golden

A golden is evidence of what Helm emitted, not an expected value to refresh, and the chart that
produced it is gone, so nothing here is regenerated. Rewriting a golden to match what the
controller outputs today turns the test into a mirror of the code it is supposed to check.

When a golden is genuinely in doubt, read the deleted files out of history. The commit that
removed them is the one this test replaced:

    git log --diff-filter=D -- modules/040-node-manager/templates/capi_cluster.yaml
    git show <that commit>^:<module>/capi/cluster.yaml

The provider file is the template, and the define in `_capi_bootstrap_secret.tpl` next to it is
what Helm fed it through `tpl`. Render it with the values in the provider's `inputs.yaml`.

When the fixture in the test changes on purpose, rewrite the input records with:

    UPDATE_PARITY_INPUTS=1 go test ./internal/cloudprovider/

## The one deviation

`huaweicloud/cluster.yaml` holds `spec: null`, exactly as Helm emitted it. The contract renders
`spec: {}` instead, so node-controller can write the live `controlPlaneEndpoint` back into it.
The test patches the expectation in that one place rather than editing this file: the file states
what Helm did, the test states why the controller is allowed to differ.
