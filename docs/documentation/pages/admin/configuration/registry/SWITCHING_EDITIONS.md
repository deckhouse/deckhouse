---
title: "Switching editions"
permalink: en/admin/configuration/registry/switching-editions.html
---

## Switching DKP to CE/BE/SE/SE+/EE

Switching DKP to CE/BE/SE/SE+/EE can be done in one of the following ways:

- [using the `registry` module](#switching-using-the-registry-module);
- [without using the `registry` module](#switching-without-using-the-registry-module).

{% alert level="warning" %}

- The functionality of this guide is validated for Deckhouse versions starting from `v1.70`. If your version is older, use the corresponding documentation.
- For commercial editions, you need a valid license key that supports the desired edition. If necessary, you can [request a temporary key](/products/enterprise_edition.html).
- The guide assumes the use of the public container registry address: `registry.deckhouse.io`. If you are using a different container registry address, modify the commands accordingly or refer to the [guide on switching Deckhouse to use a different registry](./third-party.html).
- The Deckhouse CE/BE/SE/SE+ editions do not support the cloud providers `dynamix`, `openstack`, `VCD`, and `vSphere` (vSphere is supported in SE+) and a number of modules.
- All commands are executed on the master node of the existing cluster with `root` user.
{% endalert %}

### Switching using the registry module

1. Make sure the cluster has been migrated to be managed by the [`registry` module](/modules/registry/faq.html#how-to-migrate-to-the-registry-module).  
If the cluster is not managed by the `registry` module, proceed to the [instruction](#switching-without-using-the-registry-module).

1. Prepare variables for the license token and new edition name:

    > It is not necessary to fill the `NEW_EDITION` variable when switching to Deckhouse CE edition.
    > The `NEW_EDITION` variable should match your desired Deckhouse edition. For example, to switch to:
    - CE, the variable should be `ce`;
    - BE, the variable should be `be`;
    - SE, the variable should be `se`;
    - SE+, the variable should be `se-plus`;
    - EE, the variable should be `ee`.

    ```shell
    NEW_EDITION=<PUT_YOUR_EDITION_HERE>
    LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
    ```

1. Ensure the Deckhouse queue is empty and error-free:

   ```shell
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

   Example of the output (queues are empty):

   ```console
   Summary:
   - 'main' queue: empty.
   - 88 other queues (0 active, 88 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Start a temporary pod for the new Deckhouse edition to obtain current digests and a list of modules:

   For the CE edition:

   ```shell
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   d8 k run $NEW_EDITION-image --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   For other editions:

   ```shell
   d8 k create secret docker-registry $NEW_EDITION-image-pull-secret \
    --docker-server=registry.deckhouse.ru \
    --docker-username=license-token \
    --docker-password=${LICENSE_TOKEN}

   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
   d8 k run $NEW_EDITION-image \
    --image=registry.deckhouse.ru/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION \
    --overrides="{\"spec\": {\"imagePullSecrets\":[{\"name\": \"$NEW_EDITION-image-pull-secret\"}]}}" \
    --command sleep -- infinity
   ```

   Once the pod is in `Running` state, execute the following commands:

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
   echo $MODULES_WILL_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   Wait for the Deckhouse pod to reach `Ready` state and ensure all tasks in the queue are completed:

   ```shell
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

   Example of the output (queues are empty):

   ```console
   Summary:
   - 'main' queue: empty.
   - 88 other queues (0 active, 88 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Delete the created Secret and Pod:

   ```shell
   d8 k delete pod/$NEW_EDITION-image
   d8 k delete secret/$NEW_EDITION-image-pull-secret
   ```

1. Perform the switch to the new edition. To do this, specify the following parameter in the `deckhouse` ModuleConfig. For detailed configuration, refer to the [`deckhouse`](/modules/deckhouse/) module documentation.

   ```yaml
   ---
   # Example for Direct mode
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Direct
         direct:
           # Relax mode is used to check for the presence of the current Deckhouse version in the specified registry.
           # This mode must be used to switch between editions.
           checkMode: Relax
           # Specify your value for <NEW_EDITION>.
           imagesRepo: registry.deckhouse.ru/deckhouse/<NEW_EDITION>
           scheme: HTTPS
           # Specify your value for <LICENSE_TOKEN>.
           # If switching to the CE edition, remove this parameter.
           license: <LICENSE_TOKEN>
   ---
   # Example for Unmanaged mode.
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           # Relax mode is used to check for the presence of the current Deckhouse version in the specified registry.
           # This mode must be used to switch between editions.
           checkMode: Relax
           # Specify your value for <NEW_EDITION>.
           imagesRepo: registry.deckhouse.ru/deckhouse/<NEW_EDITION>
           scheme: HTTPS
           # Specify your value for <LICENSE_TOKEN>.
           # If switching to the CE edition, remove this parameter.
           license: <LICENSE_TOKEN>
   ```

1. Wait for the registry to switch. To verify the switch progress, follow the [instruction](/modules/registry/faq.html#how-to-check-the-registry-mode-switch-status).

   Example output:

   ```yaml
   conditions:
     - lastTransitionTime: "..."
       message: |-
         Mode: Relax
         registry.deckhouse.ru: all 1 items are checked
       reason: Ready
       status: "True"
       type: RegistryContainsRequiredImages
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. After the switch, remove the `checkMode: Relax` parameter from the `deckhouse` ModuleConfig to revert to the default check mode.  
Removing this parameter will trigger a check for the presence of critical components in the registry.

1. Wait for the check to complete by following the [instruction](/modules/registry/faq.html#how-to-check-the-registry-mode-switch-status).

   Example output:

   ```yaml
   conditions:
     - lastTransitionTime: "..."
       message: |-
         Mode: Default
         registry.deckhouse.ru: all 155 items are checked
       reason: Ready
       status: "True"
       type: RegistryContainsRequiredImages
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   ```

1. Check if there are any pods with the Deckhouse old edition address left in the cluster, where `<YOUR-PREVIOUS-EDITION>` your previous edition name:

   For Unmanaged mode:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.ru/deckhouse/<YOUR-PREVIOUS-EDITION>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   For other modes that use a fixed registry address.  
   This check does not take external modules into account:  

   ```shell
   # Get the list of valid digest values from the images_digests.json file inside Deckhouse.
   IMAGES_DIGESTS=$(d8 k -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- cat /deckhouse/modules/images_digests.json | jq -r '.[][]' | sort -u)

   # Check for Pods using Deckhouse images from `registry.d8-system.svc:5001/system/deckhouse`
   # with a digest that is NOT present in the list of valid digest values (IMAGES_DIGESTS).
   d8 k get pods -A -o json |
   jq -r --argjson digests "$(printf '%s\n' $IMAGES_DIGESTS | jq -R . | jq -s .)" '
     .items[]
     | {name: .metadata.name, namespace: .metadata.namespace, containers: .spec.containers}
     | select(.containers != null)
     | select(
         .containers[]
         | select(.image | test("registry.d8-system.svc:5001/system/deckhouse") and test("@sha256:"))
         | .image as $img
         | ($img | split("@") | last) as $digest
         | ($digest | IN($digests[]) | not)
       )
     | .namespace + "\t" + .name
   ' | sort -u
   ```

### Switching without using the registry module

1. If the `registry` module is enabled, disable it by following [instruction](/modules/registry/faq.html#how-to-migrate-back-from-the-registry-module).

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
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

   Example of the output (queues are empty):

   ```console
   Summary:
   - 'main' queue: empty.
   - 88 other queues (0 active, 88 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Create a `NodeGroupConfiguration` resource for temporary authorization in `registry.deckhouse.io`:

   > Before creating a resource, refer to the section [How to add configuration for an additional registry](/modules/node-manager/faq.html#how-to-add-configuration-for-an-additional-registry)
   >
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
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
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
   echo $MODULES_WILL_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   Wait for the Deckhouse pod to reach `Ready` state and ensure all tasks in the queue are completed:

   ```shell
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

   Example of the output (queues are empty):

   ```console
   Summary:
   - 'main' queue: empty.
   - 88 other queues (0 active, 88 empty): 0 tasks.
   - no tasks to handle.
   ```

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

1. Check if there are any pods with the Deckhouse old edition address left in the cluster, where `<YOUR-PREVIOUS-EDITION>` your previous edition name:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.io/deckhouse/<YOUR-PREVIOUS-EDITION>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

1. Delete temporary files, the `NodeGroupConfiguration` resource, and variables:

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
