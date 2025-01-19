---
title: "Module embedded-registry: bootstrap architecture"
description: ""
---

To pass parameters to a module during the bootstrap stage, [InitConfiguration](candi/openapi/init_configuration.yaml) is used.
Example:

```yaml
apiVersion: deckhouse.io/v2alpha1
kind: InitConfiguration
deckhouse:
registry:
    mode: Proxy
    proxy:
    imagesRepo: nexus.company.my/deckhouse/ee
    username: "nexus-user"
    password: "nexus-p@ssw0rd"
    scheme: HTTPS
    ca: |
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
    storageMode: Fs
---
apiVersion: deckhouse.io/v2alpha1
kind: InitConfiguration
deckhouse:
registry:
    mode: Detached
    detached:
    imagesBundlePath: ~/deckhouse/bundle.tar
    storageMode: Fs
```

The parameters are parsed and passed to Basible for template rendering. The main templates are located in:
- [bootstrap folder](candi/bashible/common-steps/cluster-bootstrap/):
  - to start igniter;
  - to push Docker images to a registry in mirror mode `embedded-registry`;
  - configuration of a static pod for `embedded-registry`, pull Docker images for static pod;
  - to stop igniter;
- configuration of containerd;
- configuration of kube-api-proxy(nginx). Configuration and image pulling for kube-api-proxy are being performed (to connect the next node to the cluster);

After the bootstrap and the launch of deckhouse, the [ModuleConfig](dhctl/pkg/config/module_config.go#L102) is applied, and the module starts.
Control of the `embedded-registry` static pods is handed over to the module.
