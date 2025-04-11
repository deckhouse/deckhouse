# Changelog v1.70

## Features


 - **[candi]** Deleting all users created by deckhouse from NodeUser manifests. [#12908](https://github.com/deckhouse/deckhouse/pull/12908)
 - **[candi]** Added a `bashible` step that assigns the `node.deckhouse.io/provider-id` annotation to nodes with a `static://` provider ID [#11807](https://github.com/deckhouse/deckhouse/pull/11807)
 - **[dhctl]** Add waiting for become ready first master node [#12918](https://github.com/deckhouse/deckhouse/pull/12918)
 - **[openvpn]** Added end-of-life alerts, CA certificate re-creation and a grafana dashboard. [#12581](https://github.com/deckhouse/deckhouse/pull/12581)

## Fixes


 - **[candi]** bashible configure-kubelet step fix [#12722](https://github.com/deckhouse/deckhouse/pull/12722)
 - **[cloud-provider-openstack]** fix terraform bastion default root_disk_size [#12924](https://github.com/deckhouse/deckhouse/pull/12924)
 - **[cni-cilium]** Added restoring/hiding network access to cilium endpoint (cep) when higher/lower priority cep was removed/added. [#12793](https://github.com/deckhouse/deckhouse/pull/12793)
 - **[dhctl]** Add deny additional properties for validation schema eg module config [#12889](https://github.com/deckhouse/deckhouse/pull/12889)
 - **[dhctl]** Added waiting for kubeadm command completion result [#12826](https://github.com/deckhouse/deckhouse/pull/12826)
 - **[metallb]** Dashboards are aligned with user experience expectations. [#12666](https://github.com/deckhouse/deckhouse/pull/12666)
 - **[node-manager]** Rewrite static Node adoption for `CAPS` [#11807](https://github.com/deckhouse/deckhouse/pull/11807)

## Chore


 - **[dhctl]** Set additionalProperties "false" for all objects in openapi [#11832](https://github.com/deckhouse/deckhouse/pull/11832)
 - **[ingress-nginx]** Added ingress-nginx version 1.12. The defaultControllerVersion is set to 1.10, all ingress controllers without specified version will restart. [#12609](https://github.com/deckhouse/deckhouse/pull/12609)

