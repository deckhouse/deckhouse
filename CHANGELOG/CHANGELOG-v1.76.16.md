# Changelog v1.76.16

## Fixes


 - **[cloud-provider-vcd]** Fixes cluster bootstrap getting stuck when `capcd-controller-manager` cannot reach the API server and DNS before the CNI is ready. [#23358](https://github.com/deckhouse/deckhouse/pull/23358)
 - **[istio]** Aligned CNI templates with upstream and fixed Istio 1.25 compatibility. [#160](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/160)
 - **[istio]** Fix leaking istiod control plane after removing an Istio version or disabling the module. [#255](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/255)
 - **[istio]** Fixed the istio module getting stuck when switching `globalVersion` from 1.21 while proxies are still connected to the 1.21 control plane. [#201](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/201)
