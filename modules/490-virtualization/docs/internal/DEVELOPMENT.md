---
title: Maintenance of the virtualization module 
searchable: false
---

How to update KubeVirt
----------------------

Most manifests, such as crd and rbac, are generated automatically from the original ones.  
To update the version of KubeVirt run:

```bash
hack/update.sh v1.58.0
```

Also don't forget to bump the version in `images/artifact/werf.inc.yaml`:

```yaml
{{- $version := "0.58.0" }}
```

How to update Custom Resources
------------------------------

Custom resource definitions are generated out of API types.  
To add new fields edit specific object type in `./hooks/internal/...` and run:

```bash
make generate
make crds
```

to generate dependent resources.

> You can edit `doc-ru-*` files manually. After editing run `make crds` again to format the changes.


How to deploy KubeVirt in OpenStack for dev purposes
----------------------------------------------------

1. Organize a flavor with cpu-passthrough (and use it in OpenstackInstanceClass for libvirt-nodes):

```
openstack flavor create --vcpus 4 --ram 8192 --disk 20 --property hw:cpu_model=host-passthrough m1.large-cpu-host-passthrough

```

2. Configure cni-cilium:


Configure ModuleConfig:

```
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-cilium
spec:
  enabled: true
  settings:
    tunnelMode: VXLAN
  version: 1
```

Configure d8-cni-configuration Secret:
```
PATCH="$(kubectl -n kube-system get secret d8-cni-configuration -o json | jq -rc '.data.cilium | @base64d | fromjson | .masqueradeMode = "BPF" | tojson | @base64 | {"data": {"cilium": .}}')"
kubectl -n kube-system patch secret d8-cni-configuration -p "$PATCH"
```

3. Configure Virtualization module:


```
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  settings:
    vmCIDRs:
    - 10.20.20.0/24
  version: 1
```

4. Enable emulation:

UNDER CONSTRUCTION







kubectl patch storageprofile default --type=merge -p '{"spec": {"claimPropertySets": [{"accessModes": ["ReadWriteOnce"], "volumeMode": "Block"}]}}'
