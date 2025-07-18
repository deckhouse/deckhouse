---
title: "Switching editions"
permalink: en/editions
---

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
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   kubectl run ce-image --image=registry.deckhouse.ru/deckhouse/ce/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   This will run the image of the latest installed DKP version in the cluster.

   To determine the currently installed version, use:

   ```shell
   kubectl get deckhousereleases | grep Deployed
   ```

1. Once the pod enters the `Running` state, execute the following commands:

   Retrieve the `CE_REGISTRY_PACKAGE_PROXY` value:

   ```shell
   CE_REGISTRY_PACKAGE_PROXY=$(kubectl exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".registryPackagesProxy.registryPackagesProxy")
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
   CE_MODULES=$(kubectl exec ce-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
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
   USED_MODULES=$(kubectl get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
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
     tr ' ' '\n' | awk {'print "kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module disable",$1'} | bash
   ```

   Example output:

   ```console
   Defaulted container "deckhouse" out of: deckhouse, kube-rbac-proxy, init-external-modules (init)
   Module node-local-dns disabled
   ```

1. Update the DKP registry access secret by running the following command:

   ```bash
   kubectl -n d8-system create secret generic deckhouse-registry \
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
  HANDLER=$(kubectl exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".deckhouse.webhookHandler")
  kubectl --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/webhook-handler handler=registry.deckhouse.ru/deckhouse/ce@$HANDLER
  ```

1. Apply the DKP CE image:

   ```shell
   DECKHOUSE_KUBE_RBAC_PROXY=$(kubectl exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   DECKHOUSE_INIT_CONTAINER=$(kubectl exec ce-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   kubectl --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/deckhouse init-downloaded-modules=registry.deckhouse.ru/deckhouse/ce@$DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry.deckhouse.ru/deckhouse/ce@$DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry.deckhouse.ru/deckhouse/ce:$DECKHOUSE_VERSION
   ```

1. Wait for the DKP pod to reach the `Ready` status and for [all tasks in the queue](https://deckhouse.io/products/kubernetes-platform/documentation/latest/deckhouse-faq.html#%D0%BA%D0%B0%D0%BA-%D0%BF%D1%80%D0%BE%D0%B2%D0%B5%D1%80%D0%B8%D1%82%D1%8C-%D0%BE%D1%87%D0%B5%D1%80%D0%B5%D0%B4%D1%8C-%D0%B7%D0%B0%D0%B4%D0%B0%D0%BD%D0%B8%D0%B9-%D0%B2-deckhouse) to complete.  
If you encounter the `ImagePullBackOff` error during this process, wait for the pod to restart automatically.

   Check the status of the DKP pod:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

   Check the DKP task queue:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller queue list
   ```

1. Check if any pods in the cluster are still using the DKP EE registry address:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
      | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

   Если ранее был отключён модуль `registry-packages-proxy`, включите его повторно:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module enable registry-packages-proxy
   ```

1. Delete the temporary DKP CE pod:

   ```shell
   kubectl delete pod ce-image
   ```

## Switching DKP from CE to EE

A valid license key is required. If needed, you can [request a temporary license](https://deckhouse.io/products/enterprise_edition.html).

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

   Create a NodeGroupConfiguration resource to enable transitional authorization to `registry.deckhouse.ru`:

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
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
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
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   kubectl run ee-image --image=registry.deckhouse.ru/deckhouse/ee/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   To verify which DKP version is currently deployed:

   ```shell
   kubectl get deckhousereleases | grep Deployed
   ```

1. Once the pod reaches the `Running` state, execute the following commands:

   Retrieve the value of `EE_REGISTRY_PACKAGE_PROXY`:

   ```shell
   EE_REGISTRY_PACKAGE_PROXY=$(kubectl exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".registryPackagesProxy.registryPackagesProxy")
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
   kubectl -n d8-system create secret generic deckhouse-registry \
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
   HANDLER=$(kubectl exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".deckhouse.webhookHandler")
   kubectl --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/webhook-handler handler=registry.deckhouse.ru/deckhouse/ee@$HANDLER
   ```

1. Apply the Deckhouse EE image:

   ```shell
   DECKHOUSE_KUBE_RBAC_PROXY=$(kubectl exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   DECKHOUSE_INIT_CONTAINER=$(kubectl exec ee-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   kubectl --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/deckhouse init-downloaded-modules=registry.deckhouse.ru/deckhouse/ee@$DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry.deckhouse.ru/deckhouse/ee@$DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry.deckhouse.ru/deckhouse/ee:$DECKHOUSE_VERSION
   ```

1. Wait for the Deckhouse pod to reach the `Ready` status and for [all tasks in the queue](https://deckhouse.io/products/kubernetes-platform/documentation/latest/deckhouse-faq.html#%D0%BA%D0%B0%D0%BA-%D0%BF%D1%80%D0%BE%D0%B2%D0%B5%D1%80%D0%B8%D1%82%D1%8C-%D0%BE%D1%87%D0%B5%D1%80%D0%B5%D0%B4%D1%8C-%D0%B7%D0%B0%D0%B4%D0%B0%D0%BD%D0%B8%D0%B9-%D0%B2-deckhouse) to complete.  
   If you encounter the `ImagePullBackOff` error during this process, wait for the pod to restart automatically.

   Check the status of the DKP pod:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

   Check the DKP task queue:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller queue list
   ```

1. Check if any pods are still using the CE registry address:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
      | select(.image | contains("deckhouse.ru/deckhouse/ce"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

1. Clean up temporary files, the NodeGroupConfiguration resource, and variables:

   ```shell
   kubectl delete ngc containerd-ee-config.sh
   kubectl delete pod ee-image
   kubectl apply -f - <<EOF
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
   kubectl delete ngc del-temp-config.sh
   ```

## Switching DKP from EE to SE

To perform the switch, you will need a valid license token. If needed, you can [request a temporary license](https://deckhouse.io/products/kubernetes-platform) by clicking the *Request Consultation* button.

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

1. Create a NodeGroupConfiguration resource to enable transitional authorization to `registry.deckhouse.ru`:

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
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
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
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   kubectl run se-image --image=registry.deckhouse.ru/deckhouse/se/install:$DECKHOUSE_VERSION --command sleep -- infinity
   ```

   To check the currently installed DKP version:

   ```shell
   kubectl get deckhousereleases | grep Deployed
   ```

1. Once the pod reaches the `Running` state, execute the following steps:

   * Retrieve the value of `SE_REGISTRY_PACKAGE_PROXY`:

     ```shell
     SE_REGISTRY_PACKAGE_PROXY=$(kubectl exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".registryPackagesProxy.registryPackagesProxy")
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
     SE_MODULES=$(kubectl exec se-image -- ls -l deckhouse/modules/ | grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
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
     USED_MODULES=$(kubectl get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
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
     tr ' ' '\n' | awk {'print "kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module disable",$1'} | bash
   ```

   Wait for the DKP pod to reach the `Ready` state.

1. Update the Deckhouse registry access secret:

   ```shell
   kubectl -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry.deckhouse.ru\": { \"username\": \"license-token\", \"password\": \"$LICENSE_TOKEN\", \"auth\":    \"$AUTH_STRING\" }}}" \
     --from-literal="address"=registry.deckhouse.ru   --from-literal="path"=/deckhouse/se \
     --from-literal="scheme"=https   --type=kubernetes.io/dockerconfigjson \
     --dry-run=client \
     -o yaml | kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- kubectl replace -f -
   ```

1. Apply the new webhook-handler image:

   ```shell
   HANDLER=$(kubectl exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".deckhouse.webhookHandler")
   kubectl --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/webhook-handler handler=registry.deckhouse.ru/deckhouse/se@$HANDLER
   ``

1. Apply the DKP SE images:

   ```shell
   DECKHOUSE_KUBE_RBAC_PROXY=$(kubectl exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
   DECKHOUSE_INIT_CONTAINER=$(kubectl exec se-image -- cat deckhouse/candi/images_digests.json | jq -r ".common.init")
   DECKHOUSE_VERSION=$(kubectl -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $2}')
   kubectl --as=system:serviceaccount:d8-system:deckhouse -n d8-system set image deployment/deckhouse init-downloaded-modules=registry.deckhouse.ru/deckhouse/se@$DECKHOUSE_INIT_CONTAINER kube-rbac-proxy=registry.deckhouse.ru/deckhouse/se@$DECKHOUSE_KUBE_RBAC_PROXY deckhouse=registry.deckhouse.ru/deckhouse/se:$DECKHOUSE_VERSION
   ```

   You can check the currently installed DKP version with:

   ```shell
   kubectl get deckhousereleases | grep Deployed
   ```

1. Wait for the Deckhouse pod to reach the `Ready`.  
   If an `ImagePullBackOff` error occurs during the update, wait for the pod to restart automatically.

   To check the status of the DKP pod:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

   To check the status of the Deckhouse queue:

   ```shell
   kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller queue list
   ```

1. Make sure there are no running pods using the DKP EE registry address:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.status.phase=="Running" or .status.phase=="Pending" or .status.phase=="PodInitializing") | select(.spec.containers[] | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
   ```

1. Clean up temporary files, the NodeGroupConfiguration resource, and variables:

   ```shell
   kubectl delete ngc containerd-se-config.sh
   kubectl delete pod se-image
   kubectl apply -f - <<EOF
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
   kubectl delete ngc del-temp-config.sh
   ```

## Switching DKP from EE to CSE

{% alert level="warning" %}
This guide assumes the use of the public container registry address: `registry-cse.deckhouse.ru`.

DKP CSE does not support cloud clusters and certain modules. See the [edition comparison](revision-comparison.html) page for details on supported modules.

Migration to DKP CSE is only possible from DKP EE versions **1.58**, **1.64**, or **1.67**.

The current available DKP CSE versions are: **1.58.2** for the 1.58 release, **1.64.1** for the 1.64 release, and **1.67.0** for the 1.67 release. These versions must be used when setting the `DECKHOUSE_VERSION` variable in subsequent steps.

Migration is only supported between the same minor versions. For example, migrating from DKP EE 1.64 to DKP CSE 1.64 is allowed. Migrating from EE 1.58 to CSE 1.67 requires intermediate upgrades: first to EE 1.64, then to EE 1.67, and only then to CSE 1.67. Attempting to upgrade across multiple releases at once may render the cluster inoperable.

DKP CSE 1.58 and 1.64 support Kubernetes version 1.27. DKP CSE 1.67 supports Kubernetes versions 1.27 and 1.29.

A temporary disruption of cluster components may occur during the switch to DKP CSE.
{% endalert %}

To switch your Deckhouse Enterprise Edition cluster to Certified Security Edition, follow the steps below (all commands must be executed on a master node by a user with a configured `kubectl` context or with superuser pr

1. Настройте кластер на использование необходимой версии Kubernetes (см. примечание выше про доступные версии Kubernetes). Для этого выполните команду:

   ```shell
   kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit cluster-configuration
   ```

1. Измените параметр `kubernetesVersion` на необходимое значение, например, `"1.27"` (в кавычках) для Kubernetes 1.27.

1. Сохраните изменения. Узлы кластера начнут последовательно обновляться.

1. Дождитесь окончания обновления. Отслеживать ход обновления можно с помощью команды `kubectl get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.

1. Подготовьте переменные с токеном лицензии и создайте NodeGroupConfiguration для переходной авторизации в `registry-cse.deckhouse.ru`:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
   kubectl apply -f - <<EOF
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

   Дождитесь завершения синхронизации и появления файла `/etc/containerd/conf.d/cse-registry.toml` на узлах.

   Статус синхронизации можно отследить по значению `UPTODATE` (отображаемое число узлов в этом статусе должно совпадать с общим числом узлов (`NODES`) в группе):

   ```shell
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Пример вывода:

   ```console
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   В журнале systemd-сервиса bashible должно появиться сообщение `Configuration is in sync, nothing to do` в результате выполнения следующей команды:

   ```shell
   journalctl -u bashible -n 5
   ```

   Пример вывода:

   ```console
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 bashible.sh[53407]: Successful annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 systemd[1]: bashible.service: Deactivated successfully.
   ```
