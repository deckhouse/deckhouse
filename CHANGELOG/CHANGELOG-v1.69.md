# Changelog v1.69

## [MALFORMED]


 - #10169 unknown section "monitoring-kubenetes"
 - #12144 missing section, missing summary, missing type, unknown section ""

## Know before update


 - Minimum supported version of Kubernetes was increased.

## Features


 - **[candi]** Add support of Kubernetes version 1.32 and drop support of version 1.27. [#11501](https://github.com/deckhouse/deckhouse/pull/11501)
    Minimum supported version of Kubernetes was increased.
 - **[cilium-hubble]** Add auth.allowedUserEmails option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[dashboard]** Add auth.allowedUserEmails option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[deckhouse]** Add module descriptions and tags. [#12189](https://github.com/deckhouse/deckhouse/pull/12189)
 - **[deckhouse]** Add hook to disable default ServiceAccount token automount. [#11954](https://github.com/deckhouse/deckhouse/pull/11954)
 - **[deckhouse-tools]** Add auth.allowedUserEmails option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[documentation]** Add auth.allowedUserEmails option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[ingress-nginx]** Added the ability to add custom annotations to the Ingress-controller pods. [#11522](https://github.com/deckhouse/deckhouse/pull/11522)
 - **[istio]** Add auth.allowedUserEmails option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[multitenancy-manager]** Add resource label/annotations. [#11933](https://github.com/deckhouse/deckhouse/pull/11933)
 - **[openvpn]** Add auth.allowedUserEmails option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[prometheus]** Add auth.allowedUserEmails option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[upmeter]** Add auth.allowedUserEmails option to restrict access to the application based on user email. [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[user-authn]** Restrict user access by email in DexClient and DexAuthenticator [#12055](https://github.com/deckhouse/deckhouse/pull/12055)
 - **[user-authz]** Add use dict support. [#11943](https://github.com/deckhouse/deckhouse/pull/11943)

## Fixes


 - **[admission-policy-engine]** Fix no data metrics [#11847](https://github.com/deckhouse/deckhouse/pull/11847)
 - **[cert-manager]** Restore the original webhook name to match the cert-manager's library regexp. [#12129](https://github.com/deckhouse/deckhouse/pull/12129)
 - **[cloud-provider-huaweicloud]** Fix `EIP` creation in `cloud-controller-manager`. [#12046](https://github.com/deckhouse/deckhouse/pull/12046)
 - **[control-plane-manager]** promote etcd member from learner state if needed [#11934](https://github.com/deckhouse/deckhouse/pull/11934)
 - **[prometheus]** Fix grafana CVEs [#12062](https://github.com/deckhouse/deckhouse/pull/12062)
 - **[prometheus]** Fix mimir and promxy CVEs [#11978](https://github.com/deckhouse/deckhouse/pull/11978)
 - **[vertical-pod-autoscaler]** VPA recommender memory-save option enable [#12077](https://github.com/deckhouse/deckhouse/pull/12077)

## Chore


 - **[candi]** Bump patch versions of Kubernetes images: `v1.29.14`, `v1.30.1`, `v1.31.6`, `v1.32.2` [#12080](https://github.com/deckhouse/deckhouse/pull/12080)
    Kubernetes control-plane components will restart, kubelet will restart.
 - **[dhctl]** Global refactoring converge operation [#11086](https://github.com/deckhouse/deckhouse/pull/11086)
 - **[extended-monitoring]** Separate alert of DeploymentReplicasUnavailable for Standby-Holder [#11905](https://github.com/deckhouse/deckhouse/pull/11905)
 - **[istio]** CVEs fixed. [#11991](https://github.com/deckhouse/deckhouse/pull/11991)
 - **[prometheus]** Increse priority class [#11904](https://github.com/deckhouse/deckhouse/pull/11904)
 - **[user-authz]** Add rbacv2 for dynamix and huaweicloud providers. [#12148](https://github.com/deckhouse/deckhouse/pull/12148)

