---
title: "Switching editions"
permalink: en/admin/configuration/registry/switching-editions.html
---

## Switching DKP to CE/BE/SE/SE+/EE

{% alert level="warning" %}
When using the [`registry`](/modules/registry/) module, switching between editions is only possible in `Unmanaged` mode.  
To switch to `Unmanaged` mode, follow the [instruction](/modules/registry/examples.html).
{% endalert %}

{% alert level="warning" %}
- The functionality of this guide is validated for Deckhouse versions starting from `v1.70`. If your version is older, use the corresponding documentation.
- For commercial editions, you need a valid license key that supports the desired edition. If necessary, you can [request a temporary key](/products/enterprise_edition.html).
- The guide assumes the use of the public container registry address: `registry.deckhouse.io`. If you are using a different container registry address, modify the commands accordingly or refer to the [guide on switching Deckhouse to use a different registry](./third-party.html).
- The Deckhouse CE/BE/SE/SE+ editions do not support the cloud providers `dynamix`, `openstack`, `VCD`, and `vSphere` (vSphere is supported in SE+) and a number of modules.
- All commands are executed on the master node of the existing cluster with `root` user.
{% endalert %}

1. Prepare variables for the license token and new edition name:

    > It is not necessary to fill the `NEW_EDITION` and `AUTH_STRING` variables when switching to Deckhouse CE edition.
    The `NEW_EDITION` variable should match your desired Deckhouse edition. For example, to switch to:
    - CE, the variable should be `ce`;
    - BE, the variable should be `be`;
    - SE, the variable should be `se`;
    - SE+, the variable should be `se-plus`;
    - EE, the variable should be `ee`.

    ```shell
    NEW_EDITION=<PUT_YOUR_EDITION_HERE>
    LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
    AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
    ```

1. Ensure the Deckhouse queue is empty and error-free:

   ```shell
   d8 system queue list
   ```

1. Create a [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) resource for temporary authorization in `registry.deckhouse.io`:

   > Skip this step if switching to Deckhouse CE.

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-$NEW_EDITION-config.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 30
     content: |
       _on_containerd_config_changed() {
         bb-flag-set containerd-need-restart
       }
       bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'
       mkdir -p /etc/containerd/conf.d
       bb-sync-file /etc/containerd/conf.d/$NEW_EDITION-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry.deckhouse.io".auth]
               auth = "$AUTH_STRING"
       EOF_TOML
   EOF
   ```

   Wait for the `/etc/containerd/conf.d/$NEW_EDITION-registry.toml` file to appear on the nodes and for bashible synchronization to complete. To track the synchronization status, check the `UPTODATE` value (the number of nodes in this status should match the total number of nodes (`NODES`) in the group):

   ```shell
   d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Example output:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   Also, a message stating `Configuration is in sync, nothing to do` should appear in the systemd service log for bashible by executing the following command:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Annotate node master-ee-to-se-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master ee-to-se-0 bashible.sh[53407]: Successful annotate node master-ee-to-se-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-se-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Start a temporary pod for the new Deckhouse edition to obtain current digests and a list of modules:

   ```shell
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   d8 k run $NEW_EDITION-image --image=registry.deckhouse.io/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION --command sleep --infinity
   ```

1. Once the pod is in `Running` state, execute the following commands:

   ```shell
   NEW_EDITION_MODULES=$(d8 k exec $NEW_EDITION-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
   USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $NEW_EDITION_MODULES | tr ' ' '\n'))
   ```

1. Verify that the modules used in the cluster are supported in the desired edition. To see the list of modules not supported in the new edition and will be disabled:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Check the list to ensure the functionality of these modules is not in use in your cluster and you are ready to disable them.

   Disable the modules not supported by the new edition:

   ```shell
   echo $MODULES_WILL_DISABLE | tr ' ' '\n' | awk {'print "d8 system module disable",$1'} | bash
   ```

   Wait for the Deckhouse pod to reach `Ready` state and [ensure all tasks in the queue are completed](#how-to-check-the-job-queue-in-deckhouse).

1. Execute the `deckhouse-controller helper change-registry` command from the Deckhouse pod with the new edition parameters:

   To switch to BE/SE/SE+/EE editions:

   ```shell
   DOCKER_CONFIG_JSON=$(echo -n "{\"auths\": {\"registry.deckhouse.io\": {\"username\": \"license-token\", \"password\": \"${LICENSE_TOKEN}\", \"auth\": \"${AUTH_STRING}\"}}}" | base64 -w 0)
   d8 k --as system:sudouser -n d8-cloud-instance-manager patch secret deckhouse-registry --type merge --patch="{\"data\":{\".dockerconfigjson\":\"$DOCKER_CONFIG_JSON\"}}"  
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user=license-token --password=$LICENSE_TOKEN --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.io/deckhouse/$NEW_EDITION
   ```

   To switch to CE edition:

   ```shell
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.io/deckhouse/ce
   ```

1. Check if there are any pods with the DKP old edition address left in the cluster, where `<YOUR-PREVIOUS-EDITION>` your previous edition name:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.io/deckhouse/<YOUR-PREVIOUS-EDITION>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

1. Delete temporary files, the NodeGroupConfiguration resource, and variables:

   > Skip this step if switching to Deckhouse CE.

   ```shell
   d8 k delete ngc containerd-$NEW_EDITION-config.sh
   d8 k delete pod $NEW_EDITION-image
   d8 k apply -f - <<EOF
       apiVersion: deckhouse.io/v1alpha1
       kind: NodeGroupConfiguration
       metadata:
         name: del-temp-config.sh
       spec:
         nodeGroups:
         - '*'
         bundles:
         - '*'
         weight: 90
         content: |
           if [ -f /etc/containerd/conf.d/$NEW_EDITION-registry.toml ]; then
             rm -f /etc/containerd/conf.d/$NEW_EDITION-registry.toml
           fi
   EOF
   ```

   After the bashible synchronization completes (synchronization status on the nodes is shown by the `UPTODATE` value in NodeGroup), delete the created NodeGroupConfiguration resource:

   ```shell
   d8 k delete ngc del-temp-config.sh
   ```
