---
title: Switching a running DKP cluster to use an external registry
permalink: en/admin/configuration/registry/third-party.html
---

{% alert level="warning" %}
Using registries other than `registry.deckhouse.io` and `registry.deckhouse.ru` is only available in commercial editions of the Deckhouse Kubernetes Platform.
{% endalert %}

To switch the cluster to use an external registry, follow these steps:

1. Run the `deckhouse-controller helper change-registry` command from the DKP pod with the parameters of the new registry.  
   Example:

   ```shell
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee
   ```

1. If the registry uses self-signed certificates, place the corresponding root certificate in the file `/tmp/ca.crt` inside the DKP pod and add the -`-ca-file /tmp/ca.crt` option to the command.
   Alternatively, insert the CA content into a variable, as shown below:

   ```shell
   CA_CONTENT=$(cat <<EOF
   -----BEGIN CERTIFICATE-----
   CERTIFICATE
   -----END CERTIFICATE-----
   -----BEGIN CERTIFICATE-----
   CERTIFICATE
   -----END CERTIFICATE-----
   EOF
   )
   d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- bash -c "echo '$CA_CONTENT' > /tmp/ca.crt && deckhouse-controller helper change-registry --ca-file /tmp/ca.crt --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee"
   ```

   To view the list of available flags for the `deckhouse-controller helper change-registry` command, run:

   ```shell
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --help
   ```

   Example output:

   ```console
   usage: deckhouse-controller helper change-registry [<flags>] <new-registry>
   Change registry for deckhouse images.
   Flags:
     --help               Show context-sensitive help (also try --help-long and --help-man).
     --user=USER          User with pull access to registry.
     --password=PASSWORD  Password/token for registry user.
     --ca-file=CA-FILE    Path to registry CA.
     --scheme=SCHEME      Used scheme while connecting to registry, http or https.
     --dry-run            Don't change deckhouse resources, only print them.
     --new-deckhouse-tag=NEW-DECKHOUSE-TAG
                         New tag that will be used for deckhouse deployment image (by default
                          current tag from deckhouse deployment will be used).
   Args:
     <new-registry>  Registry that will be used for deckhouse images (example:
                     registry.deckhouse.io/deckhouse/ce). By default, https will be used, if you need
                     http - provide '--scheme' flag with http value
   ```

1. Wait until the registry pod reaches the `Ready` status. If the pod is in the `ImagePullBackoff` state, restart it.
1. Wait for bashible to apply the new settings on the master node.

   Check the bashible system service log on the master node, for example, using the following command:

   ```shell
   journalctl -u bashible -n 20
   ```

   The log should contain the message `Configuration is in sync, nothing to do`.

   Example of output when viewing the bashible service log:

   ```console
   $ journalctl -u bashible -n 20
   ...
   Aug 13 05:03:08 kube-master-0 systemd[1]: Started Bashible service.
   Aug 13 05:03:10 kube-master-0 bash[1847265]: Configuration is in sync, nothing to do.   <--
   Aug 13 05:03:10 kube-master-0 systemd[1]: bashible.service: Deactivated successfully.
   Aug 13 05:03:10 kube-master-0 systemd[1]: bashible.service: Consumed 1.075s CPU time.
   ```

1. If you need to disable automatic registry updates via the external registry, remove the `releaseChannel` parameter from the `deckhouse` module configuration.
1. Check if any pods in the cluster are still using the original registry address:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | startswith("registry.deckhouse"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```
