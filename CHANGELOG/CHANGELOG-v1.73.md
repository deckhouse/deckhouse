# Changelog v1.73

## Know before update


 - Fixes multiple security vulnerabilities that could affect authentication components

## Features


 - **[candi]** Added support of the new module csi-vsphere. [#14549](https://github.com/deckhouse/deckhouse/pull/14549)
 - **[cloud-provider-dvp]** Now cilium module is enabled if others CNIs not provided [#15084](https://github.com/deckhouse/deckhouse/pull/15084)
 - **[cloud-provider-dvp]** fix single-node LoadBalancer bug and add multi-LB per node support [#14883](https://github.com/deckhouse/deckhouse/pull/14883)
 - **[cloud-provider-zvirt]** Added support zvirt cloud provider to cse. [#14683](https://github.com/deckhouse/deckhouse/pull/14683)
 - **[deckhouse-controller]** add structured webhook response contract with status codes [#15256](https://github.com/deckhouse/deckhouse/pull/15256)
 - **[deckhouse-controller]** add new objects to debug archive [#15047](https://github.com/deckhouse/deckhouse/pull/15047)
 - **[deckhouse-controller]** Add conversion rules exposure in ModuleSettingsDefinition object [#15032](https://github.com/deckhouse/deckhouse/pull/15032)
    ModuleSettingsDefinition objects now include conversion rules in the conversions field, enabling users to preview how module settings will be transformed between versions
 - **[deckhouse-controller]** task queue performance improvements with linked list implementation [#14962](https://github.com/deckhouse/deckhouse/pull/14962)
 - **[deckhouse-controller]** handle errors while processing source modules [#14953](https://github.com/deckhouse/deckhouse/pull/14953)
 - **[ingress-nginx]** Added NGINX memory profiling for the Ingress controller. [#14736](https://github.com/deckhouse/deckhouse/pull/14736)
 - **[user-authn]** Propagate proxy envs to dex to allow requesting OIDC discovery endpoints in closed environments. [#15292](https://github.com/deckhouse/deckhouse/pull/15292)

## Fixes


 - **[candi]** kubectl-exec makes a direct request to the control plane if the proxy is unavailable [#15279](https://github.com/deckhouse/deckhouse/pull/15279)
 - **[cloud-provider-huaweicloud]** fix providerID format and exclude 127.0.0.0/8 in node IP selection [#15183](https://github.com/deckhouse/deckhouse/pull/15183)
 - **[cloud-provider-vcd]** Fix fetching VM templates from organization catalogs without direct access to organizastion [#14980](https://github.com/deckhouse/deckhouse/pull/14980)
 - **[cloud-provider-yandex]** Set CSI driver metadata querying timeouts [#15054](https://github.com/deckhouse/deckhouse/pull/15054)
 - **[deckhouse]** fix shell-operator http client to handle resources correctly [#15182](https://github.com/deckhouse/deckhouse/pull/15182)
 - **[deckhouse-controller]** fixed bug with re-enabled module using old values [#15045](https://github.com/deckhouse/deckhouse/pull/15045)
 - **[deckhouse-controller]** Implement structured releaseQueueDepth calculation with hierarchical version delta tracking [#15031](https://github.com/deckhouse/deckhouse/pull/15031)
    The releaseQueueDepth metric now accurately reflects actionable release gaps with patch version normalization; major version tracking added for future alerting
 - **[deckhouse-tools]** add tolerations support to DexAuthenticator configuration [#14869](https://github.com/deckhouse/deckhouse/pull/14869)
 - **[documentation]** add tolerations support to DexAuthenticator configuration [#14869](https://github.com/deckhouse/deckhouse/pull/14869)
 - **[ingress-nginx]** Disabled log messages such as `Error obtaining Endpoints for Service...`. [#15260](https://github.com/deckhouse/deckhouse/pull/15260)
 - **[istio]** Fixed metrics port for operator 1.25 and newer [#15124](https://github.com/deckhouse/deckhouse/pull/15124)
 - **[monitoring-kubernetes]** Add `tier=cluster` label [#15290](https://github.com/deckhouse/deckhouse/pull/15290)
 - **[openvpn]** ovpn-admin upgraded to fix the validation of static IP addresses, as well as add routes migration during the rotation of client certificates. [#14578](https://github.com/deckhouse/deckhouse/pull/14578)
    the openvpn instances will be restarted.
 - **[runtime-audit-engine]** reduced memory limit floor for falco containers [#15301](https://github.com/deckhouse/deckhouse/pull/15301)
 - **[user-authn]** Fix CVE-2025-30204, CVE-2025-22868, and CVE-2024-28180 in the user-authn module [#15208](https://github.com/deckhouse/deckhouse/pull/15208)
    Fixes multiple security vulnerabilities that could affect authentication components
 - **[user-authn]** User now can't create groups with  recursive loops in nested group's hierarchy [#15139](https://github.com/deckhouse/deckhouse/pull/15139)

## Chore


 - **[admission-policy-engine]** Fix CVE's [#15237](https://github.com/deckhouse/deckhouse/pull/15237)
 - **[candi]** Update Deckhouse CLI (d8) version to 0.17.0. [#15251](https://github.com/deckhouse/deckhouse/pull/15251)
 - **[candi]** Update Deckhouse CLI (d8) version to 0.16.0. [#15111](https://github.com/deckhouse/deckhouse/pull/15111)
 - **[cni-cilium]** Images for 1.17 were refactored to achieve distroless. [#14192](https://github.com/deckhouse/deckhouse/pull/14192)
 - **[ingress-nginx]** Switched to a distroless base image for the ingress controller v1.12, reducing its size and fixing multiple CVEs. [#14469](https://github.com/deckhouse/deckhouse/pull/14469)
    ingress controller pods will restart.
 - **[node-local-dns]** Build refactored and improved observability by adding alerts about resolving issues. [#14364](https://github.com/deckhouse/deckhouse/pull/14364)
 - **[registry]** Fixed CVE's: CVE-2020-26160, CVE-2020-8911, CVE-2020-8912, CVE-2022-21698, CVE-2022-2582, CVE-2025-22868, CVE-2025-22869, CVE-2025-22870, CVE-2025-22872, CVE-2025-27144 [#15235](https://github.com/deckhouse/deckhouse/pull/15235)

