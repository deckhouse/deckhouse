#!/usr/bin/python3
from typing import Optional

from deckhouse import hook
from dotmap import DotMap

config = """
configVersion: v1
kubernetesValidating:
- group: main
  includeSnapshotsFrom:
  - prometheusremotewrites
  name: prometheusremotewrite-policy.deckhouse.io
  rules:
  - apiGroups:
    - deckhouse.io
    apiVersions:
    - v1alpha1
    - v1
    operations:
    - CREATE
    - UPDATE
    resources:
    - prometheusremotewrites
    scope: Cluster
kubernetes:
- name: prometheusremotewrites
  group: main
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  apiVersion: deckhouse.io/v1
  jqFilter: |
    {
      "name": .metadata.name,
      "url": .spec.url,
    }
  kind: PrometheusRemoteWrite
"""

def main(ctx: hook.Context):
    try:
        # DotMap is a dict with dot notation
        binding_context = DotMap(ctx.binding_context)
        message, allowed = validate(binding_context)
        if allowed:
            if message:
                ctx.output.validations.allow(message)  # warning
            else:
                ctx.output.validations.allow()
        else:
            ctx.output.validations.deny(message)
    except Exception as e:
        ctx.output.validations.error(str(e))

def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    operation = ctx.review.request.operation
    if operation == "CREATE" or operation == "UPDATE":
        return validate_creation_or_update(ctx)
    else:
        raise Exception(f"Unknown operation {ctx.operation}")


def validate_creation_or_update(ctx: DotMap) -> tuple[Optional[str], bool]:
    error = check_verify_url_signatures(ctx)
    if error is not None:
        return error, False
    error = check_verify_ca_signatures(ctx)
    if error is not None:
        return error, False
    return None, True


# check that all image references don't have intersection, it's required by ratify
# https://ratify.dev/docs/plugins/verifier/cosign/#scopes
def check_verify_url_signatures(ctx: DotMap) -> Optional[str]:
    url = ctx.review.request.object.spec.url
    if len(url) == 0:
        return "Url has empty string"
    filtered_name = ctx.review.request.name
    if len([rw for rw in ctx.snapshots.prometheusremotewrites if rw.filterResult.url == url and rw.filterResult.name != filtered_name]) > 0:
        return f"Remote write URL {url} is already in use"
    # search in all prometheusremote write if url alredy used
    return None

def check_verify_ca_signatures(ctx: DotMap) -> Optional[str]:
    ca = ctx.review.request.object.spec.tlsConfig.ca
    if len(ca) == 0:
        return None
    try:
        return None
    except Exception as e:
        return f"Certificate verification failed: {e}"


if __name__ == "__main__":
    hook.run(main, config=config)
