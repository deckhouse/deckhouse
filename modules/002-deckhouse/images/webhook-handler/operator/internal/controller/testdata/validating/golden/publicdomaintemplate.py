#!/usr/bin/python3
from typing import Optional

from deckhouse import hook
from dotmap import DotMap

config = """
configVersion: v1
kubernetesValidating:
- group: main
  includeSnapshotsFrom:
  - d8-kube-dns-cm-wh
  - d8-cluster-configuration
  - kube-dns-module
  name: publicdomaintemplate-policy.deckhouse.io
  rules:
  - apiGroups:
    - deckhouse.io
    apiVersions:
    - '*'
    operations:
    - CREATE
    - UPDATE
    resources:
    - moduleconfigs
    scope: Cluster
kubernetes:
- name: d8-kube-dns-cm-wh
  group: main
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  apiVersion: v1
  jqFilter: |-
    {
      "kube-dns-conf": .data.Corefile,
    }
  kind: ConfigMap
  nameSelector:
    matchNames:
    - d8-kube-dns
  namespace:
    nameSelector:
      matchNames:
      - kube-system
- name: d8-cluster-configuration
  group: main
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  apiVersion: v1
  jqFilter: |-
    {
      "clusterDomain": ( .data."cluster-configuration.yaml" // "" | @base64d | match("[ ]*clusterDomain:[ ]+(.+)").captures[0].string),
    }
  kind: Secret
  nameSelector:
    matchNames:
    - d8-cluster-configuration
  namespace:
    nameSelector:
      matchNames:
      - kube-system
- name: kube-dns-module
  group: main
  executeHookOnEvent: []
  executeHookOnSynchronization: false
  keepFullObjectsInMemory: false
  apiVersion: deckhouse.io/v1alpha1
  jqFilter: |-
    {
      "status": .status.status,
    }
  kind: Module
  nameSelector:
    matchNames:
    - kube-dns
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

import re

def validate(ctx: DotMap) -> tuple[Optional[str], bool]:
    mc_name = ctx.review.request.object.metadata.name
    if mc_name != "global":
        return None, True

    module_kube_dns_snapshots = ctx.snapshots.get("kube-dns-module", [])
    module_status = None
    for snap in module_kube_dns_snapshots:
        module_status = snap.filterResult.get("status")

    if module_status == "Ready":
        kube_dns_snapshots = ctx.snapshots.get("d8-kube-dns-cm-wh", [])
        kube_dns_conf = ""
        for snap in kube_dns_snapshots:
            kube_dns_conf = snap.filterResult.get("kube-dns-conf", "")
        dns_domains = re.findall(r'(?<=kubernetes )(.*?)(?=ip6\.arpa)', kube_dns_conf)
    else:
        cluster_config_snapshots = ctx.snapshots.get("d8-cluster-configuration", [])
        dns_domains = []
        for snap in cluster_config_snapshots:
            domain = snap.filterResult.get("clusterDomain", "")
            if domain:
                dns_domains.append(domain)

    public_domain_template = ctx.review.request.object.spec.settings.modules.get(
        "publicDomainTemplate", ""
    )
    if public_domain_template:
        public_domain_template = public_domain_template.replace("%s.", "")

    for domain in dns_domains:
        domain = domain.strip()
        if public_domain_template == domain:
            domain_list = " ".join(d.strip() for d in dns_domains)
            return (
                f'The publicDomainTemplate "%s.{public_domain_template}" MUST NOT '
                f'match the one specified in the clusterDomain/clusterDomainAliases '
                f'parameters of the resource: ["{domain_list}"].',
                False
            )
    return None, True


if __name__ == "__main__":
    hook.run(main, config=config)
