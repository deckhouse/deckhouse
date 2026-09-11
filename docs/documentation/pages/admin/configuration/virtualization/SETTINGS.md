---
title: "Virtualization module parameters"
permalink: en/admin/configuration/virtualization/settings.html
description: "Parameters of the `virtualization` module ModuleConfig: enabling and disabling the module, configuration version, and Ingress settings for image upload."
search: module parameters, ModuleConfig, virtualization settings, ingressClass
---

You configure the `virtualization` module in the [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) resource. The following example sets the Ingress controller class, the image storage, and the subnet for virtual machines:

{% tabs moduleconfig %}

{% tab "Using the CLI" %}

Apply the manifest with the required parameters:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  version: 1
  settings:
    ingressClass: nginx # Optional parameter.
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 50G
          storageClassName: rv-thin-r1
        type: PersistentVolumeClaim
    virtualMachineCIDRs:
      - 10.66.10.0/24
```

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **System** tab, then to **Deckhouse** → **Modules**.
1. Select the `virtualization` module from the list.
1. In the window that opens, select the **Configuration** tab.
1. To show the settings, click the **Advanced settings** toggle.
1. Set the parameters. The form field names match the parameter names in YAML.
1. Click **Save**.

{% endtab %}

{% endtabs %}

## Enabling and disabling the module

The [`.spec.enabled`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig-v1alpha1-spec-enabled) parameter controls the module state. Set it to `true` to enable the module, or to `false` to disable it.

Disabling the module stops every system component that creates and runs virtual machines (VMs), so the module can't be disabled by default.
To make it possible, add the `modules.deckhouse.io/allow-disabling` annotation set to `true` to the `virtualization` [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig).

Before disabling the module, prepare the cluster:

1. Delete all module resources, including virtual machines, disks, and images.
1. Verify that no active resources are left in the cluster:

   ```shell
   d8 k get virtualization -A
   d8 k get virtualization-cluster
   ```

Then edit the `virtualization` [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
  annotations:
    modules.deckhouse.io/allow-disabling: "true"
spec:
  enabled: false
  version: 1
  settings:
    # Specify the existing settings.
```

{% alert level="danger" %}
If the module resources aren't deleted, disabling the module can lead to data loss.
{% endalert %}

## Configuration version

The [`.spec.version`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig-v1alpha1-spec-version) parameter defines the settings schema version. The parameter structure can change between versions; for the current values, see the [module settings](/modules/virtualization/configuration.html).

## Ingress settings

Virtual machine images are uploaded to the cluster through an [Ingress controller](/modules/ingress-nginx/), whose class is defined by the [`.spec.settings.ingressClass`](/modules/virtualization/configuration.html#parameters-ingressclass) parameter.
The parameter is optional: if you leave it unset, the module uses the global value from the Deckhouse Platform (DP) configuration.
Set it only when image upload requires a separate Ingress controller.

Example:

```yaml
spec:
  settings:
    ingressClass: nginx
```

{% alert level="info" %}
Large virtual machine images take a long time to upload over a slow connection, and restarting or updating the Ingress controller interrupts the upload.
To avoid this, increase the worker shutdown timeout in the [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) resource.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  config:
    worker-shutdown-timeout: 1800s  # 30 minutes or more, if required.
```

{% endalert %}
