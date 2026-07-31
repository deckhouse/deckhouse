---
title: "Switching between DKP editions"
permalink: en/admin/configuration/update/switching-editions.html
description: "Switching between Deckhouse Kubernetes Platform editions. Migration from Community Edition to Enterprise Edition and license management."
---

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

{% capture take_care_of_the_internal_modules %}
1. Determine the list of internal modules used in the cluster that are not supported in DKP new edition. To do this, follow these steps:

   <!REMOVE_FOR_CE>

   1. Set the environment variable with the license key for the edition you plan to switch to:

      ```shell
      LICENSE_TOKEN=<LICENSE_KEY>
      ```

   <!/REMOVE_FOR_CE>

   1. Get the list of modules not supported in DKP $NEW_EDITION:

      ```shell
      (set -e
      trap 'echo "Execution error"' ERR

      echo
      echo "Running the deckhouse pod of the new edition with the command 'sleep -- infinity'"

      DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')
      <!REMOVE_FOR_CE>
      d8 k create secret docker-registry $NEW_EDITION-image-pull-secret --docker-server=registry.deckhouse.io --docker-username=license-token --docker-password=${LICENSE_TOKEN}
      <!/REMOVE_FOR_CE>
      d8 k run $NEW_EDITION-image --image=registry.deckhouse.io/deckhouse/$NEW_EDITION/install:$DECKHOUSE_VERSION \
      <!REMOVE_FOR_CE>
      --overrides="{\"spec\": {\"imagePullSecrets\":[{\"name\": \"$NEW_EDITION-image-pull-secret\"}]}}" \
      <!/REMOVE_FOR_CE>
      --command sleep -- infinity
      d8 k wait --for=condition=ready pod/$NEW_EDITION-image --timeout=300s

      echo
      echo "Getting information about internal modules from the current and new edition"
      echo "Comparing internal modules of the current and new edition"
      NEW_MODULES=$(d8 k exec $NEW_EDITION-image -- ls -l deckhouse/modules/ |   grep -oE "\d.*-\w*" | awk {'print $9'} | cut -c5-)
      USED_MODULES=$(d8 k get modules -o custom-columns=NAME:.metadata.name,SOURCE:.properties.source,STATE:.properties.state,ENABLED:.status.phase | grep Embedded | grep -E 'Enabled|Ready' | awk {'print $1'})
      MODULES_TO_DISABLE=$(echo "$USED_MODULES" | tr ' ' '\n' | grep -Fxv -f <(echo "$NEW_MODULES" | tr ' ' '\n') 2>&1) || { status=$?; [ $status -eq 1 ] && MODULES_TO_DISABLE="" || exit $status; }

      echo
      echo "Modules not supported in the desired edition (edition code - $NEW_EDITION, version - $DECKHOUSE_VERSION):"
      echo MODULES_TO_DISABLE=\"$MODULES_TO_DISABLE\"
      )

      echo
      echo "Deleting the deckhouse pod of the new edition"
      d8 k delete pod/$NEW_EDITION-image secret/$NEW_EDITION-image-pull-secret --wait=true --ignore-not-found=true
      ```

   1. Disable the modules from the list if acceptable (the module functionality is not used, or you are ready to give it up). Otherwise, **abort the switching process.**

      You can disable the modules from the list in the DKP web interface under System → System Management → Deckhouse → Modules, or by running the following command:

      ```shell
      echo $MODULES_TO_DISABLE | tr ' ' '\n' | awk {'print "d8 platform module disable",$1'} | bash
      ```

{% endcapture %}

{% capture take_care_of_the_external_modules %}
1. Determine the list of external modules launched via `moduleSource/deckhouse` that are not supported in DKP new edition. To do this, follow these steps:

   <!REMOVE_FOR_CE>

   1. Set the environment variable with the license key for the edition you plan to switch to:

      ```shell
      LICENSE_TOKEN=<LICENSE_KEY>
      ```

   <!/REMOVE_FOR_CE>

   1. Get the list of external modules launched via `moduleSource/deckhouse` with the result of checking their availability in DKP new edition:

      ```shell
      d8-crane() {
         if command -v d8 &>/dev/null && d8 cr &>/dev/null 2>&1; then
            d8 cr "$@"
         else
            crane "$@"
         fi
      }

      <!REMOVE_FOR_CE>
      d8-crane-login() {
         if command -v d8 &>/dev/null && d8 cr &>/dev/null 2>&1; then
            d8 cr login "$1" -u "license-token" -p "$2"
         else
            crane auth login "$1" -u "license-token" -p "$2"
         fi
      }
      <!/REMOVE_FOR_CE>

      check_external_modules() {
         local repo="$1"

         local modules=$(d8 k get modulerelease -o json | jq -r '.items[] | select(.metadata.ownerReferences[] | select(.kind=="ModuleSource" and .name=="deckhouse")) | "\(.spec.moduleName) \(.spec.version)"')
         [[ -z "$modules" ]] && return 0
         local registry_modules=$(d8-crane ls "$repo" 2>/dev/null)

         echo "Module | In registry | Version in registry"
         echo "-------|-------------|--------------------"

         echo "$modules" | while read module version; do
            grep -qx "$module" <<< "$registry_modules" && list="✅" || list="❌"
            d8-crane manifest "$repo/${module}/release:v${version}" &>/dev/null && release="✅" || release="❌"
            echo "$module ($version) | $list | $release"
         done
      }

      <!REMOVE_FOR_CE>
      d8-crane-login "registry.deckhouse.io" "$LICENSE_TOKEN"
      <!/REMOVE_FOR_CE>
      check_external_modules "registry.deckhouse.io/deckhouse/$NEW_EDITION/modules"
      ```

   1. Disable the modules that were not found in the new edition if acceptable. Otherwise, **abort the switching process.**
{% endcapture %}

{% capture take_care_deckhuse_imagepullbackoff %}
1. Make sure that deckhouse has started after the registry change:

   1. To do this, run the command:

      ```bash
      d8 k -n d8-system get pods -l "app=deckhouse"
      ```

   1. If Deckhouse is in ImagePullBackoff with an error downloading sidecar containers, specify the actual images:

      Get the actual digests of the sidecar images:

      ```bash
      DKP_REPO=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F':' '{print $1}')
      DKP_TAG=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F':' '{print $2}')

      d8 k run dkp-image --image=$DKP_REPO/install:$DKP_TAG --command sleep -- infinity
      d8 k wait --for=condition=ready pod/dkp-image --timeout=300s

      DECKHOUSE_KUBE_RBAC_PROXY=$(d8 k exec dkp-image -- cat /deckhouse/candi/images_digests.json | jq -r ".common.kubeRbacProxy")
      DECKHOUSE_INIT=$(d8 k exec dkp-image -- cat /deckhouse/candi/images_digests.json | jq -r ".deckhouse.init")

      d8 k delete pod dkp-image
      ```

      Apply images:

      ```bash
      d8 k -n d8-system set image deployment/deckhouse \
      kube-rbac-proxy=$DKP_REPO@$DECKHOUSE_KUBE_RBAC_PROXY \
      init-downloaded-modules=$DKP_REPO@$DECKHOUSE_INIT
      ```

   1. Make sure there is no image download error:

      ```bash
      d8 k -n d8-system get pods -l "app=deckhouse"
      ```

{% endcapture %}

{% capture take_care_of_the_queue %}
1. Make sure all tasks in the DKP queue are complete before continuing the switching process:

   {{ wait_queue | regex_replace: "^", "   " }}
{% endcapture %}

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

{% capture check_old_pods %}

```shell
d8 k get pods -A -o json | jq -r '.items[] | select(.spec.containers[] | select(.image | contains("deckhouse.io/deckhouse/<PREVIOUS_EDITION_CODE>"))) | .metadata.namespace + "\t" + .metadata.name' | sort | uniq
```

{% endcapture %}

{% capture change_registry_mc_deckhouse_unmanaged %}

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
        checkMode: <CHECK_MODE>
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

{% capture registry_status_example_relax %}

```yaml
conditions:
  - lastTransitionTime: "2026-05-05T13:53:23Z"
    message: |-
      Mode: Default
      registry.deckhouse.io: all 1 items are checked
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

{% capture alert_additional_registry %}
{% alert level="info" %}
If you need to add configuration for an additional registry in containerd, refer to the [How to add configuration for an additional registry in containerd](/modules/node-manager/faq.html#how-to-add-configuration-for-an-additional-registry-in-containerd) section.
{% endalert %}
{% endcapture %}

{% capture change_registry_helper_ce %}

```shell
DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')

d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry \
--new-deckhouse-tag=$DECKHOUSE_VERSION \
registry.deckhouse.io/deckhouse/ce
```

{% endcapture %}

{% capture change_registry_helper %}

```shell
DECKHOUSE_VERSION=$(d8 k -n d8-system get deploy deckhouse -ojson | jq -r '.spec.template.spec.containers[] | select(.name == "deckhouse") | .image' | awk -F: '{print $NF}')

AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
DOCKER_CONFIG_JSON=$(echo -n "{\"auths\": {\"registry.deckhouse.io\": {\"username\": \"license-token\", \"password\": \"${LICENSE_TOKEN}\", \"auth\": \"${AUTH_STRING}\"}}}" | base64 -w 0)

d8 k --as system:sudouser -n d8-cloud-instance-manager patch secret deckhouse-registry \
--type merge \
--patch="{\"data\":{\".dockerconfigjson\":\"$DOCKER_CONFIG_JSON\"}}"

d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry \
--user=license-token \
--password=$LICENSE_TOKEN \
--new-deckhouse-tag=$DECKHOUSE_VERSION \
registry.deckhouse.io/deckhouse/$NEW_EDITION
```

{% endcapture %}

{% capture ngc_auth_registry %}
{{ alert_additional_registry }}

For ContainerdV1:

```shell
AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
d8 k --as=system:sudouser apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerdv1-$NEW_EDITION-config.sh
spec:
  nodeGroups:
  - '*'
  bundles:
  - '*'
  weight: 1
  content: |
    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/$NEW_EDITION-registry.toml - << "EOF"
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry.configs]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry.deckhouse.io"]
              endpoint = ["https://registry.deckhouse.io"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs."registry.deckhouse.io".auth]
            auth = "$AUTH_STRING"
    EOF
EOF
```

For ContainerdV2:

```shell
d8 k --as=system:sudouser apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerdv2-$NEW_EDITION-config.sh
spec:
  nodeGroups:
  - '*'
  bundles:
  - '*'
  weight: 31
  content: |
    mkdir -p /etc/containerd/registry.d/registry.deckhouse.io
    bb-sync-file "/etc/containerd/registry.d/registry.deckhouse.io/hosts.toml" - << "EOF"
    [host]

      [host."https://registry.deckhouse.io"]
         capabilities = ["pull", "resolve"]
         [host."https://registry.deckhouse.io".auth]
            username = "license-token"
            password = "${LICENSE_TOKEN}"
    EOF
EOF
```

{% endcapture %}

{% capture ngc_cleanup_registry %}

For ContainerdV1

```shell
d8 k --as=system:sudouser delete ngc containerdv1-$NEW_EDITION-config.sh
d8 k --as=system:sudouser apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: del-temp-config.sh
spec:
  nodeGroups:
  - '*'
  bundles:
  - '*'
  weight: 1
  content: |
    if [ -f /etc/containerd/conf.d/$NEW_EDITION-registry.toml ]; then
      rm -f /etc/containerd/conf.d/$NEW_EDITION-registry.toml
    fi
EOF
```

```shell
d8 k --as=system:sudouser delete ngc del-temp-config.sh
```

For ContainerdV2

```shell
d8 k --as=system:sudouser delete ngc containerdv2-$NEW_EDITION-config.sh
```

{% endcapture %}

This guide describes the steps required to switch the Deckhouse Kubernetes Platform edition in a running cluster. Follow the sections in order.

The switching process differs depending on how you work with the image registry. Choose the method that applies to your cluster and follow the instructions.

A valid license key is required when switching to DKP BE/SE/SE+/EE. It is not required when switching to DKP CE.

{% alert level="warning" %}
This guide assumes the use of a public container image registry (`registry.deckhouse.io`). If you use a different registry address, adjust the commands or refer to the [guide for switching Deckhouse to a third-party container image registry](../registry/third-party.html).

Execute all commands on the master node of the existing cluster as the `root` user.
{% endalert %}

## Pre-switch preparation

Before switching between revisions, follow these steps:

1. Make sure that [the DKP queues are empty](#pre-switch-preparation).
1. Determine the [current DKP revision and version](#determining-the-current-edition-and-version).
1. Verify that you can switch [from the current version to the desired one](#checking-whether-switching-to-the-desired-edition-is-possible).

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

Different DKP editions support different sets of modules, Kubernetes versions, and features. It is important to understand what functional changes will occur during the switch and which capabilities will become unavailable. This will help you prepare for the switching process.

A comparison of DKP editions by module set can be found in the documentation on the [Edition comparison](../../../reference/revision-comparison.html) page.

What to consider before switching:

{% tabs step1 %}
{% tab "To DKP CE" %}

{{
   take_care_of_the_internal_modules
   | regex_replace: "(?m)^[ \t]*<!REMOVE_FOR_CE>.+?<!/REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "ce"
}}

{{
   take_care_of_the_external_modules
   | regex_replace: "(?m)^[ \t]*<!REMOVE_FOR_CE>.+?<!/REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "ce"
}}

{{ take_care_of_the_queue }}
{% endtab %}

{% tab "To DKP BE" %}
{{
   take_care_of_the_internal_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "be"
}}

{{
   take_care_of_the_external_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "be"
}}

{{ take_care_of_the_queue }}
{% endtab %}

{% tab "To DKP SE" %}
{{
   take_care_of_the_internal_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "se"
}}

{{
   take_care_of_the_external_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "se"
}}

{{ take_care_of_the_queue }}
{% endtab %}

{% tab "To DKP SE+" %}
{{
   take_care_of_the_internal_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "se-plus"
}}

{{
   take_care_of_the_external_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "se-plus"
}}

{{ take_care_of_the_queue }}
{% endtab %}

{% tab "To DKP EE" %}
{{
   take_care_of_the_internal_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "ee"
}}

{{
   take_care_of_the_external_modules
   | regex_replace: "(?m)^[ \t]*<!/?REMOVE_FOR_CE>\n?", ""
   | regex_replace: "\$NEW_EDITION", "ee"
}}

{{ take_care_of_the_queue }}
{% endtab %}
{% endtabs %}

## Switching the edition

### Selecting a switching method

When selecting a switching method, consider how the DKP container image repository is configured within the cluster.

There are two ways to work with the DKP container image registry:

- Using the [`registry`](/modules/registry/) module — **(recommended)** — the configuration for working with the DKP registry is specified in the [`registry`](/modules/deckhouse/configuration.html#parameters-registry) section of the `deckhouse` module parameters (ModuleConfig `deckhouse`). This method provides a smoother transition process and automatic verification that the required images are present. If your cluster uses this method for working with the DKP container registry, use the section ["Switching Using the registry Module"](#switching-using-the-registry-module) to switch editions.

- Without using the `registry` module — the configuration for working with the DKP registry is set during cluster installation [in `InitConfiguration`](../../../reference/api/cr.html#initconfiguration-deckhouse-imagesrepo), and the [`registry.mode`](/modules/deckhouse/configuration. html#parameters-registry-mode) of the `deckhouse` module (ModuleConfig `deckhouse`) is set to `Unmanaged`, and the [`registry.unmanaged`](/modules/deckhouse/configuration.html#parameters-registry-unmanaged) parameter of the `deckhouse` module is not specified.

  This method is the only one available for managed Kubernetes clusters where the control plane is managed by a cloud provider rather than DKP (e.g. Amazon EKS, Azure AKS, Google GKE, etc.).

  If your cluster uses this method for managing the DKP container image repository, refer to the section ["Switching without using the registry module"](#switching-without-the-registry-module) to switch versions.

Before proceeding, complete the preparatory steps described in the [Pre-switch preparation](#pre-switch-preparation) section.

### Switching using the registry module

{% alert level="warning" %}
Not applicable for managed Kubernetes (EKS, AKS, GKE).
{% endalert %}

1. Make sure the cluster uses the `registry` module. ModuleConfig `deckhouse` must contain the registry parameters of the previous DKP edition in `Unmanaged` mode. If this is not the case, perform the [migration to using the registry module](../registry/managing-interaction.html#migration-to-registry-management-format-using-the-registry-module).

   After completing the migration, ModuleConfig `deckhouse` must contain the registry parameters:

   {% tabs switch-registry-edition-1 %}
   {% tab "DKP CE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<EDITION_CODE>", "<PREVIOUS_EDITION_CODE>"
         | regex_replace: "<CHECK_MODE>", "Default"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io"
         | regex_replace: "(?m)<!REMOVE_FOR_CE>.+?<!/REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP BE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<EDITION_CODE>", "<PREVIOUS_EDITION_CODE>"
         | regex_replace: "<CHECK_MODE>", "Default"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP SE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<EDITION_CODE>", "<PREVIOUS_EDITION_CODE>"
         | regex_replace: "<CHECK_MODE>", "Default"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP SE+" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<EDITION_CODE>", "<PREVIOUS_EDITION_CODE>"
         | regex_replace: "<CHECK_MODE>", "Default"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP EE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<EDITION_CODE>", "<PREVIOUS_EDITION_CODE>"
         | regex_replace: "<CHECK_MODE>", "Default"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}
   {% endtabs %}

   The switch status must not contain errors:

   {{ registry_status_cmd | regex_replace: "^", "   " }}

   Example of a successful output:

   {{ registry_status_example | regex_replace: "^", "   " }}

1. In ModuleConfig [`deckhouse`](/modules/deckhouse/configuration.html#parameters-registry), set `imagesRepo` to the target edition and `checkMode: Relax`:

   Run the command to edit ModuleConfig `deckhouse`:

   ```shell
   d8 k edit moduleconfig deckhouse
   ```

   Choose the example for your edition:

   {% tabs switch-registry-edition %}
   {% tab "DKP CE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<EDITION_CODE>", "ce"
         | regex_replace: "<CHECK_MODE>", "Relax"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io"
         | regex_replace: "(?m)<!REMOVE_FOR_CE>.+?<!/REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP BE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<EDITION_CODE>", "be"
         | regex_replace: "<CHECK_MODE>", "Relax"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP SE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<EDITION_CODE>", "se"
         | regex_replace: "<CHECK_MODE>", "Relax"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP SE+" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<EDITION_CODE>", "se-plus"
         | regex_replace: "<CHECK_MODE>", "Relax"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}

   {% tab "DKP EE" %}
      {{
         change_registry_mc_deckhouse_unmanaged
         | regex_replace: "<EDITION_CODE>", "ee"
         | regex_replace: "<CHECK_MODE>", "Relax"
         | regex_replace: "<REGISTRY_HOST>", "registry.deckhouse.io"
         | regex_replace: "<!/?REMOVE_FOR_CE>\n?", ""
      }}
   {% endtab %}
   {% endtabs %}

{{ take_care_deckhuse_imagepullbackoff }}

1. Wait for the switch to complete.

   Check the status:

   {{ registry_status_cmd | regex_replace: "^", "   " }}

   Example of a successful output:

   {{ registry_status_example_relax | regex_replace: "^", "   " }}

1. Set `checkMode` back to `Default`:

   ```shell
   d8 k patch moduleconfig deckhouse --type=json -p='[{"op": "replace", "path": "/spec/settings/registry/unmanaged/checkMode", "value": "Default"}]'
   ```

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

   {{ check_old_pods | regex_replace: "^", "   " }}

### Switching without the registry module

{% alert level="warning" %}
Before proceeding, make sure that the `registry` module is not used in the cluster. ModuleConfig `deckhouse` must not contain registry parameters. The `registry` module must be disabled. If this is not the case, perform the [migration to the deprecated registry management format (without the registry module)](../registry/managing-interaction.html#migration-to-the-deprecated-registry-management-format-without-the-registry-module).
{% endalert %}

Choose the target edition:

{% tabs switch-without-registry %}
{% tab "DKP CE/BE/SE/SE+/EE" %}
1. Run the command to set the authentication credentials for the image registry:

   {% tabs without-registry-auth %}
   {% tab "DKP CE" %}
      Not required for DKP CE.
   {% endtab %}

   {% tab "DKP BE" %}
      {{
         ngc_auth_registry
         | regex_replace: "\$NEW_EDITION", "be"
         | regex_replace: "^", "   "
      }}

      {{
         bashible_sync_wait
         | regex_replace: "^", "   "
      }}
   {% endtab %}

   {% tab "DKP SE" %}
      {{
         ngc_auth_registry
         | regex_replace: "\$NEW_EDITION", "se"
         | regex_replace: "^", "   "
      }}

      {{
         bashible_sync_wait
         | regex_replace: "^", "   "
      }}
   {% endtab %}

   {% tab "DKP SE+" %}
      {{
         ngc_auth_registry
         | regex_replace: "\$NEW_EDITION", "se-plus"
         | regex_replace: "^", "   "
      }}

      {{
         bashible_sync_wait
         | regex_replace: "^", "   "
      }}
   {% endtab %}

   {% tab "DKP EE" %}
      {{
         ngc_auth_registry
         | regex_replace: "\$NEW_EDITION", "ee"
         | regex_replace: "^", "   "
      }}

      {{
         bashible_sync_wait
         | regex_replace: "^", "   "
      }}
   {% endtab %}
   {% endtabs %}

1. Switch the container image registry:

   {% tabs without-registry-switch %}
   {% tab "DKP CE" %}
      {{
         change_registry_helper_ce
         | regex_replace: "^", "   "
      }}
   {% endtab %}

   {% tab "DKP BE" %}
      {{
         change_registry_helper
         | regex_replace: "\$NEW_EDITION", "be"
         | regex_replace: "^", "   "
      }}
   {% endtab %}

   {% tab "DKP SE" %}
      {{
         change_registry_helper
         | regex_replace: "\$NEW_EDITION", "se"
         | regex_replace: "^", "   "
      }}
   {% endtab %}

   {% tab "DKP SE+" %}
      {{
         change_registry_helper
         | regex_replace: "\$NEW_EDITION", "se-plus"
         | regex_replace: "^", "   "
      }}
   {% endtab %}

   {% tab "DKP EE" %}
      {{
         change_registry_helper
         | regex_replace: "\$NEW_EDITION", "ee"
         | regex_replace: "^", "   "
      }}
   {% endtab %}
   {% endtabs %}

{{ take_care_deckhuse_imagepullbackoff }}

1. Wait for DKP to be ready:

   {{ wait_queue | regex_replace: "^", "   " }}

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

   {{ check_old_pods | regex_replace: "^", "   " }}

1. Perform cleanup:

   {% tabs without-registry-cleanup %}
   {% tab "DKP CE" %}
      Not required for DKP CE.
   {% endtab %}

   {% tab "DKP BE" %}
      {{
         ngc_cleanup_registry
         | regex_replace: "\$NEW_EDITION", "be"
         | regex_replace: "^", "   "
      }}
   {% endtab %}

   {% tab "DKP SE" %}
      {{
         ngc_cleanup_registry
         | regex_replace: "\$NEW_EDITION", "se"
         | regex_replace: "^", "   "
      }}
   {% endtab %}

   {% tab "DKP SE+" %}
      {{
         ngc_cleanup_registry
         | regex_replace: "\$NEW_EDITION", "se-plus"
         | regex_replace: "^", "   "
      }}
   {% endtab %}

   {% tab "DKP EE" %}
      {{
         ngc_cleanup_registry
         | regex_replace: "\$NEW_EDITION", "ee"
         | regex_replace: "^", "   "
      }}
   {% endtab %}
   {% endtabs %}
{% endtab %}
{% endtabs %}
