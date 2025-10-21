---
title: "Switching editions"
permalink: en/admin/configuration/registry/switching-editions.html
---

## Switching DKP from CE to EE

A valid license key is required. If needed, you can [request a temporary license](/products/enterprise_edition.html).

{% alert level="warning" %}
This instruction assumes the use of the public container registry: `registry.deckhouse.ru`.
{% endalert %}

To switch from Deckhouse Community Edition to Enterprise Edition, follow these steps  
(all commands should be executed on a master node either as a user with a configured `kubectl` context or with superuser privileges):

1. Prepare variables with your license token:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64)"
   ```

   Create a [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) resource to enable transitional authorization to `registry.deckhouse.ru`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-ee-config.sh
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
       bb-sync-file /etc/containerd/conf.d/ee-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry.deckhouse.ru".auth]
               auth = "$AUTH_STRING"
       EOF_TOML

   EOF
   ```

   Wait until the `/etc/containerd/conf.d/ee-registry.toml` file appears on the nodes and Bashible synchronization is complete.

   You can monitor the sync status using the `UPTODATE` column (the number of `UPTODATE` nodes should match the total number of nodes in the group):

   ```shell
   d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Example output:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   You should also see the message `Configuration is in sync, nothing to do.` in the bashible systemd service logs, for example:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ce-to-ee-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ce-to-ee-0 bashible.sh[53407]: Annotate node master-ce-to-ee-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master ce-to-ee-0 bashible.sh[53407]: Successful annotate node master-ce-to-ee-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ce-to-ee-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

   Then, launch a temporary DKP EE pod to retrieve the latest image digests and module list:

   ```shell
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   d8 k run ee-image --image=registry.deckhouse.ru/deckhouse/ee/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   To verify which DKP version is currently deployed:

   ```shell
   d8 k get deckhousereleases | grep Deployed
   ```

1. Once the pod reaches the `Running` state, execute the following commands:

   Retrieve the value of `EE_REGISTRY_PACKAGE_PROXY`:

   ```shell
   EE_REGISTRY_PACKAGE_PROXY=$(d8 k exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".registryPackagesProxy.registryPackagesProxy")
   ```

   Pull the Deckhouse EE image using the obtained digest:

   ```shell
   crictl pull registry.deckhouse.ru/deckhouse/ee@$EE_REGISTRY_PACKAGE_PROXY
   ```

   Example output:

   ```console
   Image is up to date for sha256:8127efa0f903a7194d6fb7b810839279b9934b200c2af5fc416660857bfb7832
   ```

1. Update the DKP registry access secret by running the following command:

   ```shell
   d8 k -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry.deckhouse.ru\": { \"username\": \"license-token\", \"password\": \"$LICENSE_TOKEN\", \"auth\":    \"$AUTH_STRING\" }}}" \
     --from-literal="address"=registry.deckhouse.ru \
     --from-literal="path"=/deckhouse/ee \
     --from-literal="scheme"=https \
     --type=kubernetes.io/dockerconfigjson \
     --dry-run='client' \
     -o yaml | kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- kubectl replace -f -
   ```

1. Apply the webhook-handler image:

   ```shell
   HANDLER=$(d8 k exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".deckhouse.webhookHandler")
   d8 k --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/webhook-handler handler=registry.deckhouse.ru/deckhouse/ee@$HANDLER
   ```

1. Apply the Deckhouse EE image:

   ```shell
   DECKHOUSE_KUBE_RBAC_PROXY=$(d8 k exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   DECKHOUSE_INIT_CONTAINER=$(d8 k exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   d8 k --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/deckhouse init-downloaded-modules=registry.deckhouse.ru/deckhouse/ee@$DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry.deckhouse.ru/deckhouse/ee@$DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry.deckhouse.ru/deckhouse/ee:$DECKHOUSE_VERSION
   ```

1. Wait for the Deckhouse pod to reach the `Ready` status and for [all tasks in the queue](../platform-scaling/control-plane/control-plane-management-and-configuration.html#checking-dkp-status-and-queues) to complete.  
   If you encounter the `ImagePullBackOff` error during this process, wait for the pod to restart automatically.

   Check the status of the DKP pod:

   ```shell
   d8 k -n d8-system get po -l app=deckhouse
   ```

   Check the DKP task queue:

   ```shell
   d8 platform queue list
   ```

1. Check if any pods are still using the CE registry address:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
      | select(.image | contains("deckhouse.ru/deckhouse/ce"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

1. Clean up temporary files, the NodeGroupConfiguration resource, and variables:

   ```shell
   d8 k delete ngc containerd-ee-config.sh
   d8 k delete pod ee-image
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
           if [ -f /etc/containerd/conf.d/ee-registry.toml ]; then
             rm -f /etc/containerd/conf.d/ee-registry.toml
           fi
   EOF
   ```

   After Bashible synchronization (you can track it by the `UPTODATE` status of the NodeGroup), delete the temporary configuration resource:

   ```shell
   d8 k delete ngc del-temp-config.sh
   ```

## Switching DKP from EE to CE

{% alert level="warning" %}
This instruction assumes the use of the public container registry: `registry.deckhouse.ru`.  
Using registries other than `registry.deckhouse.io` and `registry.deckhouse.ru` is only available in commercial editions of Deckhouse Kubernetes Platform.

Cloud clusters on OpenStack and VMware vSphere are not supported in DKP CE.
{% endalert %}

To switch from Deckhouse Enterprise Edition to Community Edition, follow these steps  
(all commands should be executed on a master node either as a user with a configured `kubectl` context or with superuser privileges):

1. To retrieve the current image digests and module list, create a temporary DKP CE pod using the following command:

   ```shell
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   d8 k run ce-image --image=registry.deckhouse.ru/deckhouse/ce/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   This will run the image of the latest installed DKP version in the cluster.

   To determine the currently installed version, use:

   ```shell
   d8 k get deckhousereleases | grep Deployed
   ```

1. Once the pod enters the `Running` state, execute the following commands:

   Retrieve the `CE_REGISTRY_PACKAGE_PROXY` value:

   ```shell
   CE_REGISTRY_PACKAGE_PROXY=$(d8 k exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".registryPackagesProxy.registryPackagesProxy")
   ```

   Pull the DKP CE image using the obtained digest:

   ```shell
   crictl pull registry.deckhouse.ru/deckhouse/ce@$CE_REGISTRY_PACKAGE_PROXY
   ```

   Example output:

   ```console
   Image is up to date for sha256:8127efa0f903a7194d6fb7b810839279b9934b200c2af5fc416660857bfb7832
   ```

   Retrieve the list of `CE_MODULES`:

   ```shell
   CE_MODULES=$(d8 k exec ce-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
   ```

   Check the result:

   ```shell
   echo $CE_MODULES
   ```

   Example output:

   ```console
   common priority-class deckhouse external-module-manager registrypackages ...
   ```

   Retrieve the list of currently enabled embedded modules:

   ```shell
   USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   ```

   Verify the result:

   ```shell
   echo $USED_MODULES
   ```

   Example output:

   ```console
   admission-policy-engine cert-manager chrony ...
   ```

   Determine which modules will be disabled after switching to CE:

   ```shell
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $CE_MODULES | tr ' ' '\n'))
   ```

   Verify the result:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   Example output:

   ```console
   node-local-dns registry-packages-proxy
   ```

   > If `registry-packages-proxy` appears in `$MODULES_WILL_DISABLE`, it must be manually re-enabled. Otherwise, the cluster will not be able to switch to DKP CE images. Instructions for re-enabling it are provided in Step 8.

1. Make sure that the modules currently used in the cluster are supported in DKP CE.

   To display the list of modules that are **not supported** and will be disabled:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   Review the list carefully and make sure that the functionality provided by these modules is not critical for your cluster, and that you are ready to disable them.

   To disable unsupported modules:

   ```shell
   echo $MODULES_WILL_DISABLE |
     tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   Example output:

   ```console
   Defaulted container "deckhouse" out of: deckhouse, kube-rbac-proxy, init-external-modules (init)
   Module node-local-dns disabled
   ```

1. Update the DKP registry access secret by running the following command:

   ```bash
   d8 k -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry.deckhouse.ru\": {}}}" \
     --from-literal="address"=registry.deckhouse.ru \
     --from-literal="path"=/deckhouse/ce \
     --from-literal="scheme"=https \
     --type=kubernetes.io/dockerconfigjson \
     --dry-run='client' \
     -o yaml | kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- kubectl replace -f -
   ```

1. Apply the `webhook-handler` image:

   ```shell
   HANDLER=$(d8 k exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".deckhouse.webhookHandler")
   d8 k --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/webhook-handler handler=registry.deckhouse.ru/deckhouse/ce@$HANDLER
   ```

1. Apply the DKP CE image:

   ```shell
   DECKHOUSE_KUBE_RBAC_PROXY=$(d8 k exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   DECKHOUSE_INIT_CONTAINER=$(d8 k exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   d8 k --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/deckhouse init-downloaded-modules=registry.deckhouse.ru/deckhouse/ce@$DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry.deckhouse.ru/deckhouse/ce@$DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry.deckhouse.ru/deckhouse/ce:$DECKHOUSE_VERSION
   ```

1. Wait for the DKP pod to reach the `Ready` status and for [all tasks in the queue](../platform-scaling/control-plane/control-plane-management-and-configuration.html#checking-dkp-status-and-queues) to complete.  
If you encounter the `ImagePullBackOff` error during this process, wait for the pod to restart automatically.

   Check the status of the DKP pod:

   ```shell
   d8 k -n d8-system get po -l app=deckhouse
   ```

   Check the DKP task queue:

   ```shell
   d8 platform queue list
   ```

1. Check if any pods in the cluster are still using the DKP EE registry address:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
      | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   If the `registry-packages-proxy` module was previously disabled, re-enable it:

   ```shell
   d8 platform module enable registry-packages-proxy
   ```

1. Delete the temporary DKP CE pod:

   ```shell
   d8 k delete pod ce-image
   ```

## Switching DKP from EE to SE

To perform the switch, you will need a valid license token.

{% alert level="info" %}
This instruction uses the public container registry address: `registry.deckhouse.ru`.

DKP SE does **not** support cloud providers such as `dynamix`, `openstack`, `VCD`, `VSphere`, and several modules.
{% endalert %}

The steps below describe how to switch a Deckhouse Enterprise Edition cluster to Standard Edition:

{% alert level="warning" %}
All commands must be executed on the master node of the existing cluster.
{% endalert %}

1. Prepare environment variables with your license token:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
   ```

1. Create a [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) resource to enable transitional authorization to `registry.deckhouse.ru`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-se-config.sh
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
       bb-sync-file /etc/containerd/conf.d/se-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry.deckhouse.ru".auth]
               auth = "$AUTH_STRING"
       EOF_TOML

   EOF
   ```

   Wait until the file `/etc/containerd/conf.d/se-registry.toml` appears on the nodes and Bashible synchronization completes. You can track the synchronization status by checking the `UPTODATE` value (it should match the total number of nodes in each group):

   ```shell
   d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Example output:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   You should also see the message `Configuration is in sync, nothing to do.` in the bashible systemd service logs:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-se-0 bashible.sh[53407]: Annotate node master-ee-to-se-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master ee-to-se-0 bashible.sh[53407]: Successful annotate node master-ee-to-se-0 with annotation node.deckhouse.io/   configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-se-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Launch a temporary DKP SE pod to retrieve the latest image digests and module list:

   ```shell
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   d8 k run se-image --image=registry.deckhouse.ru/deckhouse/se/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   To check the currently installed DKP version:

   ```shell
   d8 k get deckhousereleases | grep Deployed
   ```

1. Once the pod reaches the `Running` state, execute the following steps:

   * Retrieve the value of `SE_REGISTRY_PACKAGE_PROXY`:

     ```shell
     SE_REGISTRY_PACKAGE_PROXY=$(d8 k exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".registryPackagesProxy.registryPackagesProxy")
     ```

     Pull the DKP SE image manually:

     ```shell
     sudo /opt/deckhouse/bin/crictl pull registry.deckhouse.ru/deckhouse/se@$SE_REGISTRY_PACKAGE_PROXY
     ```

     Example output:

     ```console
     Image is up to date for sha256:7e9908d47580ed8a9de481f579299ccb7040d5c7fade4689cb1bff1be74a95de
     ```

   * Retrieve the list of available modules in SE:

     ```shell
     SE_MODULES=$(d8 k exec se-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
     ```

     Check the result:

     ```shell
     echo $SE_MODULES
     ```

     Example output:

     ```console
     common priority-class deckhouse external-module-manager ...
     ```

   * Retrieve the list of currently enabled embedded modules:

     ```shell
     USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
     ```

     Check the result:

     ```shell
     echo $USED_MODULES
     ```

     Example output:

     ```console
     admission-policy-engine cert-manager chrony ...
     ```

   * Determine which modules must be disabled:

     ```shell
     MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $SE_MODULES | tr ' ' '\n'))
     ```

1. Make sure the modules currently used in the cluster are supported by the SE edition.
   To check which modules are not supported and will be disabled, run:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   Review the list and make sure the functionality of these modules is not critical for your cluster before proceeding.

   Disable the unsupported modules:

   ```shell
   echo $MODULES_WILL_DISABLE | 
     tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   Wait for the DKP pod to reach the `Ready` state.

1. Update the Deckhouse registry access secret:

   ```shell
   d8 k -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry.deckhouse.ru\": { \"username\": \"license-token\", \"password\": \"$LICENSE_TOKEN\", \"auth\":    \"$AUTH_STRING\" }}}" \
     --from-literal="address"=registry.deckhouse.ru   --from-literal="path"=/deckhouse/se \
     --from-literal="scheme"=https   --type=kubernetes.io/dockerconfigjson \
     --dry-run=client \
     -o yaml | kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- kubectl replace -f -
   ```

1. Apply the new webhook-handler image:

   ```shell
   HANDLER=$(d8 k exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".deckhouse.webhookHandler")
   d8 k --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/webhook-handler handler=registry.deckhouse.ru/deckhouse/se@$HANDLER
   ``

1. Apply the DKP SE images:

   ```shell
   DECKHOUSE_KUBE_RBAC_PROXY=$(d8 k exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   DECKHOUSE_INIT_CONTAINER=$(d8 k exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   d8 k --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/deckhouse init-downloaded-modules=registry.deckhouse.ru/deckhouse/se@$DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry.deckhouse.ru/deckhouse/se@$DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry.deckhouse.ru/deckhouse/se:$DECKHOUSE_VERSION
   ```

   You can check the currently installed DKP version with:

   ```shell
   d8 k get deckhousereleases | grep Deployed
   ```

1. Wait for the Deckhouse pod to reach the `Ready`.  
   If an `ImagePullBackOff` error occurs during the update, wait for the pod to restart automatically.

   To check the status of the DKP pod:

   ```shell
   d8 k -n d8-system get po -l app=deckhouse
   ```

   To check the status of the Deckhouse queue:

   ```shell
   d8 platform queue list
   ```

1. Make sure there are no running pods using the DKP EE registry address:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.status.phase=="Running" or .status.phase=="Pending" or .status.phase=="PodInitializing") | select(.spec.containers[] | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

1. Clean up temporary files, the NodeGroupConfiguration resource, and variables:

   ```shell
   d8 k delete ngc containerd-se-config.sh
   d8 k delete pod se-image
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
           if [ -f /etc/containerd/conf.d/se-registry.toml ]; then
             rm -f /etc/containerd/conf.d/se-registry.toml
           fi
   EOF
   ```

   After bashible synchronization completes (indicated by `UPTODATE` in the NodeGroup status), delete the temporary NodeGroupConfiguration resource:

   ```shell
   d8 k delete ngc del-temp-config.sh
   ```

## Switching DKP from EE to CSE

{% alert level="warning" %}
This guide assumes the use of the public container registry address: `registry-cse.deckhouse.ru`.

DKP CSE does not support cloud clusters and certain modules. See the [edition comparison](../../../reference/revision-comparison.html) page for details on supported modules.

Migration to DKP CSE is only possible from DKP EE versions **1.58**, **1.64**, or **1.67**.

The current available DKP CSE versions are: **1.58.2** for the 1.58 release, **1.64.1** for the 1.64 release, and **1.67.0** for the 1.67 release. These versions must be used when setting the `DECKHOUSE_VERSION` variable in subsequent steps.

Migration is only supported between the same minor versions. For example, migrating from DKP EE 1.64 to DKP CSE 1.64 is allowed. Migrating from EE 1.58 to CSE 1.67 requires intermediate upgrades: first to EE 1.64, then to EE 1.67, and only then to CSE 1.67. Attempting to upgrade across multiple releases at once may render the cluster inoperable.

DKP CSE 1.58 and 1.64 support Kubernetes version 1.27. DKP CSE 1.67 supports Kubernetes versions 1.27 and 1.29.

A temporary disruption of cluster components may occur during the switch to DKP CSE.
{% endalert %}

To switch your Deckhouse Enterprise Edition cluster to Certified Security Edition, follow the steps below (all commands must be executed on a master node by a user with a configured `kubectl` context or with superuser pr

1.Configure the cluster to use the required Kubernetes version (see the note above regarding the available Kubernetes versions). To do this, run the following command:

   ```shell
   d8 platform edit cluster-configuration
   ```

1. Change the [`kubernetesVersion`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-kubernetesversion) parameter to the desired value, for example, `"1.27"` (in quotes) for Kubernetes 1.27.

1. Save the changes. The cluster nodes will begin updating sequentially.

1. Wait for the update to complete. You can monitor the update progress using the `d8 k get no` command. The update is considered complete when the `VERSION` column for each node shows the updated version.

1. Prepare the license token variables and create a [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) resource to configure temporary authorization for access to `registry-cse.deckhouse.ru`:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
   d8 k apply -f - <<EOF
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-cse-config.sh
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
       bb-sync-file /etc/containerd/conf.d/cse-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry]
             [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
           [plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry-cse.deckhouse.ru"]
             endpoint = ["https://registry-cse.deckhouse.ru"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry-cse.deckhouse.ru".auth]
               auth = "$AUTH_STRING"
       EOF_TOML
   EOF
   ```

   Wait until the synchronization is complete and the `/etc/containerd/conf.d/cse-registry.toml` file appears on the nodes.

   You can monitor the synchronization status using the `UPTODATE` value (the number of nodes in this status should match the total number of nodes (`NODES`) in the group):

   ```shell
   d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Example output:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   In the systemd log of the `bashible` service, the `Configuration is in sync, nothing to do` message should appear, indicating successful synchronization:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 bashible.sh[53407]: Successful annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Run the following commands to start a temporary DKP CSE pod to retrieve the current image digests and module list:

   ```shell
   DECKHOUSE_VERSION=v<DECKHOUSE_VERSION_CSE>
   # For example, DECKHOUSE_VERSION=v1.58.2
   d8 k run cse-image --image=registry-cse.deckhouse.ru/deckhouse/cse/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   Once the pod reaches the `Running` status, execute the following commands:

   ```shell
   CSE_SANDBOX_IMAGE=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | grep pause | grep -oE 'sha256:\w*')
   CSE_K8S_API_PROXY=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | grep kubernetesApiProxy | grep -oE 'sha256:\w*')
   CSE_MODULES=$(d8 k exec cse-image -- ls -l deckhouse/modules/ | awk {'print $9'} |grep -oP "\d.*-\w*" | cut -c5-)
   USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $CSE_MODULES | tr ' ' '\n'))
   CSE_DECKHOUSE_KUBE_RBAC_PROXY=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   ```

   Additional command required only when switching to DKP CSE version 1.64:

   ```shell
   CSE_DECKHOUSE_INIT_CONTAINER=$(d8 k exec cse-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   ```

1. Make sure that the modules currently used in the cluster are supported in DKP CSE.  
   For example, in Deckhouse CSE 1.58 and 1.64, the [`cert-manager`](/modules/cert-manager/) module is not available. Therefore, before disabling the `cert-manager` module, you must switch the HTTPS mode of certain components (such as [`user-authn`](/modules/user-authn/configuration.html#parameters-https-mode) or [`prometheus`](/modules/prometheus/configuration.html#parameters-https-mode)) to alternative modes, or change the [global HTTPS mode parameter](../../../reference/api/global.html#parameters-modules-https-mode) accordingly.

   To display the list of modules that are not supported in DKP CSE and will be disabled, run:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   Review the list and make sure that the listed modules are not actively used in your cluster and that you are ready to disable them.

   Disable the modules not supported in DKP CSE:

   ```shell
   echo $MODULES_WILL_DISABLE | 
     tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```

   The `earlyOOM` component is not supported in DKP CSE. Disable it using the [earlyOomEnabled](/modules/node-manager/configuration.html#parameters-earlyoomenabled)) setting.

   Wait for the DKP pod to reach the `Ready` status and for all tasks in the queue to complete:

   ```shell
   d8 platform queue list
   ```

   Verify that the disabled modules are now in the `Disabled` state:

   ```shell
   d8 k get modules
   ```

1. Create a [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: cse-set-sha-images.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 50
     content: |
        _on_containerd_config_changed() {
          bb-flag-set containerd-need-restart
        }
        bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

        bb-sync-file /etc/containerd/conf.d/cse-sandbox.toml - containerd-config-file-changed << "EOF_TOML"
        [plugins]
          [plugins."io.containerd.grpc.v1.cri"]
            sandbox_image = "registry-cse.deckhouse.ru/deckhouse/cse@$CSE_SANDBOX_IMAGE"
        EOF_TOML

        sed -i 's|image: .*|image: registry-cse.deckhouse.ru/deckhouse/cse@$CSE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
        sed -i 's|crictl pull .*|crictl pull registry-cse.deckhouse.ru/deckhouse/cse@$CSE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
   EOF
   ```

   Wait for `bashible` synchronization to complete on all nodes.

   You can track the synchronization status by checking the `UPTODATE` value (the number of nodes in this state should match the total number of nodes (`NODES`) in the group):

   ```shell
   d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   The following message should appear in the `bashible` systemd service logs on the nodes, indicating that the configuration is fully synchronized:

   ```shell
   journalctl -u bashible -n 5
   ```

   Example output:

   ```console
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 bashible.sh[53407]: Successful annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Update the secret for accessing the DKP CSE registry:

   ```shell
   d8 k -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry-cse.deckhouse.ru\": { \"username\": \"license-token\", \"password\": \"$LICENSE_TOKEN\", \"auth\": \"$AUTH_STRING\" }}}" \
     --from-literal="address"=registry-cse.deckhouse.ru \
     --from-literal="path"=/deckhouse/cse \
     --from-literal="scheme"=https \
     --type=kubernetes.io/dockerconfigjson \
     --dry-run='client' \
     -o yaml | kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- kubectl replace -f -
   ```

1. Update the DKP image to use the DKP CSE image:

   For DKP CSE version 1.58:

   ```shell
   d8 k -n d8-system set image deployment/deckhouse kube-rbac-proxy=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry-cse.deckhouse.ru/deckhouse/cse:$DECKHOUSE_VERSION
   ```

   For DKP CSE versions 1.64 and 1.67:

   ```shell
   d8 k -n d8-system set image deployment/deckhouse init-downloaded-modules=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry-cse.deckhouse.ru/deckhouse/cse@$CSE_DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry-cse.deckhouse.ru/deckhouse/cse:$DECKHOUSE_VERSION
   ```

1. Wait for the DKP pod to reach the `Ready` status and for all tasks in the queue to complete. If the `ImagePullBackOff` error occurs, wait for the pod to automatically restart.

   Check the DKP pod status:

   ```shell
   d8 k -n d8-system get po -l app=deckhouse
   ```

   Check the DKP task queue:

   ```shell
   d8 platform queue list
   ```

1. Verify that no pods are using the EE registry image:

   ```shell
   d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   If the output contains pods from the [`chrony`](/modules/chrony/) module, re-enable the module (it's disabled by default in DKP CSE):

   ```shell
   d8 platform module enable chrony
   ```

1. Clean up temporary files, the NodeGroupConfiguration resource, and temporary variables:

   ```shell
   rm /tmp/cse-deckhouse-registry.yaml
   d8 k delete ngc containerd-cse-config.sh cse-set-sha-images.sh
   kd8 k delete pod cse-image
   ```

   ```yaml
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
       if [ -f /etc/containerd/conf.d/cse-registry.toml ]; then
         rm -f /etc/containerd/conf.d/cse-registry.toml
       fi
       if [ -f /etc/containerd/conf.d/cse-sandbox.toml ]; then
         rm -f /etc/containerd/conf.d/cse-sandbox.toml
       fi
   EOF
   ```

   After synchronization (track status by `UPTODATE` value for NodeGroup), delete the cleanup configuration:

   ```shell
   d8 k delete ngc del-temp-config.sh
   ```

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

1. Ensure the Deckhouse queue is empty and error-free.

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
   echo $MODULES_WILL_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
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
