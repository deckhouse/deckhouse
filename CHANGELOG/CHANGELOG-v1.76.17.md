# Changelog v1.76.17

## Know before update


 - A diff will appear in the Terraform state for the base infrastructure: the E zone will be added. Because of this, the Terraform check will fail on the first master node. Run `dhctl converge` for the base infrastructure, and everything should get back to normal.

## Features


 - **[cloud-provider-yandex]** Add support for the `ru-central1-e` zone in Yandex Cloud. [#550](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/550)
    A diff will appear in the Terraform state for the base infrastructure: the E zone will be added. Because of this, the Terraform check will fail on the first master node. Run `dhctl converge` for the base infrastructure, and everything should get back to normal.

## Fixes


 - **[cloud-provider-azure]** use .normal.apiserverEndpoints in data device Secret fallback [#696](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/696)
 - **[cloud-provider-vsphere]** Fixed master node creation when the datastore is set as a path with parent folders. [#362](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/362)
 - **[deckhouse]** Restore kubectl plugin lookup in d8 k and d8 kubectl [#587](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/587)
 - **[dhctl]** Prevent dhctl converge from panicking when kubeconfig is used without an SSH host. [#499](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/499)
 - **[istio]** The Istio operator now recovers Helm releases stuck in the `pending-*` or `uninstalling` state. [#261](https://fox.flant.com/deckhouse/deckhouse/-/merge_requests/261)
