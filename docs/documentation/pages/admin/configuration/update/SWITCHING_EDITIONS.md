---
title: "Switching between DKP editions"
permalink: en/admin/configuration/update/switching-editions.html
description: "Switching between Deckhouse Kubernetes Platform editions. Migration from Community Edition to Enterprise Edition and license management."
---

This guide describes the steps required to switch the Deckhouse Kubernetes Platform edition in a running cluster. Follow the sections in order.

The switching process differs depending on how you work with the image registry. Choose the method that applies to your cluster and follow the instructions.

A valid license key is required when switching to DKP BE/SE/SE+/EE. It is not required when switching to DKP CE.

{% alert level="warning" %}
This guide assumes the use of a public container image registry (`registry.deckhouse.io`). If you use a different registry address, adjust the commands or refer to the [guide for switching Deckhouse to a third-party container image registry](./registry/third-party.html).

All commands are executed on the master node of the existing cluster as the `root` user.
{% endalert %}

{% capture wait_queue %}
```bash
d8 system queue list
```

{% offtopic title="Example output (queues are empty)..." %}
```console
Summary:
- 'main' queue: empty.
- 88 other queues (0 active, 88 empty): 0 tasks.
- no tasks to handle.
```
{% endofftopic %}
{% endcapture %}

## Pre-switch preparation

### Queue check

Make sure the DKP queues are empty and there are no running tasks that could interfere with the switch:

{{ wait_queue }}

### Determining the current edition and version

To ensure the correctness of further steps, determine the current DKP edition used in the cluster. This helps avoid errors during the switch and confirms that the required modules and features are supported in the new edition.

You can find the edition and version currently used in the cluster on the main page of the DKP web interface, or by using CLI commands:

- edition:

  ```bash
  d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values -o yaml | yq '.deckhouseEdition'
  ```
- version:

  ```bash
  d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}'
  ```

### Checking whether switching to the desired edition is possible

{% capture check_new_modules %}
```shell
(set -e
trap 'echo "Execution error"' ERR

<!REMOVE_FOR_CE>
d8 k create secret docker-registry $NEW_EDITION-image-pull-secret --docker-server=registry.deckhouse.io --docker-username=license-token --docker-password=${LICENSE_TOKEN}
<!/REMOVE_FOR_CE>

DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
d8 k run $NEW_EDITION-image --image=registry.deckhouse.io/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION \
<!REMOVE_FOR_CE>    --overrides="{\"spec\": {\"imagePullSecrets\":[{\"name\": \"$NEW_EDITION-image-pull-secret\"}]}}" \<!/REMOVE_FOR_CE>
    --command sleep -- infinity

d8 k wait --for=condition=ready pod/$NEW_EDITION-image --timeout=300s

NEW_MODULES=$(d8 k exec $NEW_EDITION-image -- ls -l deckhouse/modules/ |   grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
MODULES_TO_DISABLE=$(echo $USED_MODULES | tr ' ' '\n' | grep -Fxv -f <(echo $NEW_MODULES | tr ' ' '\n'))

d8 k delete pod/$NEW_EDITION-image --wait=false
d8 k delete secret/$NEW_EDITION-image-pull-secret

echo
echo "Modules not supported in the desired edition (edition code - $NEW_EDITION, version - $DECKHOUSE_VERSION):"
echo $MODULES_TO_DISABLE)
```
{% endcapture %}

{% capture disable_modules %}
1. Disable the modules from the list if acceptable (the module functionality is not used, or you are ready to give it up). Otherwise, **abort the switching process.**

   You can disable the modules from the list in the DKP web interface under System → System Management → Deckhouse → Modules, or by running the following command:

   ```shell
   echo $MODULES_TO_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
   ```
      
1. Make sure all tasks in the DKP queue are complete before continuing the switching process:
      
   {{ wait_queue | regex_replace: "^", "   " }}
{% endcapture %}

Different DKP editions support different sets of modules, Kubernetes versions, and features. It is important to understand what functional changes will occur during the switch and which capabilities will become unavailable. This will help you prepare for the switching process.

A comparison of DKP editions by module set can be found in the documentation on the [Edition comparison](../../reference/revision-comparison.html) page.

What to consider before switching:

{% tabs step1 %}
{% tab "To DKP CE" %}
1. Determine the list of modules used in the cluster that are not supported in DKP CE. To do this, follow these steps:

   1. Get the list of modules not supported in DKP CE: 

      {{ check_new_modules | regex_replace: "(?m)<!REMOVE_FOR_CE>.+?<!/REMOVE_FOR_CE>\n?", "" | regex_replace: "\$NEW_EDITION", "ce" | regex_replace: "^", "      " }}

{{ disable_modules }}
{% endtab %}
{% tab "To DKP BE/SE/SE+/EE" %}
1. Determine the list of modules used in the cluster that are not supported in the desired DKP edition. To do this, follow these steps:

   1. Set the environment variable with the code of the desired edition:

      {% tabs env-edition %}
      {% tab "DKP BE" %}
      ```shell
      NEW_EDITION=be
      ```
      {% endtab %}
      {% tab "DKP SE" %}
      ```shell
      NEW_EDITION=se
      ```
      {% endtab %}
      {% tab "DKP SE+" %}
      ```shell
      NEW_EDITION=se-plus
      ```
      {% endtab %}
      {% tab "DKP EE" %}
      ```shell
      NEW_EDITION=ee
      ```
      {% endtab %}
      {% endtabs %}

   1. Set the environment variable with the license key for the edition you plan to switch to:

      ```shell
      LICENSE_TOKEN=<LICENSE_KEY>
      ```

   1. Get the list of modules not supported in the desired DKP edition: 

      {{ check_new_modules | regex_replace: "<!/?REMOVE_FOR_CE>", "" | regex_replace: "^", "      " }}

{{ disable_modules }}
{% endtab %}
{% endtabs %}

## Switching the edition

There are two ways to work with the DKP container image registry:
- Using the [registry](/modules/registry/) module — **(recommended)**, the configuration for working with the DKP image registry is set in the [registry](/modules/deckhouse/configuration.html#parameters-registry) section of the `deckhouse` module parameters (ModuleConfig `deckhouse`). This method provides a smoother transition process and automatic verification that the required images are present.

- Without the `registry` module — the configuration for working with the DKP image registry is set during cluster installation [in InitConfiguration](../reference/api/cr.html#initconfiguration-deckhouse-imagesrepo), and the [registry.mode](/modules/deckhouse/configuration.html#parameters-registry-mode) parameter of the `deckhouse` module (ModuleConfig `deckhouse`) is set to `Unmanaged`.

  This method is the only one available for managed Kubernetes clusters where the control plane is managed by a cloud provider rather than DKP (e.g. Amazon EKS, Azure AKS, Google GKE, etc.).

Use the method that matches your cluster configuration.

Before proceeding, complete the preparatory steps described in the [Pre-switch preparation](#pre-switch-preparation) section.

{% capture bashible_sync_wait %}
Wait for the bashible service to synchronize (the `UPTODATE` column value for a NodeGroup must match `NODES`):

```shell
d8 k get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate
```

The bashible log should contain `Configuration is in sync, nothing to do`:

```shell
journalctl -u bashible -n 5
```
{% endcapture %}

{% capture check_old_pods_unmanaged %}
```shell
d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.io/deckhouse/<PREVIOUS_EDITION_CODE>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
```
{% endcapture %}

{% capture check_old_pods_direct %}
{% alert level="info" %}
The check does not account for external modules.
{% endalert %}

```shell
IMAGES_DIGESTS=$(d8 k -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- cat /deckhouse/modules/images_digests.json | jq -r '.[][]' | sort -u)

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
{% endcapture %}

### Switching using the registry module

{% alert level="warning" %}
Not applicable for managed Kubernetes (EKS, AKS, GKE).
{% endalert %}

{% capture change-registry-mc-deckhouse-direct %}
```yaml
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
<!REMOVE_FOR_CE>
        license: <LICENSE_KEY>
<!/REMOVE_FOR_CE>
        checkMode: Relax
        imagesRepo: <REGISTRY_HOST>/deckhouse/<EDITION_CODE>
        scheme: HTTPS
```
{% endcapture %}

{% capture change-registry-mc-deckhouse-unmanaged %}
```yaml
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
<!REMOVE_FOR_CE>
        license: <LICENSE_KEY>
<!/REMOVE_FOR_CE>
        checkMode: Relax
        imagesRepo: <REGISTRY_HOST>/deckhouse/<EDITION_CODE>
        scheme: HTTPS
```
{% endcapture %}

{% capture registry_status_cmd %}
```shell
d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml) | {"conditions": [.conditions[] | select(.type == "Ready" or .type == "RegistryContainsRequiredImages")]}'
```
{% endcapture %}

{% capture registry_status_example %}
```yaml
conditions:
  - lastTransitionTime: "2026-05-05T13:53:23Z"
    message: |-
      Mode: Default
      registry.deckhouse.io: all 182 items are checked
    reason: Ready
    status: "True"
    type: RegistryContainsRequiredImages
  - lastTransitionTime: "2026-05-05T13:54:49Z"
    message: ""
    reason: ""
    status: "True"
    type: Ready
```
{% endcapture %}

1. In ModuleConfig `deckhouse`, set `imagesRepo` to the target edition and `checkMode: Relax`.

   Run the command to edit ModuleConfig `deckhouse`:

   ```shell
   d8 k edit moduleconfig deckhouse
   ```

   Choose the example for your edition and mode (`Direct` / `Unmanaged`):

   {% tabs switch-registry-edition %}
   {% tab "DKP CE" %}
   {% tabs switch-registry-ce-mode %}
   {% tab "Direct" %}{{ change-registry-mc-deckhouse-direct | regex_replace: "<EDITION_CODE>", "ce" | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io" | regex_replace: "(?m)<!REMOVE_FOR_CE>.+?<!/REMOVE_FOR_CE>\n?", "" }}{% endtab %}
   {% tab "Unmanaged" %}{{ change-registry-mc-deckhouse-unmanaged | regex_replace: "<EDITION_CODE>", "ce" | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io" | regex_replace: "(?m)<!REMOVE_FOR_CE>.+?<!/REMOVE_FOR_CE>\n?", "" }}{% endtab %}
   {% endtabs %}
   {% endtab %}
   {% tab "DKP BE" %}
   {% tabs switch-registry-be-mode %}
   {% tab "Direct" %}{{ change-registry-mc-deckhouse-direct | regex_replace: "<EDITION_CODE>", "be" | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io" | regex_replace: "<!/?REMOVE_FOR_CE>", "" }}{% endtab %}
   {% tab "Unmanaged" %}{{ change-registry-mc-deckhouse-unmanaged | regex_replace: "<EDITION_CODE>", "be" | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io" | regex_replace: "<!/?REMOVE_FOR_CE>", "" }}{% endtab %}
   {% endtabs %}
   {% endtab %}
   {% tab "DKP SE" %}
   {% tabs switch-registry-se-mode %}
   {% tab "Direct" %}{{ change-registry-mc-deckhouse-direct | regex_replace: "<EDITION_CODE>", "se" | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io" | regex_replace: "<!/?REMOVE_FOR_CE>", "" }}{% endtab %}
   {% tab "Unmanaged" %}{{ change-registry-mc-deckhouse-unmanaged | regex_replace: "<EDITION_CODE>", "se" | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io" | regex_replace: "<!/?REMOVE_FOR_CE>", "" }}{% endtab %}
   {% endtabs %}
   {% endtab %}
   {% tab "DKP SE+" %}
   {% tabs switch-registry-seplus-mode %}
   {% tab "Direct" %}{{ change-registry-mc-deckhouse-direct | regex_replace: "<EDITION_CODE>", "se-plus" | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io" | regex_replace: "<!/?REMOVE_FOR_CE>", "" }}{% endtab %}
   {% tab "Unmanaged" %}{{ change-registry-mc-deckhouse-unmanaged | regex_replace: "<EDITION_CODE>", "se-plus" | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io" | regex_replace: "<!/?REMOVE_FOR_CE>", "" }}{% endtab %}
   {% endtabs %}
   {% endtab %}
   {% tab "DKP EE" %}
   {% tabs switch-registry-ee-mode %}
   {% tab "Direct" %}{{ change-registry-mc-deckhouse-direct | regex_replace: "<EDITION_CODE>", "ee" | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io" | regex_replace: "<!/?REMOVE_FOR_CE>", "" }}{% endtab %}
   {% tab "Unmanaged" %}{{ change-registry-mc-deckhouse-unmanaged | regex_replace: "<EDITION_CODE>", "ee" | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io" | regex_replace: "<!/?REMOVE_FOR_CE>", "" }}{% endtab %}
   {% endtabs %}
   {% endtab %}
   {% endtabs %}

1. Wait for the switch to complete.

   Check the status:

   {{ registry_status_cmd | regex_replace: "^", "   " }}

   Example of a successful output:

   {{ registry_status_example | regex_replace: "^", "   " }}

1. Set `checkMode` back to `Default` (choose the command for your mode):

   {% tabs switch-registry-relax %}
   {% tab "Direct" %}
   ```shell
   d8 k patch moduleconfig deckhouse --type=json -p='[{"op": "replace", "path": "/spec/settings/registry/direct/checkMode", "value": "Default"}]'
   ```
   {% endtab %}
   {% tab "Unmanaged" %}
   ```shell
   d8 k patch moduleconfig deckhouse --type=json -p='[{"op": "replace", "path": "/spec/settings/registry/unmanaged/checkMode", "value": "Default"}]'
   ```
   {% endtab %}
   {% endtabs %}

1. Check the switch status again.

   Check the status:

   {{ registry_status_cmd | regex_replace: "^", "   " }}

   Example of a successful output:

   {{ registry_status_example | regex_replace: "^", "   " }}

1. Check for pods with image pull errors:

   ```shell
   d8 k get pods -A | awk 'NR==1 || /^d8-/' | grep -E 'ImagePullBackOff|ErrImagePull'
   ```

   For each problematic module, run the following commands **on all master nodes**, specifying the module name:

   ```shell
   rm -rf /var/lib/deckhouse/downloaded/<MODULE_NAME>/
   d8 k rollout restart deploy -n d8-system deckhouse
   ```

1. Check for pods using the old registry:

   {% tabs switch-registry-check-old %}
   {% tab "Direct" %}{{ check_old_pods_direct }}{% endtab %}
   {% tab "Unmanaged" %}{{ check_old_pods_unmanaged }}{% endtab %}
   {% endtabs %}

### Switching without the registry module

{% capture alert_additional_registry %}
{% alert level="info" %}
If you need to add configuration for an additional registry in containerd, refer to the [How to add configuration for an additional registry in containerd](/modules/node-manager/faq.html#how-to-add-configuration-for-an-additional-registry-in-containerd) section.
{% endalert %}
{% endcapture %}

{% capture ngc_auth_registry %}
{{ alert_additional_registry }}

```shell
AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
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
{% endcapture %}

{% capture change_registry_helper_ce %}
```shell
DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.io/deckhouse/ce
```
{% endcapture %}

{% capture change_registry_helper_commercial %}
```shell
DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
DOCKER_CONFIG_JSON=$(echo -n "{\"auths\": {\"registry.deckhouse.io\": {\"username\": \"license-token\", \"password\": \"${LICENSE_TOKEN}\", \"auth\": \"${AUTH_STRING}\"}}}" | base64 -w 0)
d8 k --as system:sudouser -n d8-cloud-instance-manager patch secret deckhouse-registry --type merge --patch="{\"data\":{\".dockerconfigjson\":\"$DOCKER_CONFIG_JSON\"}}"
d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user=license-token --password=$LICENSE_TOKEN --new-deckhouse-tag=$DECKHOUSE_VERSION registry.deckhouse.io/deckhouse/$NEW_EDITION
```
{% endcapture %}

{% capture ngc_cleanup_registry %}
```shell
d8 k delete ngc containerd-$NEW_EDITION-config.sh
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
d8 k delete ngc del-temp-config.sh
```
{% endcapture %}

Choose the target edition:

{% tabs switch-without-registry %}
{% tab "DKP CE" %}
1. Switch the registry:

   {{ change_registry_helper_ce | regex_replace: "^", "   " }}

1. Wait for DKP to be ready:

   {{ wait_queue | regex_replace: "^", "   " }}

1. Check that images from the previous edition are no longer in use (specify the code of the **previous** edition):

   {{ check_old_pods_unmanaged | regex_replace: "^", "   " }}
{% endtab %}
{% tab "DKP BE" %}
1. Run the command to set the authentication credentials for the image registry:

   {{ ngc_auth_registry | regex_replace: "\$NEW_EDITION", "be" | regex_replace: "^", "   " }}

   {{ bashible_sync_wait | regex_replace: "^", "   " }}

1. Switch the registry:

   {{ change_registry_helper_commercial | regex_replace: "\$NEW_EDITION", "be" | regex_replace: "^", "   " }}

1. Wait for DKP to be ready:

   {{ wait_queue | regex_replace: "^", "   " }}

1. Check that images from the previous edition are no longer in use (specify the code of the **previous** edition):

   {{ check_old_pods_unmanaged | regex_replace: "^", "   " }}

1. Perform cleanup:

   {{ ngc_cleanup_registry | regex_replace: "\$NEW_EDITION", "be" | regex_replace: "^", "   " }}
{% endtab %}
{% tab "DKP SE" %}
1. Run the command to set the authentication credentials for the image registry:

   {{ ngc_auth_registry | regex_replace: "\$NEW_EDITION", "se" | regex_replace: "^", "   " }}

   {{ bashible_sync_wait | regex_replace: "^", "   " }}

1. Switch the registry:

   {{ change_registry_helper_commercial | regex_replace: "\$NEW_EDITION", "se" | regex_replace: "^", "   " }}

1. Wait for DKP to be ready:

   {{ wait_queue | regex_replace: "^", "   " }}

1. Check that images from the previous edition are no longer in use (specify the code of the **previous** edition):

   {{ check_old_pods_unmanaged | regex_replace: "^", "   " }}

1. Perform cleanup:

   {{ ngc_cleanup_registry | regex_replace: "\$NEW_EDITION", "se" | regex_replace: "^", "   " }}
{% endtab %}
{% tab "DKP SE+" %}
1. Run the command to set the authentication credentials for the image registry:

   {{ ngc_auth_registry | regex_replace: "\$NEW_EDITION", "se-plus" | regex_replace: "^", "   " }}

   {{ bashible_sync_wait | regex_replace: "^", "   " }}

1. Switch the registry:

   {{ change_registry_helper_commercial | regex_replace: "\$NEW_EDITION", "se-plus" | regex_replace: "^", "   " }}

1. Wait for DKP to be ready:

   {{ wait_queue | regex_replace: "^", "   " }}

1. Check that images from the previous edition are no longer in use (specify the code of the **previous** edition):

   {{ check_old_pods_unmanaged | regex_replace: "^", "   " }}

1. Perform cleanup:

   {{ ngc_cleanup_registry | regex_replace: "\$NEW_EDITION", "se-plus" | regex_replace: "^", "   " }}
{% endtab %}
{% tab "DKP EE" %}
1. Run the command to set the authentication credentials for the image registry:

   {{ ngc_auth_registry | regex_replace: "\$NEW_EDITION", "ee" | regex_replace: "^", "   " }}

   {{ bashible_sync_wait | regex_replace: "^", "   " }}

1. Switch the registry:

   {{ change_registry_helper_commercial | regex_replace: "\$NEW_EDITION", "ee" | regex_replace: "^", "   " }}

1. Wait for DKP to be ready:

   {{ wait_queue | regex_replace: "^", "   " }}

1. Check that images from the previous edition are no longer in use (specify the code of the **previous** edition):

   {{ check_old_pods_unmanaged | regex_replace: "^", "   " }}

1. Perform cleanup:

   {{ ngc_cleanup_registry | regex_replace: "\$NEW_EDITION", "ee" | regex_replace: "^", "   " }}
{% endtab %}
{% endtabs %}
